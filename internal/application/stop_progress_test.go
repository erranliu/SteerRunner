package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"io"
	"log"
	"testing"
	"time"
)

type cancellableStopBudgetPort struct {
	b.LifecyclePort
	entered chan context.Context
}

func (p *cancellableStopBudgetPort) StopLifecycle(ctx context.Context, q b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.entered <- ctx
	<-ctx.Done()
	return b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown}, ctx.Err()
}

func TestSRStopTransportQueueBudgetRemainsBoundedAndCancellable(t *testing.T) {
	for _, adapter := range []string{"orca-terminal", "codex-app-server"} {
		t.Run(adapter, func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e := &Engine{ctx: ctx, store: store, runs: map[string]*runstate.Record{}, logger: log.New(io.Discard, "", 0), launchOwner: func(context.Context, string) error { return nil }}
			r := &runstate.Record{ID: "run", Allowed: false, ResumeRequired: true, Status: "reconciling", PendingStop: "original-stop", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: adapter}}, CommandStates: map[string]string{}}
			r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt", External: b.ProcessIdentity{Kind: "terminal", Terminal: "original"}}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
			if err = e.commitLocked(r, nil, "accepted"); err != nil {
				t.Fatal(err)
			}
			port := &cancellableStopBudgetPort{entered: make(chan context.Context, 1)}
			done := make(chan struct{})
			go func() { defer close(done); e.stopRun(r.ID, &worker{port: port}, port) }()
			select {
			case attempt := <-port.entered:
				deadline, ok := attempt.Deadline()
				remaining := time.Until(deadline)
				if !ok || remaining <= 0 || remaining > 61*time.Second || adapter == "orca-terminal" && remaining < 45*time.Second || adapter != "orca-terminal" && remaining > 16*time.Second {
					t.Error("invalid bounded stop transport budget", remaining)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("original stop was not dispatched")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("service cancellation did not release stop")
			}
			if e.runs[r.ID].Allowed || !e.runs[r.ID].ResumeRequired || e.runs[r.ID].StopStatus == "stopped" || len(e.runs[r.ID].Intents) != 1 {
				t.Fatal("cancellation changed authorization or fabricated stop")
			}
		})
	}
}

type unavailableStopPort struct {
	b.LifecyclePort
	phase              string
	diagnostic         string
	stopped            bool
	starts, reconciles int
	original           string
}

func (p *unavailableStopPort) result(q b.LifecycleRequest) (b.LifecycleReceipt, error) {
	r := b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown}
	// Like the native adapter, return bounded owner progress rather than echoing
	// the request's journal receipt into another layer on every probe.
	r.Operation.Receipt = []byte(fmt.Sprintf(`{"phase":%q}`, p.phase))
	if p.stopped {
		r.Disposition, r.State.Stopped, r.State.WorkerAbsent = b.OperationConfirmed, true, true
		return r, nil
	}
	diagnostic := p.diagnostic
	if diagnostic == "" {
		diagnostic = "original runtime unavailable"
	}
	return r, errors.New(diagnostic)
}

func (p *unavailableStopPort) StopLifecycle(_ context.Context, q b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.starts++
	p.original = q.Operation.ID
	return p.result(q)
}

func (p *unavailableStopPort) Reconcile(_ context.Context, q b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	var saved b.LifecycleReceipt
	if q.Operation.ID != p.original || json.Unmarshal(q.Operation.Receipt, &saved) != nil || saved.ValidateFor(q) != nil {
		return b.LifecycleReceipt{}, nil, errors.New("original stop binding lost")
	}
	r, err := p.result(q)
	return r, nil, err
}

