package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/environmentcheck"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func freezeEnvironmentCheck(request contracts.RunRequest) (*runstate.Record, error) {
	if err := contracts.ValidateEnvironmentCheck(request); err != nil {
		return nil, err
	}
	if request.Execution.Adapter == "codex-app-server" && runtime.GOOS != "windows" {
		return nil, contracts.Fail(3, "unsupported_capability", "Direct environment checkout execution is qualified on Windows.")
	}
	id, attempt, generation := contracts.NewID(), contracts.NewID(), contracts.NewID()
	target := b.ExecutionTarget{ID: "local", ExecutionHost: "local"}
	backend := b.BackendLocal
	if request.Execution.Adapter == "orca-terminal" {
		backend = b.BackendOrca
		if env := request.Execution.Target.Environment; env != "" {
			host, err := native.UsageHostID(env)
			if err != nil {
				return nil, contracts.Fail(4, "environment_unavailable", "The original paired Orca host is unavailable.")
			}
			target = b.ExecutionTarget{ID: env, ExecutionHost: host}
		}
	}
	now := time.Now().UTC()
	digest := reliable.Digest(canonical(request))
	snapshot := b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1, Backend: backend, Protocol: b.ProtocolLocalProcess, Target: target, RunID: id, AttemptID: attempt, BaseCommit: request.Repository.BaseRevision},
		Session:             b.SessionContext{WorkflowRunID: id, ExecutionID: id, AttemptID: attempt, Target: target, AttemptNumber: 1},
		ConfigurationDigest: digest, ResourceGeneration: generation}
	return &runstate.Record{ID: id, Request: request, InputDigest: digest, ResourceDigest: resources.Digest(), Snapshot: snapshot,
		EnvironmentInvocation: &runstate.EnvironmentInvocation{}, Allowed: true, Status: "queued", Phase: "accepted", CommandStates: map[string]string{}, CreatedAt: now, UpdatedAt: now}, nil
}

func (e *Engine) runEnvironmentCheck(id string, w *worker) {
	if r := e.environmentRecord(id); r.StopStatus == "stopped" {
		return
	}
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	var localDone <-chan environmentExecution
	for {
		select {
		case <-e.ctx.Done():
			if w.handle != nil {
				w.handle.Kill()
				if localDone != nil {
					select {
					case <-localDone:
						if tree, ok := w.handle.(b.ProcessTreeHandle); ok && tree.WritersAbsent() {
							e.finishEnvironment(id, environmentcheck.Result{}, errors.New("Runner stopped during the environment invocation"), true)
						}
					case <-time.After(5 * time.Second):
					}
				}
				w.handle.Close()
			} else {
				e.shutdownEnvironmentTerminal(id, w)
			}
			return
		case outcome := <-localDone:
			localDone = nil
			tree, ok := w.handle.(b.ProcessTreeHandle)
			if ok && !tree.WritersAbsent() {
				w.handle.Kill()
				for i := 0; i < 50 && !tree.WritersAbsent(); i++ {
					time.Sleep(20 * time.Millisecond)
				}
			}
			settled := ok && tree.WritersAbsent()
			w.handle.Close()
			w.handle = nil
			if !settled {
				e.noteEnvironment(id, "Original local process tree absence is unverified")
				continue
			}
			e.finishEnvironment(id, outcome.result, outcome.err, true)
		case <-timer.C:
			e.signal(w)
		case <-w.wake:
			w.gate.Lock()
			r := e.environmentRecord(id)
			switch {
			case r.StopStatus == "stopped":
				w.gate.Unlock()
				return
			case r.Request.Execution.Adapter == "orca-terminal":
				e.advanceEnvironmentTerminal(r)
			case localDone != nil:
				if r.PendingStop != "" {
					w.handle.Kill()
				}
			case !r.EnvironmentInvocation.Attempted && r.Allowed && r.PendingStop == "":
				localDone = e.startLocalEnvironment(r, w)
			case !r.EnvironmentInvocation.Attempted:
				e.finishEnvironment(id, environmentcheck.Result{}, nil, true)
			default:
				e.recoverLocalEnvironment(r)
			}
			w.gate.Unlock()
		}
	}
}

