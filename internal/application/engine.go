// Package application owns accepted Runs and their single-writer transitions.
package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"log"
	"path/filepath"
	"sync"
	"time"
)

type Engine struct {
	openNativeConsole func(context.Context, b.LaunchCommand) error
	lastMessages      map[string]contracts.LastMessage
	epoch             string
	identity          *b.ProcessIdentity
	mu                sync.Mutex
	requests          sync.Map
	dir               string
	store             *storage.Store
	ctx               context.Context
	cancel            context.CancelFunc
	runs              map[string]*runstate.Record
	workers           map[string]*worker
	checks            map[string]*runstate.CheckRecord
	checkWorkers      map[string]*checkWorker
	logger            *log.Logger
	wait              sync.WaitGroup
}
type worker struct {
	gate     sync.Mutex // serializes new native dispatch with committed revocation
	wake     chan struct{}
	port     b.LifecyclePort
	observer b.LifecycleObserver
	handle   b.StopHandle
}

func New(ctx context.Context, dir string, store *storage.Store, logger *log.Logger) (*Engine, error) {
	ctx, cancel := context.WithCancel(ctx)
	e := &Engine{dir: dir, store: store, ctx: ctx, cancel: cancel, logger: logger, runs: map[string]*runstate.Record{}, workers: map[string]*worker{}}
	e.checks = map[string]*runstate.CheckRecord{}
	e.checkWorkers = map[string]*checkWorker{}
	checks, checkErr := store.Checks()
	if checkErr != nil {
		cancel()
		return nil, checkErr
	}
	for _, r := range checks {
		e.checks[r.View.CheckID] = r
		if next := reconcileCheckOwnership(r); next != nil {
			if err := e.commitCheckLocked(next, nil); err != nil {
				cancel()
				return nil, err
			}
		}
	}
	runs, err := store.Runs()
	if err != nil {
		cancel()
		return nil, err
	}
	e.epoch = contracts.NewID()
	e.identity, err = process.NewServiceFacts().CurrentIdentity()
	if err != nil {
		cancel()
		return nil, err
	}
	for _, r := range runs {
		e.runs[r.ID] = r
		e.workers[r.ID] = &worker{wake: make(chan struct{}, 1)}
		next := r.Clone()
		if next.PendingResume != "" {
			next.CommandStates[next.PendingResume] = "failed"
			next.PendingResume = ""
		}
		next.Allowed = false
		next.ResumeRequired = true
		if next.Status != "pr_ready" && next.StopStatus != "stopped" {
			next.Status = "reconciling"
			next.Phase = "service_restarted"
		}
		if err := e.commitLocked(next, nil, "service_restarted"); err != nil {
			cancel()
			return nil, err
		}
	}
	return e, nil
}
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, w := range e.workers {
		e.launch(id, w)
	}
	for id, r := range e.checks {
		if r.View.CleanupStatus != "completed" || r.Allowed {
			next := r.Clone()
			next.Allowed = false
			next.AgentSettled = r.AgentSettled || r.View.Status == "completed"
			next.View.Status = "interrupted"
			next.View.Diagnostic = "service_restarted_reconcile_original_only"
			if !next.AgentSettled && next.View.Result == "passed" && r.View.Status != "completed" {
				next.View.Result = "unverified"
			}
			if e.commitCheckLocked(next, nil) == nil {
				e.launchCheckLocked(id)
			}
		}
	}
}
func (e *Engine) launch(id string, w *worker) {
	e.wait.Add(1)
	go func() { defer e.wait.Done(); e.run(id, w) }()
	e.signal(w)
}
func (e *Engine) signal(w *worker) {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}
func (e *Engine) commitLocked(next *runstate.Record, command *storage.Command, events ...string) error {
	prior := e.runs[next.ID]
	var expected uint64
	if prior != nil {
		expected = prior.Revision
	}
	next.Revision = expected + 1
	next.UpdatedAt = time.Now().UTC()
	if err := e.store.Commit(next, expected, command, events...); err != nil {
		return err
	}
	e.runs[next.ID] = next
	return nil
}
func fault(err error) *contracts.Fault {
	var f *contracts.Fault
	if errors.As(err, &f) {
		return f
	}
	return contracts.Fail(1, "internal_error", "Cannot commit or read the requested operation.")
}
func (e *Engine) Handle(command, runID, requestID, commandID, after string, limit int, refresh bool, input *contracts.Request) contracts.Response {
	id := ""
	if input != nil {
		id = input.RequestID
	}
	reject := func(err error) contracts.Response { return contracts.Rejected(id, fault(err)) }
	if input != nil {
		digest := storage.RequestDigest(command, runID, *input)
		entry, _ := e.requests.LoadOrStore(id, &sync.Mutex{})
		requestLock := entry.(*sync.Mutex)
		requestLock.Lock()
		defer requestLock.Unlock()
		if original, found, err := e.store.Original(input.RequestID, digest); err != nil {
			return reject(err)
		} else if found {
			return original
		}
		if command == "submit" {
			r, err := freeze(e.ctx, e.dir, *input)
			if err != nil {
				return reject(err)
			}
			receipt := storage.NewCommand(command, r.ID, *input)
			receipt.Digest = digest
			r.SubmitCommandID = receipt.ID
			r.CommandStates[receipt.ID] = "pending"
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.ctx.Err() != nil {
				return reject(contracts.Fail(4, "service_unavailable", "Service is stopping; submission was not accepted."))
			}
			if err := e.commitLocked(r, &receipt, "run_accepted"); err != nil {
				return reject(contracts.Fail(6, "request_outcome_unknown", "Submission commit could not be verified; inspect the original request_id."))
			}
			w := &worker{wake: make(chan struct{}, 1)}
			e.workers[r.ID] = w
			e.launch(r.ID, w)
			return receipt.Response
		}
		e.mu.Lock()
		w := e.workers[runID]
		e.mu.Unlock()
		if w == nil {
			return reject(contracts.Fail(5, "run_not_found", "Run was not found."))
		}
		// Acquiring this gate waits only for an already dispatched native call;
		// once revocation is accepted no later call can cross the boundary.
		w.gate.Lock()
		defer w.gate.Unlock()
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.ctx.Err() != nil {
			return reject(contracts.Fail(4, "service_unavailable", "Service is stopping; command was not accepted."))
		}
		r := e.runs[runID]
		next := r.Clone()
		var revision contracts.RevisionInput
		_ = json.Unmarshal(input.Input, &revision)
		if revision.ExpectedRevision != r.Revision {
			return reject(contracts.Fail(5, "stale_revision", "Run revision changed; inspect the current run before a new request."))
		}
		receipt := storage.NewCommand(command, runID, *input)
		next.CommandStates[receipt.ID] = "pending"
		switch command {
		case "stop":
			next.Allowed = false
			next.ResumeRequired = true
			next.PendingStop = receipt.ID
			next.StopStatus = "pending"
			next.Phase = "stop_requested"
		case "resume":
			if !r.ResumeRequired {
				return reject(contracts.Fail(5, "invalid_state", "Run does not require a new resume authorization."))
			}
			if unknown(r) {
				return reject(contracts.Fail(5, "operation_unverified", "An original operation is unresolved; resume cannot replay it."))
			}
			next.PendingResume = receipt.ID
			next.Phase = "resume_qualification"
		case "reply":
			var reply contracts.ReplyInput
			_ = json.Unmarshal(input.Input, &reply)
			if r.ResumeRequired || !r.Allowed {
				return reject(contracts.Fail(5, "resume_required", "Resume the current state before authorizing new input."))
			}
			if unknown(r) {
				return reject(contracts.Fail(5, "operation_unverified", "The original input outcome is unresolved."))
			}
			if reply.Kind == "answer" {
				if r.Question == nil || reply.QuestionID != r.Question.ID {
					return reject(contracts.Fail(5, "stale_question", "Question is no longer current."))
				}
				if r.PendingAnswer != nil && r.PendingAnswer.Epoch == e.epoch {
					return reject(contracts.Fail(5, "answer_pending", "An original answer still awaits its delivery outcome."))
				}
				if r.PendingAnswer != nil {
					next.AnswerHistory = append(next.AnswerHistory, *r.PendingAnswer)
					next.CommandStates[r.PendingAnswer.CommandID] = "failed"
				}
				next.PendingAnswer = &runstate.Answer{CommandID: receipt.ID, QuestionID: reply.QuestionID, Text: reply.Text, Epoch: e.epoch}
				next.Phase = "answer_saved"
			} else {
				if r.Status != "pr_ready" && r.Status != "blocked" {
					return reject(contracts.Fail(5, "invalid_state", "Business feedback requires an ended original turn."))
				}
				if r.PendingFeedback != nil {
					next.AnswerHistory = append(next.AnswerHistory, *r.PendingFeedback)
					next.CommandStates[r.PendingFeedback.CommandID] = "failed"
				}
				next.PendingFeedback = &runstate.Answer{CommandID: receipt.ID, Text: reply.Text, Epoch: e.epoch}
				next.Phase = "feedback_saved"
			}
		default:
			return reject(contracts.Fail(2, "invalid_arguments", "Unknown Run command."))
		}
		if err := e.commitLocked(next, &receipt, command+"_accepted"); err != nil {
			return reject(contracts.Fail(6, "request_outcome_unknown", "The command commit is unverified; inspect its original request_id."))
		}
		e.signal(w)
		return receipt.Response
	}
	if command == "inspect" && runID == "" {
		e.mu.Lock()
		defer e.mu.Unlock()
		selector, target := "request_id", requestID
		if commandID != "" {
			selector, target = "command_id", commandID
		}
		value, err := e.store.Command(selector, target)
		if errors.Is(err, sql.ErrNoRows) {
			return reject(contracts.Fail(5, selector[:len(selector)-3]+"_not_found", "The original command was not found."))
		}
		if err != nil {
			return reject(err)
		}
		var view *contracts.RunView
		if value.RunID != nil {
			if r := e.runs[*value.RunID]; r != nil {
				v := r.View()
				view = &v
			}
		}
		return contracts.Success("", map[string]any{"command": value, "run": view})
	}
	if refresh {
		if err := e.refreshPR(runID); err != nil {
			return reject(err)
		}
	}
	e.mu.Lock()
	r := e.runs[runID]
	if command == "last-message" && r != nil {
		defer e.mu.Unlock()
		view := contracts.LastMessageView{RunID: runID, AttemptID: r.Snapshot.Session.AttemptID}
		if message, ok := e.lastMessages[runID]; ok && message.AttemptID == view.AttemptID {
			view.Message = &message
		}
		return contracts.Success("", view)
	}
	if r != nil {
		r = r.Clone()
	}
	e.mu.Unlock()
	if r == nil {
		return reject(contracts.Fail(5, "run_not_found", "Run was not found."))
	}

	if command == "events" {
		page, err := e.store.Events(runID, after, limit)
		if err != nil {
			return reject(err)
		}
		return contracts.Success("", page)
	}
	return contracts.Success("", map[string]any{"command": nil, "run": r.View()})
}
func unknown(r *runstate.Record) bool {
	for _, intent := range r.Intents {
		if intent.NeedsReconciliation() {
			return true
		}
	}
	return false
}
func (e *Engine) setup(r *runstate.Record, w *worker) b.LifecycleSetup {
	repository := r.RepositoryPath
	if r.Request.Execution.Adapter == "orca-terminal" && r.Request.Repository.LocalPath != "" {
		repository = r.Request.Repository.LocalPath
	}
	return b.LifecycleSetup{Snapshot: r.Snapshot.Clone(), ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest, AgentLaunchCommand: r.LaunchCommand, ProcessHost: process.New(60 * time.Second), OwnedProcess: w.handle, RepositoryPath: repository, RepositoryURL: r.Request.Repository.URL, RunDirectory: e.dir, CommandTimeout: 60 * time.Second, AttemptNumber: r.Snapshot.Session.AttemptNumber, GitExecutable: "git", RemoteName: r.Request.Repository.Remote, TargetBranch: r.Request.Repository.BaseBranch, WorkerTreeAbsent: func(identity b.ProcessIdentity) bool {
		e.mu.Lock()
		current := e.runs[r.ID].Clone()
		e.mu.Unlock()
		return e.writersAbsent(current, w, identity)
	}}
}
func (e *Engine) port(r *runstate.Record, w *worker) (b.LifecyclePort, error) {
	if w.port != nil {
		return w.port, nil
	}
	var p b.LifecyclePort
	var err error
	if r.Request.Execution.Adapter == "codex-app-server" {
		p, err = codex.NewExecutionLifecycle(e.setup(r, w))
	} else {
		p, err = orca.NewLifecycle(e.setup(r, w))
	}
	if err == nil {
		w.port = p
	}
	return p, err
}

