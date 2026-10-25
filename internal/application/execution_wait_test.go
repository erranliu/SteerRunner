package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestSRExecutionWaitDurableProgressAndRealRevision(t *testing.T) {
	for _, mode := range []string{"success", "concurrent", "stop", "failed-commit", "exhausted"} {
		t.Run(mode, func(t *testing.T) {
			e, w, db := operationEngine(t)
			i, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecyclePrepareWorkspace, Input: b.LifecycleInput{Workspace: &b.WorkspaceIntent{}}})
			if err != nil {
				t.Fatal(err)
			}
			original := e.runs["run"].Intents[i].Request.Operation.ID
			w.gate.Lock()
			defer w.gate.Unlock()
			err = e.preparationQuery("run", w, context.Background(), func(ctx context.Context) error {
				progress := b.PreparationProgressFrom(ctx)
				now := time.Now().UTC()
				q, err := progress("runtime_status", func(q *b.PreparationQuery) error {
					q.Visible = true
					q.Wait = b.ExecutionWait{Contract: b.ExecutionWaitContract, Query: "runtime_status", Stage: "queue", ReasonCode: "orca_cli_busy", StartedAt: now, UpdatedAt: now, DeadlineAt: now.Add(time.Minute), MaxCalls: 5, Diagnostic: strings.Repeat("等", 600)}
					return nil
				})
				if err != nil {
					return err
				}
				if len(q.Wait.Diagnostic) > 1024 || q.Wait.OperationID != original {
					t.Fatal("public boundary", q)
				}
				if mode == "failed-commit" {
					failOperationCommit(t, db)
				}
				if mode == "stop" || mode == "concurrent" {
					w.gate.Lock()
					e.mu.Lock()
					next := e.runs["run"].Clone()
					if mode == "stop" {
						next.Allowed = false
						next.PendingStop = "durable-stop"
					} else {
						next.InputDigest = "concurrent-input"
					}
					commitErr := e.commitLocked(next, nil, "external-control")
					e.mu.Unlock()
					w.gate.Unlock()
					if commitErr != nil {
						t.Fatal(commitErr)
					}
				}
				_, err = progress("runtime_status", func(q *b.PreparationQuery) error {
					q.Wait.Stage, q.Wait.ReasonCode = "calling", "orca_query_running"
					q.Wait.CallsStarted++
					if mode == "exhausted" {
						q.Wait.Stage, q.Wait.ReasonCode = "exhausted", "query_attempts_exhausted"
						q.Wait.CallsStarted = 5
					}
					return nil
				})
				return err
			})
			if mode == "failed-commit" {
				_, _ = db.Exec(`DROP TRIGGER fail_operation_event`)
			}
			if mode == "success" || mode == "exhausted" {
				if err != nil {
					t.Fatal("own progress invalidated query", err)
				}
			} else if err == nil {
				t.Fatal("concurrent control or failed persistence authorized calling")
			}
			if mode == "concurrent" && !errors.Is(err, reliable.ErrStale) {
				t.Fatal(err)
			}
			if mode == "stop" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			rows, err := e.store.Runs()
			if err != nil || len(rows) != 1 {
				t.Fatal(err)
			}
			q := rows[0].Intents[i].PreparationQuery
			if q == nil || q.Wait.OperationID != original || q.Wait.DeadlineAt.IsZero() {
				t.Fatal("lost original budget")
			}
			if mode == "success" && (q.Visible || q.Wait.CallsStarted != 1 || rows[0].View().ExecutionWait != nil) {
				t.Fatal("clear reset query or exposed stale wait", q)
			}
			if mode == "exhausted" && (!q.Visible || rows[0].View().ExecutionWait.Stage != "exhausted") {
				t.Fatal("exhaustion not retained")
			}
			page, err := e.store.Events("run", "", 500)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range page.Events {
				raw, _ := json.Marshal(event)
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(raw, &fields)
				_, present := fields["execution_wait"]
				if present != (event.Type == "execution_wait_changed") {
					t.Fatal("event projection missing or leaked", event)
				}
			}
		})
	}
}

type waitingQueryPort struct {
	operationCommitPort
	scope   func(context.Context, func(context.Context) error) error
	entered chan struct{}
}

func (p *waitingQueryPort) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: b.MechanismNotApplicable, Session: b.MechanismNotApplicable}}
}
func (p *waitingQueryPort) PreparationFacts(context.Context, b.LifecycleSnapshot) (b.PreparationFacts, error) {
	return b.PreparationFacts{Workspace: b.GateRequired, Resource: b.GateNotApplicable, Session: b.GateNotApplicable}, nil
}

func (p *waitingQueryPort) Execute(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	err := p.scope(ctx, func(ctx context.Context) error {
		now := time.Now().UTC()
		_, err := b.PreparationProgressFrom(ctx)("runtime_status", func(q *b.PreparationQuery) error {
			q.Visible = true
			q.Wait = b.ExecutionWait{Contract: b.ExecutionWaitContract, Query: "runtime_status", Stage: "queue", ReasonCode: "orca_cli_busy", StartedAt: now, UpdatedAt: now, DeadlineAt: now.Add(time.Minute), MaxCalls: 5}
			return nil
		})
		if err != nil {
			return err
		}
		close(p.entered)
		<-ctx.Done()
		return ctx.Err()
	})
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}, nil, err
}

