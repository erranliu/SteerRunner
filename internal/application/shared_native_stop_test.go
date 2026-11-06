package application

import (
	"context"
	"database/sql"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Historical business admission and stop authority remain Engine contracts.
// This fixture supplies saved resources and a port stub, not a legacy executor.
type sharedNativeWire struct{ closes, sends int }
type historicalPolicyStopPort struct {
	operationCommitPort
	wire *sharedNativeWire
}

func (p *historicalPolicyStopPort) StopLifecycle(ctx context.Context, r b.LifecycleRequest, h b.StopHandle) (b.LifecycleReceipt, error) {
	p.wire.closes++
	return p.operationCommitPort.StopLifecycle(ctx, r, h)
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
	host := &sharedNativeWire{}
	p := b.ProcessIdentity{Kind: "orca_terminal", RuntimeID: "candidate-runtime", Terminal: "candidate-terminal", Incarnation: "candidate-incarnation", WorktreeID: "candidate-worktree", ExecutionHost: profile.Target().ExecutionHost}
	r = e.runs[r.ID].Clone()
	r.Snapshot.Ref.External = p
	r.Snapshot.Workspace = b.WorkspaceIntent{Path: filepath.Join(f.source, "candidate-worktree"), Branch: "candidate-branch", BaseCommit: r.Snapshot.WorkspacePlan.BaseCommit}
	r.Snapshot.Ref.WorkspacePath, r.Snapshot.Ref.WorkspaceBranch = r.Snapshot.Workspace.Path, r.Snapshot.Workspace.Branch
	r.Snapshot.Session.SessionID, r.Snapshot.Session.Generation = orca.SessionID(p), p.Incarnation
	state, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: orca.StateKey, CodecVersion: orca.NativeStateVersion, Purpose: b.ImplementationResourcePurpose, ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, ConfigDigest: r.Snapshot.ConfigurationDigest}, canonical(orca.State{Process: p}))
	if err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Implementation = []b.ImplementationDataEnvelope{state}
	if err = e.commitLocked(r, nil, "saved-resource-fixture"); err != nil {
		t.Fatal(err)
	}
	w.port = &historicalPolicyStopPort{wire: host}
	return e, w, host, nil, r.ID
}
