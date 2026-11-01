package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type scopedStopCommitPort struct {
	operationCommitPort
	afterStop func()
}

func (p *scopedStopCommitPort) StopLifecycle(ctx context.Context, r b.LifecycleRequest, h b.StopHandle) (b.LifecycleReceipt, error) {
	receipt, err := p.operationCommitPort.StopLifecycle(ctx, r, h)
	p.afterStop()
	return receipt, err
}
func (p *scopedStopCommitPort) Reconcile(_ context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}}, nil, nil
}

func TestLocalCodexStopLostReceiptCommitOnlyReconciles(t *testing.T) {
	e, w, db := operationEngine(t)
	r := e.runs["run"].Clone()
	r.StopEvidenceScope = orca.LocalCodexStopScope
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_terminal", Terminal: "original"}
	r.PendingStop = "stop-command"
	r.CommandStates[r.PendingStop] = "pending"
	r.Allowed = false
	if err := e.commitLocked(r, nil, "fixture"); err != nil {
		t.Fatal(err)
	}
	p := &scopedStopCommitPort{afterStop: func() { failOperationCommit(t, db) }}
	w.port = p
	e.stopRun("run", w, p)
	if p.stops != 1 || e.runs["run"].StopStatus == "stopped" {
		t.Fatal("failed receipt transaction was published")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
		t.Fatal(err)
	}
	runs, err := e.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	e.runs["run"] = runs[0]
	e.stopRun("run", w, p)
	if p.stops != 1 || p.reconciles != 1 || e.runs["run"].StopStatus != "stopped" {
		t.Fatal("lost receipt caused another effect", p.stops, p.reconciles)
	}
}

func TestLocalCodexStopCommittedWithoutReceiptOnlyReconciles(t *testing.T) {
	e, w, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.StopEvidenceScope = orca.LocalCodexStopScope
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_terminal", Terminal: "original"}
	r.PendingStop = "stop-command"
	r.Allowed = false
	if err := e.commitLocked(r, nil, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.saveIntent("run", r, b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{}}}); err != nil {
		t.Fatal(err)
	}
	// Reload the committed aggregate, exactly the crash window before a receipt.
	runs, err := e.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	e.runs["run"] = runs[0]
	p := w.port.(*operationCommitPort)
	e.stopRun("run", w, p)
	if p.stops != 0 || p.reconciles != 1 {
		t.Fatal("unreceipted stop was replayed", p.stops, p.reconciles)
	}
}

func TestLocalCodexStopNewRequestCannotReplacePendingStop(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.StopEvidenceScope = orca.LocalCodexStopScope
	r.PendingStop = "original-stop"
	r.Allowed = false
	if err := e.commitLocked(r, nil, "fixture"); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision})
	response := e.Handle("stop", "run", "", "", "", 0, false, &contracts.Request{Protocol: contracts.Protocol, RequestID: "another-stop", Input: input})
	if response.OK || response.Error == nil || response.Error.Code != "operation_unverified" || e.runs["run"].PendingStop != "original-stop" {
		t.Fatal("new stop replaced unresolved original", response)
	}
	if e.runs["run"].View().StopEvidenceScope != orca.LocalCodexStopScope {
		t.Fatal("scope projection changed")
	}
}

type scopedResourcePort struct {
	operationCommitPort
	live map[string]bool
}

func (p *scopedResourcePort) StopLifecycle(_ context.Context, r b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.stops++
	p.live[r.Snapshot.Ref.External.Terminal] = false
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}}, nil
}

func TestLocalCodexStopPreservesOtherRunsOriginalResource(t *testing.T) {
	e, w, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.StopEvidenceScope = orca.LocalCodexStopScope
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_terminal", Terminal: "original"}
	other := r.Clone()
	other.ID = "other-run"
	other.Snapshot.Ref.RunID = other.ID
	other.Snapshot.Ref.AttemptID = "other-attempt"
	other.Snapshot.Session.ExecutionID = other.ID
	other.Snapshot.Session.WorkflowRunID = other.ID
	other.Snapshot.Session.AttemptID = "other-attempt"
	other.Snapshot.ResourceGeneration = "other-generation"
	other.Snapshot.Ref.External.Terminal = "other-original"
	if err := e.commitLocked(other, nil, "fixture"); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(e.runs[other.ID])
	r.PendingStop, r.Allowed = "stop-command", false
	if err := e.commitLocked(r, nil, "fixture"); err != nil {
		t.Fatal(err)
	}
	p := &scopedResourcePort{live: map[string]bool{"original": true, "other-original": true}}
	w.port = p
	e.workers[other.ID] = &worker{port: p, wake: make(chan struct{}, 1)}
	e.stopRun("run", w, p)
	after, _ := json.Marshal(e.runs[other.ID])
	if p.stops != 1 || p.live["original"] || !p.live["other-original"] || string(before) != string(after) || e.runs["run"].StopStatus != "stopped" {
		t.Fatal("stop crossed Run/resource binding", p.live)
	}
}

func TestLocalCodexOfflineStoppedWithoutResourceRejectsUnknownCreation(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.InputDigest = r.Snapshot.ConfigurationDigest
	r.StopEvidenceScope, r.StopStatus = orca.LocalCodexStopScope, "stopped"
	profile := orca.Profile{Executable: "orca.exe", WaitTimeout: "1s", RuntimeExecutable: "helper.exe"}
	r.Snapshot.Ref.Backend, r.Snapshot.Ref.Protocol = b.BackendOrca, b.ProtocolOrcaTerminalV3
	r.Snapshot.Ref.Agent.Name, r.Snapshot.Ref.Target = "codex", profile.Target()
	raw, _ := json.Marshal(profile)
	envelope, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.ProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.ProfileKey, ProjectID: r.Snapshot.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}, raw)
	if err != nil {
		t.Fatal(err)
	}
	r.Profile = []b.ImplementationDataEnvelope{envelope}
	if err := e.commitLocked(r, nil, "fixture"); err != nil {
		t.Fatal(err)
	}
	runs, _ := e.store.Runs()
	if v := VerifyStopped(context.Background(), "unused", runs); !v.ResourcesSettled {
		t.Fatal("positive never-created stop rejected", v)
	}
	if _, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "native-control", Objective: "original"}}}); err != nil {
		t.Fatal(err)
	}
	runs, _ = e.store.Runs()
	if v := VerifyStopped(context.Background(), "unused", runs); v.ResourcesSettled {
		t.Fatal("saved stopped hid unknown resource creation", v)
	}
}