type environmentExecution struct {
	result environmentcheck.Result
	err    error
}

func (e *Engine) environmentRecord(id string) *runstate.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runs[id].Clone()
}

func (e *Engine) startLocalEnvironment(r *runstate.Record, w *worker) <-chan environmentExecution {
	root := r.Request.Repository.LocalPath
	prepared, err := environmentcheck.Build(r.Request, root, r.ID, runtime.GOOS == "windows", false)
	if err != nil {
		e.finishEnvironment(r.ID, environmentcheck.Result{}, err, true)
		return nil
	}
	cmd := exec.Command(prepared.Executable, prepared.Args...)
	cmd.Dir = root
	var output environmentOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	handle, err := process.StartManagedBound(cmd, func(identity b.ProcessIdentity) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		next := e.runs[r.ID].Clone()
		next.EnvironmentInvocation.Attempted = true
		next.NativeOwner = e.identity
		next.Snapshot.Ref.External = identity
		bindEnvironmentStart(next, root)
		return e.commitLocked(next, nil, "environment_process_started")
	})
	if err != nil {
		e.finishEnvironment(r.ID, environmentcheck.Result{}, err, true)
		return nil
	}
	w.handle = handle
	done := make(chan environmentExecution, 1)
	go func() {
		err := cmd.Wait()
		if cmd.ProcessState == nil {
			done <- environmentExecution{err: err}
			return
		}
		if output.truncated {
			done <- environmentExecution{err: errors.New("environment output exceeds bounds")}
			return
		}
		code := cmd.ProcessState.ExitCode()
		result, err := environmentcheck.Decode(strings.Split(output.body.String(), "\n"), r.ID, &code)
		done <- environmentExecution{result, err}
	}()
	return done
}

type environmentOutput struct {
	body      strings.Builder
	truncated bool
}

func (o *environmentOutput) Write(data []byte) (int, error) {
	if len(data) > 128<<10-o.body.Len() {
		o.truncated = true
		return len(data), nil
	}
	return o.body.Write(data)
}

func (e *Engine) recoverLocalEnvironment(r *runstate.Record) {
	// This service never replays an admitted process. Its original Windows Job
	// has kill-on-close ownership by the service that durably recorded the root.
	if runtime.GOOS == "windows" && r.NativeOwner != nil &&
		process.NewServiceFacts().Probe(r.NativeOwner).State == "absent" &&
		process.NewServiceFacts().Probe(&r.Snapshot.Ref.External).State == "absent" {
		e.finishEnvironment(r.ID, environmentcheck.Result{}, errors.New("Original environment process ended during Runner restart; result unverified"), true)
	} else {
		e.noteEnvironment(r.ID, "Original environment process and result remain unverified")
	}
}

func bindEnvironmentStart(r *runstate.Record, root string) {
	r.Snapshot.Ref.WorkspacePath = root
	r.WorkspaceFact = &b.WorkspaceInspection{InspectionScope: "existing_checkout", Worktree: root}
	r.Status, r.Phase = "running", "observing"
	r.NativeStarted = true
	r.CommandStates[r.SubmitCommandID] = "running"
}

