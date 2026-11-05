package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Only the external Orca command responses are replaced. The request freeze,
// candidate profile/state, actual Orca adapter, operations and SQLite commits
// are the production implementations. No business model is launched here.
type sharedNativeWire struct {
	b.ManagedProcessHost
	record     *runstate.Record
	worktree   native.Worktree
	live       bool
	mode       string
	closes     int
	sends      int
	afterClose func()
	beforeShow func()
}

type candidateIdlePort struct {
	b.LifecyclePort
	idle    bool
	queries int
}

func TestSharedNativeBlockedRunRetainsFirstStopAuthority(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			e, w, wire, _, id := sharedNativeEngine(t, environment)
			r := e.runs[id]
			r.Allowed, r.Status = false, "blocked"
			response := e.Handle("stop", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "blocked-original-stop", Input: canonical(contracts.RevisionInput{ExpectedRevision: r.Revision})})
			if !response.OK {
				t.Fatal("blocked inputs lost original stop authority", response)
			}
			e.stopRun(id, w, w.port)
			if e.runs[id].StopStatus != "stopped" || wire.closes != 1 || wire.sends != 0 {
				t.Fatal("blocked original resources were not stopped once", wire.closes, e.runs[id].View())
			}
		})
	}
}

func (p *candidateIdlePort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	p.queries++
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, ResourceAvailable: true, SessionIdle: p.idle}, nil
}

func TestSharedNativeBusinessFeedbackAdmission(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		for _, idle := range []bool{false, true} {
			t.Run(environment+map[bool]string{true: "/idle", false: "/active"}[idle], func(t *testing.T) {
				e, w, wire, _, id := sharedNativeEngine(t, environment)
				for _, step := range []string{workflow.StepImplement, workflow.StepPRReady} {
					r := e.runs[id]
					r.Status, r.Workflow.StepID = "waiting_input", step
					response := e.Handle("reply", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "feedback-reject-" + step, Input: canonical(contracts.ReplyInput{ExpectedRevision: r.Revision, Kind: "feedback", Text: "Continue implementation"})})
					if response.OK || response.Error.Code != "invalid_state" || e.runs[id].PendingFeedback != nil || wire.sends != 0 {
						t.Fatal("active native question or milestone acquired a feedback edge", response)
					}
				}
				r := e.runs[id]
				r.Status, r.Workflow.StepID = "waiting_input", workflow.StepImplementInput
				before, session := len(r.Workflow.StageInputs), r.Snapshot.Session.SessionID
				response := e.Handle("reply", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "business-feedback", Input: canonical(contracts.ReplyInput{ExpectedRevision: r.Revision, Kind: "feedback", Text: "Continue implementation"})})
				if !response.OK || e.runs[id].PendingFeedback == nil || wire.sends != 0 {
					t.Fatal("frozen implementation input edge rejected", response)
				}
				port := &candidateIdlePort{idle: idle}
				if continued := e.feedback(id, w, port); continued != idle || port.queries != 1 || wire.sends != 0 || e.runs[id].Snapshot.Session.SessionID != session {
					t.Fatal("feedback bypassed original idle qualification", continued, port.queries)
				}
				want := before
				if idle {
					want++
				}
				if len(e.runs[id].Workflow.StageInputs) != want {
					t.Fatal("feedback froze an unexpected number of inputs")
				}
			})
		}
	}
}

