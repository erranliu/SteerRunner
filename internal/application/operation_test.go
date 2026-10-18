package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

type operationCommitPort struct {
	b.LifecyclePort
	executes, reconciles int
	requests             []b.LifecycleRequest
	stops                int
}

type partialReceiptPort struct{ operationCommitPort }

func (p *partialReceiptPort) Execute(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.executes++
	p.requests = append(p.requests, request.Clone())
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationUnknown, State: b.LifecycleStateFacts{SessionID: "original-session"}}, nil, errors.New("receipt lost after session creation")
}

func (p *partialReceiptPort) Reconcile(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	p.requests = append(p.requests, request.Clone())
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationUnknown}, nil, errors.New("original request still unavailable")
}

func TestSROperationReconciliationKeepsPartialBinding(t *testing.T) {
	e, w, _ := operationEngine(t)
	p := &partialReceiptPort{}
	w.port = p
	if e.advance("run", w) || e.runs["run"].Snapshot.Session.SessionID != "original-session" {
		t.Fatal("partial native receipt was not committed as an unknown original operation")
	}
	original := e.runs["run"].Intents[0].Request.Clone()
	for n := 0; n < 2; n++ {
		if e.advance("run", w) {
			t.Fatal("missing reconciliation facts reported progress")
		}
	}
	if p.executes != 1 || p.reconciles != 2 || len(e.runs["run"].Intents) != 1 {
		t.Fatal("unverified reconciliation resent or replaced the original effect")
	}
	for _, request := range p.requests[1:] {
		if request.Snapshot.Session.SessionID != "original-session" || request.Operation.ID != original.Operation.ID || request.Operation.IdempotencyKey != original.Operation.IdempotencyKey || request.Operation.PayloadDigest != original.Operation.PayloadDigest || string(request.Operation.Request) != string(original.Operation.Request) || *request.Operation.LifecycleIdentity != *original.Operation.LifecycleIdentity {
			t.Fatal("reconciliation lost partial facts or replaced its fixed request binding")
		}
	}
	runs, err := e.store.Runs()
	if err != nil || len(runs) != 1 || runs[0].Snapshot.Session.SessionID != "original-session" || runs[0].Intents[0].Request.Snapshot.Session.SessionID != "" || !runs[0].Intents[0].NeedsReconciliation() {
		t.Fatal("reconciliation erased durable partial facts or rewrote the intent snapshot", err)
	}
}

type gatedOperationPort struct {
	operationCommitPort
	entered chan struct{}
	release chan struct{}
}

func (p *gatedOperationPort) Execute(ctx context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	close(p.entered)
	select {
	case <-p.release:
		return p.operationCommitPort.Execute(ctx, request)
	case <-ctx.Done():
		return b.LifecycleReceipt{}, nil, ctx.Err()
	}
}

func TestSRStopSerializesWithNativeDispatch(t *testing.T) {
	e, w, _ := operationEngine(t)
	p := &gatedOperationPort{entered: make(chan struct{}), release: make(chan struct{})}
	w.port = p
	defer func() {
		select {
		case <-p.release:
		default:
			close(p.release)
		}
	}()
	advanced := make(chan bool, 1)
	go func() { advanced <- e.advance("run", w) }()
	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("native dispatch did not reach the barrier")
	}
	if w.gate.TryLock() {
		w.gate.Unlock()
		t.Fatal("native dispatch does not hold the revocation gate")
	}
	e.mu.Lock()
	// The in-flight receipt will commit once before this stop can acquire gate.
	expected := e.runs["run"].Revision + 1
	e.mu.Unlock()
	input := &contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-at-dispatch", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, expected))}
	stopped := make(chan contracts.Response, 1)
	go func() { stopped <- e.Handle("stop", "run", "", "", "", 0, false, input) }()
	select {
	case <-stopped:
		t.Fatal("stop acknowledged revocation while native dispatch held the gate")
	default:
	}
	close(p.release)
	select {
	case <-advanced:
	case <-time.After(5 * time.Second):
		t.Fatal("native dispatch did not settle")
	}
	select {
	case response := <-stopped:
		if !response.OK {
			t.Fatalf("stop was not accepted: %+v", response)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not acquire the released gate")
	}
	e.mu.Lock()
	revoked := e.runs["run"].Clone()
	e.mu.Unlock()
	if revoked.Allowed || !revoked.ResumeRequired || revoked.PendingStop == "" {
		t.Fatal("stop receipt preceded durable revocation")
	}
	for n := 0; n < 3; n++ {
		e.advance("run", w)
	}
	if p.executes != 1 || p.reconciles != 0 || len(e.runs["run"].Intents) != 1 {
		t.Fatal("a fresh input crossed the dispatch boundary after revocation")
	}
}

func (p *operationCommitPort) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismNotApplicable, Resource: b.MechanismApplicable, Session: b.MechanismNotApplicable}}
}
func (p *operationCommitPort) PreparationFacts(context.Context, b.LifecycleSnapshot) (b.PreparationFacts, error) {
	return b.PreparationFacts{Workspace: b.GateNotApplicable, Resource: b.GateRequired, ResourceRequired: &b.ResourceIntent{Purpose: "agent-start", Objective: "fixed"}, Session: b.GateNotApplicable}, nil
}
func (p *operationCommitPort) Execute(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.executes++
	p.requests = append(p.requests, request.Clone())
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{ResourceID: "original-resource"}}, nil, nil
}
func (p *operationCommitPort) Reconcile(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	p.requests = append(p.requests, request.Clone())
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{ResourceID: "original-resource"}}, nil, nil
}
func (p *operationCommitPort) StopLifecycle(_ context.Context, request b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.stops++
	p.requests = append(p.requests, request.Clone())
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true}}, nil
}