func (e *Engine) finishEnvironment(id string, result environmentcheck.Result, err error, settled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if r.StopStatus == "stopped" || !settled {
		return
	}
	r.Allowed, r.Closed = false, true
	r.StopStatus, r.Phase = "stopped", "closed"
	r.Status = "failed"
	if err != nil {
		r.Diagnostic = e.redact(r, err.Error())
	} else {
		r.Diagnostic = e.redact(r, result.Diagnostic)
	}
	if r.PendingStop != "" {
		r.Status = "stopped"
		r.CommandStates[r.PendingStop] = "completed"
		r.PendingStop = ""
	} else if err == nil && result.ExitCode == 0 && len(result.Data) != 0 {
		r.Status = "completed"
		r.LastFinal = &contracts.FinalView{Verified: true, NativeOutcome: "succeeded", CommandID: r.SubmitCommandID, AttemptID: r.Snapshot.Session.AttemptID, SessionID: r.ID, TurnID: r.ID, Source: "environment_process", ItemID: r.ID}
	} else if err == nil && r.Diagnostic == "" {
		r.Diagnostic = fmt.Sprintf("Environment script exited with status %d", result.ExitCode)
	}
	r.CommandStates[r.SubmitCommandID] = "failed"
	if r.Status == "completed" {
		r.CommandStates[r.SubmitCommandID] = "completed"
	}
	view := contracts.ResultFileView{Contract: contracts.ResultFileContract, RunID: r.ID, Encoding: "base64", CollectionStatus: "closed", Diagnostic: r.Diagnostic}
	if len(result.Data) != 0 && err == nil {
		encoded := base64.StdEncoding.EncodeToString(result.Data)
		now := time.Now().UTC()
		view.Version, view.ContentBase64, view.SizeBytes, view.SHA256, view.ObservedAt = 1, &encoded, len(result.Data), reliable.Digest(result.Data), &now
		view.Diagnostic = ""
	}
	r.ResultFile = &runstate.ResultFile{Sealed: true, Progress: runstate.Progress{View: view}}
	r.EnvironmentInvocation.Receipt = nil
	if err := e.commitLocked(r, nil, "environment_invocation_settled"); err != nil {
		e.logger.Printf("Cannot settle environment invocation run=%s: %v", id, err)
	}
}

func (e *Engine) noteEnvironment(id, diagnostic string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if r.Diagnostic == diagnostic {
		return
	}
	r.Diagnostic = diagnostic
	_ = e.commitLocked(r, nil, "environment_observation_unverified")
}

func resolveEnvironmentCheckout(ctx context.Context, r *runstate.Record) (native.Worktree, string, error) {
	client := native.EnvironmentTerminalClient{Environment: r.Request.Execution.Target.Environment, Host: r.Snapshot.Ref.Target.ExecutionHost}
	inventory, runtimeID, err := client.CheckoutInventory(ctx)
	if err != nil {
		return native.Worktree{}, "", err
	}
	if !inventory.Decoded() || inventory.Truncated || runtimeID == "" {
		return native.Worktree{}, "", errors.New("Orca repository inventory is incomplete")
	}
	name := func(s string) string { return path.Base(strings.ReplaceAll(s, "\\", "/")) }
	var repo native.Repository
	for _, candidate := range inventory.Repos {
		matches := strings.EqualFold(name(candidate.Path), name(r.Request.Repository.LocalPath))
		if r.Request.Execution.Target.Environment == "" {
			matches = environmentSamePath(candidate.Path, r.Request.Repository.LocalPath)
		}
		if candidate.Kind != "git" || !matches {
			continue
		}
		if repo.ID != "" || candidate.ID == "" {
			return native.Worktree{}, "", errors.New("Orca project checkout is ambiguous")
		}
		repo = candidate
	}
	if repo.ID == "" {
		return native.Worktree{}, "", errors.New("Existing project checkout was not found on the Orca execution host")
	}
	client.Runtime = runtimeID
	work, err := client.Checkout(ctx, repo.Path)
	if err != nil || work.RepoID != repo.ID || work.ID == "" || work.InstanceID == "" || !environmentSamePath(work.Path, repo.Path) {
		return native.Worktree{}, "", errors.New("Existing Orca checkout identity could not be verified")
	}
	return work, runtimeID, nil
}