func (h *sharedNativeWire) OutputCommand(_ context.Context, command b.LaunchCommand) ([]byte, error) {
	args := command.Args
	var result any
	runtime := "candidate-runtime"
	row := native.Terminal{Handle: "candidate-terminal", IncarnationID: "candidate-incarnation", WorktreeID: h.worktree.ID, WorktreePath: h.worktree.Path, Branch: h.worktree.Branch, Connected: true, Status: "running", AgentIdentity: "codex"}
	if len(args) < 2 {
		return nil, errors.New("unexpected non-Orca command")
	}
	switch args[0] + " " + args[1] {
	case "status --json":
		result = map[string]any{}
	case "repo list":
		name := filepath.Base(h.record.Request.Repository.LocalPath)
		result = map[string]any{"repos": []any{map[string]any{"id": "candidate-repo", "path": "/registered/" + name, "kind": "git"}}, "truncated": false}
	case "worktree create":
		result = map[string]any{"worktree": h.worktree, "agentTerminalHandle": row.Handle}
	case "terminal show":
		if h.beforeShow != nil {
			h.beforeShow()
		}
		result = map[string]any{"terminal": row}
	case "terminal list":
		rows := []native.Terminal{}
		if h.live {
			if h.closes > 0 && h.mode == "replacement" {
				row.IncarnationID = "replacement"
			}
			rows = append(rows, row)
		}
		hosts := []string{"local"}
		if h.closes > 0 && h.mode == "missing-host" {
			hosts = []string{"other"}
		}
		if h.closes > 0 && h.mode == "wrong-runtime" {
			runtime = "foreign-runtime"
		}
		if h.closes > 0 && h.mode == "query-failed" {
			return nil, errors.New("original inventory unavailable")
		}
		total := len(rows)
		if h.closes > 0 && h.mode == "wrong-total-count" {
			total++
		}
		result = map[string]any{"terminals": rows, "totalCount": total, "truncated": h.closes > 0 && h.mode == "truncated", "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": []string{}}}
	case "terminal close":
		h.closes++
		if h.mode != "lost-live" && h.mode != "replacement" {
			h.live = false
		}
		if h.afterClose != nil {
			h.afterClose()
		}
		if strings.HasPrefix(h.mode, "lost-") {
			return nil, errors.New("native close response lost after invocation")
		}
		result = map[string]any{"close": map[string]any{"handle": row.Handle, "ptyKilled": true}}
	case "terminal send":
		h.sends++
		return nil, errors.New("business send must not cross stop")
	default:
		return nil, errors.New("unexpected Orca operation: " + strings.Join(args[:2], " "))
	}
	return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}})
}

func sharedNativeEngine(t *testing.T, environment string) (*Engine, *worker, *sharedNativeWire, *sql.DB, string) {
	t.Helper()
	// This fixture owns no real Orca resource. Give its production CLI mutex a
	// private cache so a real user CLI or another package cannot spend its budget.
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "cache"))
	f := newBaseRefreshFixture(t, "origin", "main")
	exe, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	settings := contracts.OrcaSettings{AccessMode: "full_access"}
	settings.Transport.Executable = exe
	target := contracts.Target{Kind: "local"}
	if environment != "" {
		target = contracts.Target{Kind: "orca_environment", Environment: environment}
	}
	f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
	f.input.Input = canonical(f.request)
	r := f.freeze()
	// Replay the historical frozen terminal contract without admitting a new
	// terminal-backed Codex Run through production configuration.
	r.Request.Execution.Target = target
	r.InputDigest = reliable.Digest(canonical(r.Request))
	profile := orca.Profile{NativeTerminal: true, Executable: exe, Environment: environment, WaitTimeout: "30s"}
	r.Snapshot.Ref.Protocol, r.StopEvidenceScope = b.ProtocolOrcaTerminalV3, "orca_terminal"
	r.Snapshot.Ref.Target, r.Snapshot.Session.Target = profile.Target(), profile.Target()
	r.Snapshot.ConfigurationDigest = r.InputDigest
	envelope, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: orca.ProfileKey, CodecVersion: orca.NativeProfileVersion, Purpose: b.ImplementationConfigPurpose, ProjectID: r.Snapshot.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}, canonical(profile))
	if err != nil {
		t.Fatal(err)
	}
	r.Profile = []b.ImplementationDataEnvelope{envelope}
	store, err := storage.Open(f.state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	e, err := New(context.Background(), f.state, store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.cancel)
	w := &worker{wake: make(chan struct{}, 1)}
	e.workers[r.ID] = w
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	host := &sharedNativeWire{ManagedProcessHost: process.New(time.Second), record: r, live: true, worktree: native.Worktree{ID: "candidate-worktree", InstanceID: "candidate-instance", RepoID: "candidate-repo", Path: filepath.Join(f.source, "candidate-worktree"), Head: r.Snapshot.WorkspacePlan.BaseCommit, BaseRef: r.Snapshot.WorkspacePlan.BaseCommit, Branch: "candidate-branch", DisplayName: r.Snapshot.Session.NamePrefix + "-" + r.ID}}
	setup := e.setup(r, w)
	setup.ProcessHost = host
	// Stop never consumes a state feed; fail if a test accidentally launches or
	// observes a model. The native CLI stop implementation remains unchanged.
	port, err := orca.NewLifecycleWithStateFactory(setup, func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
		return nil, errors.New("model state feed is outside controlled stop fixture")
	})
	if err != nil {
		t.Fatal(err)
	}
	w.port = port
	index, err := e.saveIntent(r.ID, e.runs[r.ID].Clone(), b.LifecycleCandidate{Action: b.LifecyclePrepareWorkspace, Input: b.LifecycleInput{Workspace: &r.Snapshot.WorkspacePlan}})
	if err != nil {
		t.Fatal(err)
	}
	w.gate.Lock()
	prepared := e.invoke(r.ID, w, port, index, false)
	w.gate.Unlock()
	if !prepared || e.runs[r.ID].Snapshot.Ref.External.Terminal != "candidate-terminal" || e.runs[r.ID].Snapshot.Implementation[0].CodecVersion != orca.NativeStateVersion {
		t.Fatal("original production prepare/receipt failed", e.runs[r.ID].View())
	}
	// The controlled preparation never opens model RPC. Once its original
	// resource receipt is persisted, stop uses the actual production constructor
	// including its CLI FIFO, timeout and query implementation.
	stopSetup := e.setup(e.runs[r.ID], w)
	stopSetup.ProcessHost = host
	w.port, err = orca.NewLifecycle(stopSetup)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(f.state, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return e, w, host, db, r.ID
}