func (e *Engine) refreshPR(id string) error {
	e.mu.Lock()
	r := e.runs[id]
	if r == nil {
		e.mu.Unlock()
		return contracts.Fail(5, "run_not_found", "Run was not found.")
	}
	r = r.Clone()
	e.mu.Unlock()
	if r.PR == nil {
		return nil
	}
	provider, err := providerFor(r)
	if err != nil {
		return err
	}
	info, queryErr := provider.Query(e.ctx, r.PR.URL)
	if queryErr == nil {
		queryErr = validatePR(r, info)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if current == nil || current.PR == nil || current.PR.URL != r.PR.URL {
		return contracts.Fail(5, "stale_revision", "PR binding changed while refreshing.")
	}
	next := current.Clone()
	if queryErr != nil {
		next.PR.QueryError = "query_error: provider facts could not be verified"
	} else {
		next.PR = prFact(r, info)
	}
	return e.commitLocked(next, nil, "pr_refreshed")
}

func (e *Engine) Shutdown(ctx context.Context) error {
	e.cancel()
	e.mu.Lock()
	for _, r := range e.runs {
		next := r.Clone()
		next.Allowed = false
		next.ResumeRequired = true
		if err := e.commitLocked(next, nil, "service_stopping"); err != nil {
			e.mu.Unlock()
			return err
		}
	}
	e.mu.Unlock()
	done := make(chan struct{})
	go func() { e.wait.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ResourceSettlementError reads committed evidence after worker shutdown.
// Unknown/unsupported resources remain unresolved, including detached Orca.
func (e *Engine) ResourceSettlementError() error {
	runs, err := e.store.Runs()
	if err != nil {
		return err
	}
	var failures []error
	for _, r := range runs {
		if r.StopStatus != "stopped" {
			failures = append(failures, fmt.Errorf("run %s: native stop status %q", r.ID, r.StopStatus))
		}
	}
	checks, err := e.store.Checks()
	if err != nil {
		return err
	}
	for _, r := range checks {
		if r.View.CleanupStatus != "completed" || !r.View.Resources.WriterStopped || checkUnknown(r) {
			failures = append(failures, fmt.Errorf("check %s: resources remain unverified", r.View.CheckID))
		}
	}
	return errors.Join(failures...)
}

func runLogPath(dir, id string) string { return filepath.Join(dir, "logs", id+".log") }