func operationEngine(t *testing.T) (*Engine, *worker, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	// A test-only SQLite trigger exercises the real transaction failure without
	// adding a persistence interface or fault switch to the production engine.
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	w := &worker{port: &operationCommitPort{}, wake: make(chan struct{}, 1)}
	e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}, logger: log.New(io.Discard, "", 0)}
	r := &runstate.Record{ID: "run", Allowed: true, Status: "running", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, CommandStates: map[string]string{}}
	r.Request.Workflow = contracts.Workflow{ID: "development", Version: 2}
	r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt"}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
	bindTestWorkflow(r)
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	return e, w, db
}

func failOperationCommit(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TRIGGER fail_operation_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'operation commit failed'); END;`); err != nil {
		t.Fatal(err)
	}
}

func TestSROperationIntentCommitFailureDoesNotDispatch(t *testing.T) {
	e, w, db := operationEngine(t)
	failOperationCommit(t, db)
	if e.advance("run", w) {
		t.Fatal("failed intent commit reported progress")
	}
	p := w.port.(*operationCommitPort)
	if p.executes != 0 || p.reconciles != 0 || len(e.runs["run"].Intents) != 0 {
		t.Fatal("uncommitted intent reached native dispatch or memory")
	}
	runs, err := e.store.Runs()
	if err != nil || len(runs) != 1 || len(runs[0].Intents) != 0 || len(runs[0].Snapshot.Operations) != 0 {
		t.Fatal("failed intent transaction left persisted operation facts", err)
	}
}

func TestSROperationReceiptCommitFailureReconcilesOriginal(t *testing.T) {
	e, w, db := operationEngine(t)
	i, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Objective: "fixed"}}})
	if err != nil {
		t.Fatal(err)
	}
	original := e.runs["run"].Intents[i].Request.Clone()
	failOperationCommit(t, db)
	if e.invoke("run", w, w.port, i, false) {
		t.Fatal("failed receipt commit reported progress")
	}
	if !e.runs["run"].Intents[i].NeedsReconciliation() {
		t.Fatal("uncommitted receipt settled the in-memory operation")
	}
	runs, err := e.store.Runs()
	if err != nil || len(runs) != 1 || !runs[0].Intents[i].NeedsReconciliation() || runs[0].Intents[i].Receipt != nil {
		t.Fatal("receipt commit failure left partial persisted success", err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
		t.Fatal(err)
	}
	if !e.advance("run", w) {
		t.Fatal("original operation was not reconciled")
	}
	p := w.port.(*operationCommitPort)
	if p.executes != 1 || p.reconciles != 1 || len(e.runs["run"].Intents) != 1 {
		t.Fatalf("receipt loss replayed the effect: execute=%d reconcile=%d", p.executes, p.reconciles)
	}
	for _, request := range p.requests {
		if request.Operation.ID != original.Operation.ID || request.Operation.IdempotencyKey != original.Operation.IdempotencyKey || request.Operation.PayloadDigest != original.Operation.PayloadDigest || string(request.Operation.Request) != string(original.Operation.Request) || request.Operation.LifecycleIdentity.InputBindingDigest != original.Operation.LifecycleIdentity.InputBindingDigest {
			t.Fatal("receipt loss replaced the original request or its binding")
		}
	}
	if e.runs["run"].Intents[i].Request.Operation.Status != b.OperationConfirmed {
		t.Fatal("original effect never settled")
	}
}

func TestSRStopBindingCommitFailureDoesNotDispatch(t *testing.T) {
	e, w, db := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "original-writer"}
	r.Allowed, r.ResumeRequired = false, true
	r.PendingStop = "stop-command"
	r.CommandStates[r.PendingStop] = "pending"
	bindTestWorkflow(r)
	if err := e.commitLocked(r, nil, "stop_accepted"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_stop_binding BEFORE INSERT ON events WHEN json_extract(NEW.body,'$.type')='stop_intent_bound' BEGIN SELECT RAISE(ABORT,'stop binding commit failed'); END;`); err != nil {
		t.Fatal(err)
	}
	e.advance("run", w)
	p := w.port.(*operationCommitPort)
	if p.stops != 0 || len(e.runs["run"].Intents) != 0 {
		t.Fatalf("failed stop binding dispatched or left an orphan intent: stops=%d intents=%d", p.stops, len(e.runs["run"].Intents))
	}
	runs, err := e.store.Runs()
	if err != nil || len(runs) != 1 || len(runs[0].Intents) != 0 || runs[0].PendingStop != "stop-command" {
		t.Fatal("failed stop binding left partial durable state", err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_stop_binding`); err != nil {
		t.Fatal(err)
	}
	e.advance("run", w)
	settled := e.runs["run"]
	if p.stops != 1 || len(settled.Intents) != 1 || settled.StopStatus != "stopped" || settled.Intents[0].Request.Operation.IdempotencyKey != "stop-command" || p.requests[0].Operation.IdempotencyKey != "stop-command" {
		t.Fatal("retry did not dispatch and settle exactly one command-bound stop")
	}
	if settled.Snapshot.Operations[0].IdempotencyKey == "stop-command" {
		t.Fatal("stop binding normalized the existing snapshot key representation")
	}
}
