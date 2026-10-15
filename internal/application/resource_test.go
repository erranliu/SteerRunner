package application

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

type pendingResourcePort struct {
	b.LifecyclePort
	confirm  bool
	executes int
	observed []string
}

func (p *pendingResourcePort) Execute(_ context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.executes++
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}, nil, nil
}
func (p *pendingResourcePort) Reconcile(_ context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.observed = append(p.observed, r.Operation.ID)
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationAccepted, State: b.LifecycleStateFacts{ResourceID: "original-terminal", ResourcePending: true}}
	if p.confirm {
		receipt.Disposition = b.OperationConfirmed
		receipt.State.ResourcePending = false
	}
	return receipt, nil, nil
}

func TestSRAcceptedResourceUsesOriginalOperation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "deadline"}[expired], func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, logger: log.New(io.Discard, "", 0)}
			r := &runstate.Record{ID: "run", Allowed: true, Status: "running", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, CommandStates: map[string]string{}}
			r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt"}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
			if err := e.commitLocked(r, nil, "accepted"); err != nil {
				t.Fatal(err)
			}
			i, err := e.saveIntent(r.ID, r, b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Objective: "fixed original launch"}}})
			if err != nil {
				t.Fatal(err)
			}
			r = e.runs[r.ID].Clone()
			r.Intents[i].Request.Operation.Status = b.OperationAccepted
			r.Intents[i].Receipt = &b.LifecycleReceipt{Operation: r.Intents[i].Request.Operation, Disposition: b.OperationAccepted, State: b.LifecycleStateFacts{ResourceID: "original-terminal", ResourcePending: true}}
			r.Snapshot.Operations[i].Status = b.OperationAccepted
			if expired {
				r.PreparationDeadline = time.Now().Add(-time.Second)
			}
			if err := e.commitLocked(r, nil, "resource_accepted"); err != nil {
				t.Fatal(err)
			}
			original := r.Intents[i].Request.Operation.ID
			port := &pendingResourcePort{}
			worker := &worker{port: port}
			for j := 0; j < 3; j++ {
				if e.advance(r.ID, worker) {
					t.Fatal("pending resource caused an immediate new preparation loop")
				}
			}
			if len(e.runs[r.ID].Intents) != 1 || port.executes != 0 || len(port.observed) != 3 {
				t.Fatal("accepted resource was recreated", len(e.runs[r.ID].Intents), port.executes, port.observed)
			}
			for _, id := range port.observed {
				if id != original {
					t.Fatal("reconciliation replaced its original operation identity")
				}
			}
			port.confirm = true
			if !e.advance(r.ID, worker) || e.runs[r.ID].Intents[0].Request.Operation.Status != b.OperationConfirmed {
				t.Fatal("original resource readiness was not reconciled")
			}
			if expired && (e.runs[r.ID].Allowed || !e.runs[r.ID].ResumeRequired) {
				t.Fatal("late readiness revived an expired input authorization")
			}
		})
	}
}