func TestSharedNativeEngineStopReconciliation(t *testing.T) {
	// Verify durable stop/recovery effects, allowing SQLite and the real CLI
	// queue to run under load. Queue deadline behavior has its own test below.
	for _, environment := range []string{"", "paired-worker"} {
		for _, mode := range []string{"lost-absent", "lost-live", "replacement", "missing-host", "wrong-runtime", "wrong-total-count", "truncated", "query-failed", "receipt-commit-failed"} {
			t.Run(environment+"/"+mode, func(t *testing.T) {
				e, w, wire, db, id := sharedNativeEngine(t, environment)
				wire.mode = mode
				if mode == "receipt-commit-failed" {
					wire.afterClose = func() { failOperationCommit(t, db) }
				}
				input := canonical(contracts.RevisionInput{ExpectedRevision: e.runs[id].Revision})
				response := e.Handle("stop", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "original-candidate-stop", Input: input})
				var accepted struct {
					CommandID string `json:"command_id"`
				}
				_ = json.Unmarshal(canonical(response.Data), &accepted)
				if !response.OK || accepted.CommandID == "" || e.runs[id].Allowed || e.runs[id].PendingStop != accepted.CommandID {
					t.Fatal("stop did not durably revoke original Run", response)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				e.ctx = ctx
				e.stopRun(id, w, w.port)
				cancel()
				if wire.closes != 1 || wire.sends != 0 {
					t.Fatal("first stop invoked wrong effects", wire.closes, wire.sends, e.runs[id].Diagnostic)
				}
				if mode == "receipt-commit-failed" {
					if e.runs[id].StopStatus == "stopped" {
						t.Fatal("failed receipt transaction published stopped")
					}
					if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
						t.Fatal(err)
					}
				}
				runs, err := e.store.Runs()
				if err != nil || len(runs) != 1 {
					t.Fatal(err)
				}
				e.runs[id] = runs[0]
				if mode != "lost-absent" {
					input = canonical(contracts.RevisionInput{ExpectedRevision: e.runs[id].Revision})
					replacement := e.Handle("stop", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "replacement-candidate-stop", Input: input})
					if replacement.OK || e.runs[id].PendingStop != accepted.CommandID || wire.closes != 1 {
						t.Fatal("new request replaced unresolved original stop", replacement)
					}
				}
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				e.ctx = ctx
				e.stopRun(id, w, w.port)
				settled := mode == "lost-absent" || mode == "receipt-commit-failed"
				if wire.closes != 1 || wire.sends != 0 || (e.runs[id].StopStatus == "stopped") != settled || e.runs[id].Allowed || !settled && e.runs[id].PendingStop != accepted.CommandID {
					t.Fatal("reconciliation repeated close or overstated stop", wire.closes, wire.sends, e.runs[id].View())
				}
				input = canonical(contracts.ReplyInput{ExpectedRevision: e.runs[id].Revision, Kind: "feedback", Text: "No new input after original stop"})
				if reply := e.Handle("reply", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "late-candidate-feedback", Input: input}); reply.OK || wire.sends != 0 {
					t.Fatal("late feedback restored dispatch authority", reply)
				}
			})
		}
	}
}

