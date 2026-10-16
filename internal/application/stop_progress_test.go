package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"io"
	"log"
	"testing"
)

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
