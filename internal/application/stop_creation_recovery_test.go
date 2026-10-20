package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type lateCreationStopPort struct {
	b.LifecyclePort
	available                   bool
	resource                    b.ImplementationDataEnvelope
	starts, reconciles, effects int
	original                    b.LifecycleRequest
}

func (p *lateCreationStopPort) StopLifecycle(_ context.Context, q b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.starts++
	p.original = q.Clone()
	return b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown}, errors.New("original auxiliary creation receipt pending")
}

func (p *lateCreationStopPort) Reconcile(_ context.Context, q b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	r := b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown}
	if q.Operation.Action == string(b.LifecycleCreateResource) {
		if p.available {
			r.Disposition, r.Implementation = b.OperationConfirmed, []b.ImplementationDataEnvelope{p.resource}
			return r, nil, nil
		}
		return r, nil, errors.New("original auxiliary creation receipt pending")
	}
	p.reconciles++
	if q.Operation.ID != p.original.Operation.ID || q.Operation.PayloadDigest != p.original.Operation.PayloadDigest ||
		!reflect.DeepEqual(q.Operation.LifecycleIdentity, p.original.Operation.LifecycleIdentity) || q.Snapshot.Ref != p.original.Snapshot.Ref {
		return r, nil, errors.New("original stop binding changed")
	}
	if !reflect.DeepEqual(q.Snapshot.Implementation, []b.ImplementationDataEnvelope{p.resource}) {
		return r, nil, errors.New("original auxiliary resource binding missing from stop snapshot")
	}
	p.effects++
	r.Disposition, r.State.Stopped, r.State.WorkerAbsent = b.OperationConfirmed, true, true
	return r, nil, nil
}

func TestSRStopRecoversLateCreationFactsUnderOriginalIntent(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "changed-resource"}[foreign], func(t *testing.T) {
			e, w, _ := operationEngine(t)
			r := e.runs["run"].Clone()
			r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "terminal", Terminal: "original-main"}
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "native-control", Objective: "original"}}}); err != nil {
				t.Fatal(err)
			}
			stopping := e.runs["run"].Clone()
			stopping.Allowed, stopping.ResumeRequired, stopping.PendingStop = false, true, "original-stop"
			if err := e.commitLocked(stopping, nil); err != nil {
				t.Fatal(err)
			}
			s := r.Snapshot
			resource, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: 1, Purpose: b.ImplementationResourcePurpose, ImplementationKey: "test-original-control", ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}, []byte(`{"original_control":"original-handle"}`))
			if err != nil {
				t.Fatal(err)
			}
			p := &lateCreationStopPort{resource: resource}
			w.port = p
			e.stopRun("run", w, p)
			if p.starts != 1 || e.runs["run"].StopStatus == "stopped" {
				t.Fatal("stop did not retain original pending intent")
			}
			original := e.runs["run"].Intents[1].Request.Clone()
			p.available = true
			e.stopRun("run", w, p) // Commit the original creation receipt first.
			loaded, err := e.store.Runs()
			if err != nil || len(loaded) != 1 {
				t.Fatal(err)
			}
			e.runs["run"] = loaded[0]
			if foreign {
				e.runs["run"].Snapshot.Ref.External.Terminal = "foreign-main"
			}
			e.stopRun("run", w, p)
			actual := e.runs["run"]
			if (actual.StopStatus == "stopped") == foreign || p.starts != 1 || p.effects != map[bool]int{false: 1, true: 0}[foreign] || len(actual.Intents) != 2 || actual.Allowed || !actual.ResumeRequired {
				t.Fatal("late creation could not settle original stop or retargeted it", actual.StopStatus, p.starts, p.effects)
			}
			if !reflect.DeepEqual(actual.Intents[1].Request.Snapshot, original.Snapshot) || actual.Intents[1].Request.Operation.ID != original.Operation.ID {
				t.Fatal("stop recovery rewrote the persisted original intent")
			}
		})
	}
}
