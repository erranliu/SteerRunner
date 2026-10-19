package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestSRPreparationQueryStopAuthorization(t *testing.T) {
	for _, scenario := range []string{"cancel-blocked-query", "successful-query-races-stop", "failed-stop-commit", "stale-stop", "normal-query"} {
		t.Run(scenario, func(t *testing.T) {
			e, w, db := operationEngine(t)
			scope := e.setup(e.runs["run"], w).ReadOnlyPreparation
			entered := make(chan context.Context, 1)
			release := make(chan struct{}, 1)
			defer close(release)
			done := make(chan error, 1)
			go func() {
				w.gate.Lock()
				defer w.gate.Unlock()
				err := scope(context.Background(), func(ctx context.Context) error {
					entered <- ctx
					if scenario == "cancel-blocked-query" {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-release:
							return errors.New("test released blocked query")
						}
					}
					<-release
					return nil // Models a successful response concurrent with stop.
				})
				if w.gate.TryLock() {
					w.gate.Unlock()
					err = errors.New("query did not restore the dispatch gate")
				}
				if w.queryCancel != nil {
					err = errors.New("query left a live cancellation registration")
				}
				done <- err
			}()
			var queryCtx context.Context
			select {
			case queryCtx = <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("query did not start")
			}
			if !w.gate.TryLock() {
				t.Fatal("read-only query still holds the stop gate")
			}
			w.gate.Unlock()
			if scenario != "normal-query" {
				if scenario == "failed-stop-commit" {
					failOperationCommit(t, db)
				}
				e.mu.Lock()
				revision := e.runs["run"].Revision
				e.mu.Unlock()
				if scenario == "stale-stop" {
					revision++
				}
				input := &contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-query", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, revision))}
				stopped := make(chan contracts.Response, 1)
				go func() { stopped <- e.Handle("stop", "run", "", "", "", 0, false, input) }()
				select {
				case response := <-stopped:
					wantOK := scenario != "failed-stop-commit" && scenario != "stale-stop"
					if response.OK != wantOK {
						t.Fatal("stop response", response)
					}
					if wantOK != (queryCtx.Err() != nil) {
						t.Fatal("only a durably accepted stop may cancel the query", queryCtx.Err())
					}
				case <-time.After(time.Second):
					t.Fatal("stop waited for a read-only query")
				}
			}
			if scenario != "cancel-blocked-query" {
				// Buffering also bounds release if the query unexpectedly returned.
				// The deferred close unblocks it when an assertion fails.
				release <- struct{}{}
			}
			select {
			case err := <-done:
				wantCanceled := scenario == "cancel-blocked-query" || scenario == "successful-query-races-stop"
				if wantCanceled && !errors.Is(err, context.Canceled) || !wantCanceled && err != nil {
					t.Fatal("query authorization result", err)
				}
			case <-time.After(time.Second):
				t.Fatal("query did not terminate after stop/release")
			}
		})
	}
}

type preparationQueryPort struct {
	operationCommitPort
	scope   func(context.Context, func(context.Context) error) error
	entered chan struct{}
}

func (p *preparationQueryPort) Execute(ctx context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	err := p.scope(ctx, func(queryCtx context.Context) error {
		close(p.entered)
		<-queryCtx.Done()
		return queryCtx.Err()
	})
	if err != nil {
		return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationRejected}, nil, err
	}
	return p.operationCommitPort.Execute(ctx, request)
}

func TestSRPreparationQueryStopSettlesOriginalOperation(t *testing.T) {
	e, w, _ := operationEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e.ctx = ctx
	p := &preparationQueryPort{scope: e.setup(e.runs["run"], w).ReadOnlyPreparation, entered: make(chan struct{})}
	w.port = p
	done := make(chan bool, 1)
	go func() { done <- e.advance("run", w) }()
	select {
	case <-p.entered:
	case <-ctx.Done():
		t.Fatal("preparation did not reach the read-only scope")
	}
	e.mu.Lock()
	r := e.runs["run"].Clone()
	e.mu.Unlock()
	input := &contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-inventory", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, r.Revision))}
	if response := e.Handle("stop", "run", "", "", "", 0, false, input); !response.OK {
		t.Fatal(response)
	}
	e.mu.Lock()
	stopCommand := e.runs["run"].PendingStop
	e.mu.Unlock()
	select {
	case progress := <-done:
		if progress {
			t.Fatal("canceled query advanced to a mutation")
		}
	case <-ctx.Done():
		t.Fatal("query did not settle after accepted stop")
	}
	for n := 0; n < 3; n++ {
		e.advance("run", w)
	}
	stopped := e.runs["run"]
	if p.executes != 0 || p.reconciles != 0 || len(stopped.Intents) != 1 {
		t.Fatalf("stop replayed preparation: intents=%d executes=%d reconciles=%d", len(stopped.Intents), p.executes, p.reconciles)
	}
	op := stopped.Intents[0].Request.Operation
	if op.ID != r.Intents[0].Request.Operation.ID || op.Status != b.OperationRejected {
		t.Fatal("canceled preparation lost its original rejected operation", op)
	}
	if stopped.PendingStop != "" || stopped.StopStatus != "stopped" || stopped.Allowed || stopped.CommandStates[stopCommand] != "completed" {
		t.Fatal("canceled preparation lost the accepted stop command", stopped.PendingStop, stopped.StopStatus, stopped.CommandStates[stopCommand])
	}
}

func TestSRPreparationQueryRejectsRevokedOrChangedState(t *testing.T) {
	for _, scenario := range []string{"already-revoked", "revision-changed"} {
		t.Run(scenario, func(t *testing.T) {
			e, w, _ := operationEngine(t)
			w.gate.Lock()
			defer w.gate.Unlock()
			if scenario == "already-revoked" {
				e.runs["run"].Allowed = false
			}
			called := false
			err := e.preparationQuery("run", w, context.Background(), func(context.Context) error {
				called = true
				w.gate.Lock()
				defer w.gate.Unlock()
				e.mu.Lock()
				defer e.mu.Unlock()
				next := e.runs["run"].Clone()
				return e.commitLocked(next, nil, "test_changed_during_query")
			})
			if err == nil || called != (scenario == "revision-changed") {
				t.Fatal("query bypassed changed authorization", called, err)
			}
		})
	}
}