func TestSRUnreachableStopKeepsRevisionStableWithoutLosingProgress(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	p := &unavailableStopPort{phase: "not-attempted"}
	w := &worker{port: p, wake: make(chan struct{}, 1)}
	e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}, logger: log.New(io.Discard, "", 0)}
	r := &runstate.Record{ID: "run", Allowed: false, ResumeRequired: true, Status: "reconciling", PendingStop: "stop-command", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, CommandStates: map[string]string{}}
	r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt", External: b.ProcessIdentity{Kind: "terminal", Terminal: "original"}}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
	bindTestWorkflow(r)
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	// Settle the initial unknown outcome and its in-flight operation metadata.
	e.advance("run", w)
	e.advance("run", w)
	baseline := e.runs["run"].Clone()
	events, err := store.Events("run", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 5; n++ {
		if n == 2 {
			loaded, err := store.Runs()
			if err != nil || len(loaded) != 1 {
				t.Fatal("missing durable unknown stop", err)
			}
			e.runs["run"] = loaded[0]
		}
		// A real probe between inspect and reply must not starve the caller's CAS.
		view := e.runs["run"].View()
		e.advance("run", w)
		raw, _ := json.Marshal(contracts.ReplyInput{ExpectedRevision: view.Revision, Kind: "feedback", Text: "revoked input"})
		input := contracts.Request{Protocol: contracts.Protocol, RequestID: fmt.Sprintf("revoked-%d", n), Input: raw}
		response := e.Handle("reply", "run", "", "", "", 0, false, &input)
		if response.OK || response.Error == nil || response.Error.Code != "resume_required" {
			t.Fatalf("unchanged reconciliation starved revoked feedback: %+v", response.Error)
		}
	}
	actual := e.runs["run"]
	if actual.Revision != baseline.Revision || !actual.UpdatedAt.Equal(baseline.UpdatedAt) {
		t.Fatal("unchanged stop probes manufactured revisions")
	}
	after, err := store.Events("run", "", 100)
	if err != nil || len(after.Events) != len(events.Events) {
		t.Fatal("unchanged stop probes manufactured durable events", err)
	}
	p.diagnostic = "original close outcome unavailable"
	e.advance("run", w)
	if e.runs["run"].Revision <= baseline.Revision || e.runs["run"].Diagnostic != p.diagnostic {
		t.Fatal("a changed stop diagnostic was discarded")
	}
	// A changed, still-unknown owner receipt is genuine progress and must survive
	// reload under the original stop identity without reauthorizing input.
	p.phase = "attempted"
	e.advance("run", w)
	loaded, err := store.Runs()
	if err != nil || len(loaded) != 1 {
		t.Fatal("missing durable progress", err)
	}
	actual = loaded[0]
	var saved b.LifecycleReceipt
	if actual.Revision <= baseline.Revision || json.Unmarshal(actual.Intents[0].Request.Operation.Receipt, &saved) != nil || string(saved.Operation.Receipt) != `{"phase":"attempted"}` || actual.Allowed || !actual.ResumeRequired {
		t.Fatal("new stop progress was discarded or authorization changed")
	}
	e.runs["run"] = actual
	p.stopped = true
	e.advance("run", w)
	actual = e.runs["run"]
	if actual.StopStatus != "stopped" || actual.PendingStop != "" || actual.CommandStates["stop-command"] != "completed" || actual.Allowed || !actual.ResumeRequired || p.starts != 1 || p.reconciles != 9 || len(actual.Intents) != 1 || actual.Intents[0].Request.Operation.ID != p.original {
		t.Fatal("restored connectivity lost original stop completion or repeated dispatch")
	}
	// An actually stale caller still fails CAS after stop completion.
	raw, _ := json.Marshal(contracts.ReplyInput{ExpectedRevision: baseline.Revision, Kind: "feedback", Text: "stale input"})
	input := contracts.Request{Protocol: contracts.Protocol, RequestID: "stale-revoked", Input: raw}
	response := e.Handle("reply", "run", "", "", "", 0, false, &input)
	if response.OK || response.Error == nil || response.Error.Code != "stale_revision" {
		t.Fatalf("stale revision check was weakened: %+v", response)
	}
}

type stopProgressPort struct {
	b.LifecyclePort
	foreign            bool
	starts, reconciles int
	original           string
}

func (p *stopProgressPort) StopLifecycle(_ context.Context, q b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.starts++
	p.original = q.Operation.ID
	r := b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown}
	r.Operation.Receipt = []byte(`{"phase":"before-effect"}`)
	if p.foreign {
		r.Operation.ID = "foreign"
	}
	return r, errors.New("offline before effect")
}
func (p *stopProgressPort) Reconcile(_ context.Context, q b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	r := b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown}
	var saved b.LifecycleReceipt
	if q.Operation.ID != p.original || json.Unmarshal(q.Operation.Receipt, &saved) != nil || saved.ValidateFor(q) != nil || string(saved.Operation.Receipt) != `{"phase":"before-effect"}` {
		return r, nil, errors.New("unverified progress")
	}
	r.Disposition = b.OperationConfirmed
	r.State.Stopped = true
	r.State.WorkerAbsent = true
	return r, nil, nil
}

func TestSRStopProgressSurvivesStorageReloadUnderOriginalOperation(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "foreign-receipt"}[foreign], func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, logger: log.New(io.Discard, "", 0)}
			r := &runstate.Record{ID: "run", Allowed: false, ResumeRequired: true, Status: "reconciling", PendingStop: "stop-command", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, CommandStates: map[string]string{}}
			r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt", External: b.ProcessIdentity{Kind: "terminal", Terminal: "original"}}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
			if err = e.commitLocked(r, nil, "accepted"); err != nil {
				t.Fatal(err)
			}
			port := &stopProgressPort{foreign: foreign}
			w := &worker{port: port}
			e.stopRun(r.ID, w, port)
			loaded, err := store.Runs()
			if err != nil || len(loaded) != 1 {
				t.Fatal("durable stop missing", err)
			}
			e.runs[r.ID] = loaded[0]
			e.stopRun(r.ID, &worker{port: port}, port)
			actual := e.runs[r.ID]
			if port.starts != 1 || port.reconciles != 1 || len(actual.Intents) != 1 || actual.Intents[0].Request.Operation.ID != port.original || actual.Allowed || !actual.ResumeRequired {
				t.Fatal("stop identity or authorization changed")
			}
			if (actual.StopStatus == "stopped") == foreign {
				t.Fatal("original progress lost or foreign progress trusted", actual.StopStatus)
			}
		})
	}
}