func environmentSamePath(a, b string) bool {
	a, b = strings.ReplaceAll(a, "\\", "/"), strings.ReplaceAll(b, "\\", "/")
	if len(a) > 1 && a[1] == ':' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (e *Engine) advanceEnvironmentTerminal(r *runstate.Record) {
	ctx, cancel := context.WithTimeout(e.ctx, 25*time.Second)
	defer cancel()
	state := r.EnvironmentInvocation
	if !state.Attempted {
		if !r.Allowed || r.PendingStop != "" {
			e.finishEnvironment(r.ID, environmentcheck.Result{}, nil, true)
			return
		}
		e.startEnvironmentTerminal(ctx, r)
		return
	}
	client := native.EnvironmentTerminalClient{Environment: r.Request.Execution.Target.Environment, Runtime: state.Runtime, Host: state.Host}
	identity := native.TerminalIdentity{RuntimeID: state.Runtime, Handle: state.Terminal, IncarnationID: state.Incarnation}
	terminal, err := client.Terminal(ctx, identity)
	if err != nil || terminal.WorktreeID != state.Worktree || !environmentSamePath(terminal.WorktreePath, r.Snapshot.Ref.WorkspacePath) {
		e.noteEnvironment(r.ID, "Original Orca detection terminal identity is unverified")
		return
	}
	if state.Incarnation == "" {
		if err := e.bindEnvironmentTerminal(r, terminal); err != nil {
			e.noteEnvironment(r.ID, "Original Orca detection terminal could not be recorded")
		}
		return
	}
	if r.PendingStop != "" && !state.CloseAttempted && !terminal.Exited() {
		e.stopEnvironmentTerminal(ctx, r, client, identity)
		return
	}
	code, err := client.Exit(ctx, state.Terminal)
	if err != nil {
		e.noteEnvironment(r.ID, e.redact(r, err.Error()))
		return
	}
	if code != nil {
		if r.PendingStop != "" {
			e.finishEnvironment(r.ID, environmentcheck.Result{}, nil, true)
			return
		}
		if state.Receipt == nil || state.Receipt.ExitCode != *code {
			e.finishEnvironment(r.ID, environmentcheck.Result{}, errors.New("Original environment receipt is missing or differs from the native exit"), true)
			return
		}
		receipt := state.Receipt
		e.finishEnvironment(r.ID, environmentcheck.Result{Data: receipt.Data, Diagnostic: receipt.Diagnostic, ExitCode: *code}, nil, true)
		return
	}
	if state.Receipt == nil && r.PendingStop == "" {
		lines, err := client.Read(ctx, state.Terminal)
		if err == nil {
			var receipt environmentcheck.Result
			receipt, err = environmentcheck.Decode(lines, r.ID, nil)
			if err == nil {
				e.mu.Lock()
				next := e.runs[r.ID].Clone()
				next.EnvironmentInvocation.Receipt = &runstate.EnvironmentReceipt{Data: receipt.Data, Diagnostic: e.redact(next, receipt.Diagnostic), ExitCode: receipt.ExitCode}
				_ = e.commitLocked(next, nil, "environment_receipt_observed")
				e.mu.Unlock()
			}
		}
		if err != nil && !errors.Is(err, environmentcheck.ErrReceiptPending) {
			e.noteEnvironment(r.ID, e.redact(r, err.Error()))
		}
	}
}

func (e *Engine) startEnvironmentTerminal(ctx context.Context, r *runstate.Record) {
	work, runtimeID, err := resolveEnvironmentCheckout(ctx, r)
	if err != nil {
		e.finishEnvironment(r.ID, environmentcheck.Result{}, err, true)
		return
	}
	win := len(work.Path) > 1 && work.Path[1] == ':'
	command, err := environmentcheck.Build(r.Request, work.Path, r.ID, win, true)
	if err != nil {
		e.finishEnvironment(r.ID, environmentcheck.Result{}, err, true)
		return
	}
	client := native.EnvironmentTerminalClient{Environment: r.Request.Execution.Target.Environment, Runtime: runtimeID, Host: r.Snapshot.Ref.Target.ExecutionHost}
	expected, err := client.ExpectedHandle(ctx, work.ID, r.ID)
	if err != nil {
		e.finishEnvironment(r.ID, environmentcheck.Result{}, err, true)
		return
	}
	dispatched := false
	ctx = native.WithDispatchCheckpoint(ctx, func() error {
		e.mu.Lock()
		defer e.mu.Unlock()
		next := e.runs[r.ID].Clone()
		next.EnvironmentInvocation = &runstate.EnvironmentInvocation{Attempted: true, Runtime: runtimeID, Host: client.Host, Worktree: work.ID, WorkspaceInstance: work.InstanceID, Terminal: expected}
		next.Snapshot.Ref.WorkspacePath = work.Path
		next.Status, next.Phase = "running", "terminal_start"
		err := e.commitLocked(next, nil, "environment_terminal_dispatched")
		dispatched = err == nil
		return err
	})
	terminal, err := client.Start(ctx, work.ID, r.ID, command.Startup, command.Shell, command.Environment)
	if err != nil {
		if dispatched {
			e.noteEnvironment(r.ID, "Original Orca terminal creation is unverified")
		} else {
			e.finishEnvironment(r.ID, environmentcheck.Result{}, err, true)
		}
		return
	}
	if terminal.Handle != expected || terminal.WorktreeID != work.ID {
		e.noteEnvironment(r.ID, "Original Orca terminal creation receipt identity differs")
	}
}

func (e *Engine) bindEnvironmentTerminal(r *runstate.Record, terminal native.Terminal) error {
	state := r.EnvironmentInvocation
	if terminal.Handle != state.Terminal || terminal.IncarnationID == "" || terminal.WorktreeID != state.Worktree ||
		!environmentSamePath(terminal.WorktreePath, r.Snapshot.Ref.WorkspacePath) {
		return errors.New("original Orca terminal creation receipt identity differs")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[r.ID].Clone()
	next.EnvironmentInvocation.Incarnation = terminal.IncarnationID
	next.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_terminal", ExecutionHost: state.Host, RuntimeID: state.Runtime, WorktreeID: state.Worktree, Terminal: terminal.Handle, Incarnation: terminal.IncarnationID}
	bindEnvironmentStart(next, terminal.WorktreePath)
	return e.commitLocked(next, nil, "environment_terminal_started")
}

func (e *Engine) stopEnvironmentTerminal(ctx context.Context, r *runstate.Record, client native.EnvironmentTerminalClient, identity native.TerminalIdentity) {
	if r.EnvironmentInvocation.CloseAttempted {
		return
	}
	ctx = native.WithDispatchCheckpoint(ctx, func() error {
		e.mu.Lock()
		defer e.mu.Unlock()
		next := e.runs[r.ID].Clone()
		next.EnvironmentInvocation.CloseAttempted = true
		return e.commitLocked(next, nil, "environment_terminal_close_dispatched")
	})
	if err := client.Stop(ctx, identity); err != nil {
		e.noteEnvironment(r.ID, "Original Orca detection terminal stop is unverified")
		return
	}
	e.finishEnvironment(r.ID, environmentcheck.Result{}, errors.New("Environment invocation stopped before result collection"), true)
}

func (e *Engine) shutdownEnvironmentTerminal(id string, w *worker) {
	w.gate.Lock()
	defer w.gate.Unlock()
	r := e.environmentRecord(id)
	if r.StopStatus == "stopped" {
		return
	}
	if !r.EnvironmentInvocation.Attempted {
		return
	}
	state := r.EnvironmentInvocation
	if state.Terminal == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := native.EnvironmentTerminalClient{Environment: r.Request.Execution.Target.Environment, Runtime: state.Runtime, Host: state.Host}
	identity := native.TerminalIdentity{RuntimeID: state.Runtime, Handle: state.Terminal, IncarnationID: state.Incarnation}
	terminal, err := client.Terminal(ctx, identity)
	if err != nil || terminal.WorktreeID != state.Worktree || !environmentSamePath(terminal.WorktreePath, r.Snapshot.Ref.WorkspacePath) {
		return
	}
	identity.IncarnationID = terminal.IncarnationID
	if terminal.Exited() {
		if code, err := client.Exit(ctx, state.Terminal); err == nil && code != nil {
			e.finishEnvironment(id, environmentcheck.Result{}, errors.New("Runner stopped before result collection"), true)
		}
		return
	}
	e.stopEnvironmentTerminal(ctx, r, client, identity)
}
