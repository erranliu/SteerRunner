package application

import (
	"context"
	"io"
	"log"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

type shutdownPort struct {
	b.LifecyclePort
	stopped bool
}

func (p shutdownPort) ShutdownLifecycle(context.Context, b.LifecycleSnapshot, b.StopHandle) (b.LifecycleStateFacts, error) {
	return b.LifecycleStateFacts{Stopped: p.stopped}, nil
}

func TestSRShutdownSettlementUsesCommittedFinalNativeEvidence(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		t.Run(map[bool]string{false: "detached_unknown", true: "native_stopped"}[stopped], func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			e := &Engine{store: store, runs: map[string]*runstate.Record{}, logger: log.New(io.Discard, "", 0)}
			if err := e.commitLocked(&runstate.Record{ID: "original", StopStatus: "running"}, nil); err != nil {
				t.Fatal(err)
			}
			if e.ResourceSettlementError() == nil {
				t.Fatal("pre-shutdown projection was accepted")
			}
			e.closeWorker("original", &worker{port: shutdownPort{stopped: stopped}})
			if (e.ResourceSettlementError() == nil) != stopped {
				t.Fatal("shutdown ignored final persisted native evidence")
			}
		})
	}
}