func TestSRExecutionWaitStopKeepsControlPriority(t *testing.T) {
	e, w, _ := operationEngine(t)
	i, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecyclePrepareWorkspace, Input: b.LifecycleInput{Workspace: &b.WorkspaceIntent{}}})
	if err != nil {
		t.Fatal(err)
	}
	p := &waitingQueryPort{scope: e.setup(e.runs["run"], w).ReadOnlyPreparation, entered: make(chan struct{})}
	done := make(chan bool, 1)
	go func() { w.gate.Lock(); defer w.gate.Unlock(); done <- e.invoke("run", w, p, i, false) }()
	select {
	case <-p.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("query did not persist queue")
	}
	e.mu.Lock()
	revision := e.runs["run"].Revision
	e.mu.Unlock()
	req := contracts.Request{Protocol: contracts.Protocol, RequestID: "typed-wait-stop", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, revision))}
	response := e.Handle("stop", "run", "", "", "", 0, false, &req)
	if !response.OK {
		t.Fatal(response)
	}
	e.mu.Lock()
	control := e.runs["run"].View()
	e.mu.Unlock()
	select {
	case progress := <-done:
		if progress {
			t.Fatal("canceled query advanced")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("query did not cancel")
	}
	e.mu.Lock()
	after := e.runs["run"].View()
	e.mu.Unlock()
	if after.ExecutionWait != nil || after.NewInputsAllowed || after.Phase != control.Phase || after.Status != control.Status || after.Diagnostic != control.Diagnostic {
		t.Fatal("late query result overwrote accepted stop", control, after)
	}
}

func TestSRExecutionWaitResumeClearsExhaustionWithoutResettingBudget(t *testing.T) {
	e, _, _ := operationEngine(t)
	i, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecyclePrepareWorkspace, Input: b.LifecycleInput{Workspace: &b.WorkspaceIntent{}}})
	if err != nil {
		t.Fatal(err)
	}
	next := e.runs["run"].Clone()
	now := time.Now().UTC()
	next.Intents[i].PreparationQuery = &b.PreparationQuery{Visible: true, Wait: b.ExecutionWait{Contract: b.ExecutionWaitContract, OperationID: next.Intents[i].Request.Operation.ID, Query: "runtime_status", Stage: "exhausted", ReasonCode: "query_attempts_exhausted", CallsStarted: 5, MaxCalls: 5, StartedAt: now.Add(-time.Minute), UpdatedAt: now, DeadlineAt: now.Add(time.Second)}}
	if err := e.commitLocked(next, nil, "execution_wait_changed"); err != nil {
		t.Fatal(err)
	}
	next = e.runs["run"].Clone()
	budget := next.Intents[i].PreparationQuery.Wait
	next.PendingResume = "existing-resume"
	if err := e.commitLocked(next, nil, "resume_requested"); err != nil {
		t.Fatal(err)
	}
	q := e.runs["run"].Intents[i].PreparationQuery
	if q.Visible || q.Wait != budget || e.runs["run"].View().ExecutionWait != nil {
		t.Fatal("resume retained display or reset original budget", q)
	}
	// A newer successful query must never reveal an older exhausted projection.
	next = e.runs["run"].Clone()
	next.Intents[i].PreparationQuery.Visible = true
	newer := next.Intents[i]
	copy := *q
	copy.Visible = false
	newer.PreparationQuery = &copy
	next.Intents = append(next.Intents, newer)
	if next.View().ExecutionWait != nil {
		t.Fatal("successful current query resurrected historical exhaustion")
	}
}

type queryRecoveryPort struct {
	operationCommitPort
	recorded, authorized bool
}

func (p *queryRecoveryPort) Reconcile(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.recorded, p.authorized = b.PreparationQueryRecorded(ctx), b.PreparationRecovery(ctx)
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}, nil, nil
}
func TestSRExecutionWaitRecoveryNormalizesRepresentationButChecksIdentity(t *testing.T) {
	for _, mode := range []string{"nil-empty", "target-change", "revoked", "receipt-present"} {
		t.Run(mode, func(t *testing.T) {
			e, w, _ := operationEngine(t)
			i, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecyclePrepareWorkspace, Input: b.LifecycleInput{Workspace: &b.WorkspaceIntent{}}})
			if err != nil {
				t.Fatal(err)
			}
			next := e.runs["run"].Clone()
			now := time.Now().UTC()
			next.Snapshot.Implementation = nil
			next.Intents[i].Request.Snapshot.Implementation = []b.ImplementationDataEnvelope{}
			next.Intents[i].PreparationQuery = &b.PreparationQuery{Wait: b.ExecutionWait{Contract: b.ExecutionWaitContract, OperationID: next.Intents[i].Request.Operation.ID, Query: "runtime_status", Stage: "queue", ReasonCode: "orca_cli_busy", StartedAt: now, UpdatedAt: now, DeadlineAt: now.Add(time.Minute), MaxCalls: 5}}
			if mode == "target-change" {
				next.Snapshot.Ref.Target.ID = "foreign-target"
			}
			if mode == "revoked" {
				next.Allowed = false
				next.PendingStop = "original-stop"
			}
			if mode == "receipt-present" {
				next.Intents[i].Receipt = &b.LifecycleReceipt{Operation: next.Intents[i].Request.Operation, Disposition: b.OperationUnknown}
			}
			if err := e.commitLocked(next, nil, "test_reloaded"); err != nil {
				t.Fatal(err)
			}
			p := &queryRecoveryPort{}
			w.gate.Lock()
			e.invoke("run", w, p, i, true)
			w.gate.Unlock()
			if p.recorded != (mode == "nil-empty" || mode == "revoked") || p.authorized != (mode == "nil-empty") {
				t.Fatal("query recovery confused representation, identity or authorization", mode, p.recorded, p.authorized)
			}
		})
	}
}