// SerializeCLI first observes Done after registering its FIFO ticket. This
// barrier lets the test place a real queue owner between show and close.
type observedQueueContext struct {
	context.Context
	seen chan struct{}
	once sync.Once
}

func (c *observedQueueContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.seen) })
	return c.Context.Done()
}

func TestSharedNativeStopQueuePreservesFirstDispatch(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			e, w, wire, _, id := sharedNativeEngine(t, environment)
			input := canonical(contracts.RevisionInput{ExpectedRevision: e.runs[id].Revision})
			response := e.Handle("stop", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "queued-candidate-stop", Input: input})
			if !response.OK {
				t.Fatal(response)
			}
			showEntered, releaseShow := make(chan struct{}), make(chan struct{})
			var releaseShowOnce, releaseQueueOnce sync.Once
			holding, releaseQueue, queueDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			t.Cleanup(func() {
				releaseShowOnce.Do(func() { close(releaseShow) })
				releaseQueueOnce.Do(func() { close(releaseQueue) })
			})
			var showOnce sync.Once
			wire.beforeShow = func() { showOnce.Do(func() { close(showEntered); <-releaseShow }) }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e.ctx = ctx
			stopped := make(chan struct{})
			go func() { e.stopRun(id, w, w.port); close(stopped) }()
			select {
			case <-showEntered:
			case <-time.After(5 * time.Second):
				t.Fatal("original show did not enter")
			}
			queued := &observedQueueContext{Context: context.Background(), seen: make(chan struct{})}
			go func() {
				defer close(queueDone)
				_, _ = native.SerializeCLI(queued, func(context.Context) ([]byte, error) { close(holding); <-releaseQueue; return nil, nil })
			}()
			select {
			case <-queued.seen:
			case <-time.After(5 * time.Second):
				t.Fatal("queue ticket did not register")
			}
			releaseShowOnce.Do(func() { close(releaseShow) })
			select {
			case <-holding:
			case <-time.After(5 * time.Second):
				t.Fatal("real CLI queue owner did not acquire")
			}
			cancel()
			select {
			case <-stopped:
			case <-time.After(5 * time.Second):
				t.Fatal("queued close did not cancel")
			}
			releaseQueueOnce.Do(func() { close(releaseQueue) })
			<-queueDone
			if wire.closes != 0 || e.runs[id].StopStatus == "stopped" {
				t.Fatal("queued close invoked or overstated stop", wire.closes, e.runs[id].View())
			}
			e.ctx = context.Background()
			e.stopRun(id, w, w.port)
			if wire.closes != 1 || e.runs[id].StopStatus != "stopped" || wire.sends != 0 {
				t.Fatal("original never-dispatched close lost first authorization", wire.closes, e.runs[id].View())
			}
		})
	}
}

// Exercise the real adapter stop and production commit before losing the host.
func TestSharedNativeConfirmedStopSurvivesOriginalHostLoss(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			e, w, wire, _, id := sharedNativeEngine(t, environment)
			r := e.runs[id]
			response := e.Handle("stop", id, "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "original-stop", Input: canonical(contracts.RevisionInput{ExpectedRevision: r.Revision})})
			if !response.OK {
				t.Fatal(response)
			}
			e.stopRun(id, w, w.port)
			stopped := e.runs[id].Clone()
			if !confirmedOrcaTerminalStop(stopped, "orca_terminal") || wire.closes != 1 {
				t.Fatal("native stop did not persist usable proof", stopped.View())
			}
			before := canonical(stopped)
			wire.mode = "query-failed"
			proof := VerifyStopped(context.Background(), e.dir, []*runstate.Record{stopped})
			if !proof.ResourcesSettled || len(proof.Runs) != 1 || proof.Runs[0].StopEvidenceScope != "orca_terminal" || wire.closes != 1 || wire.sends != 0 {
				t.Fatal("lost confirmed stop or widened its scope", proof)
			}
			if string(before) != string(canonical(stopped)) {
				t.Fatal("verification rewrote original stop")
			}
			for _, intent := range stopped.Intents {
				if intent.Request.Operation.Action != string(b.LifecycleStopExecution) {
					continue
				}
				replay, _, err := w.port.Reconcile(context.Background(), intent.Request.Clone())
				if err != nil || replay.Disposition != b.OperationConfirmed || !replay.State.Stopped {
					t.Fatal("confirmed stop reconciliation queried lost host", replay, err)
				}
			}
		})
	}
}
