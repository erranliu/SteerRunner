package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func snapshotExecutionFixture(t *testing.T) (*Engine, *atomic.Int32) {
	t.Helper()
	e := singleExecutionFixture(t)
	r := e.runs["run"].Clone()
	r.Request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "id:original-remote"}}
	r.Snapshot.Ref.Protocol, r.Snapshot.Ref.Agent.Name = b.ProtocolOrcaAgentSessionV1, "codex"
	r.Snapshot.Ref.Backend = b.BackendOrca
	r.Snapshot.Ref.AttemptID = r.Snapshot.Session.AttemptID
	r.Snapshot.Ref.Target = b.ExecutionTarget{ID: "orca-fixture-target", ExecutionHost: "id:original-remote", ExternalRuntime: "id:original-remote"}
	r.ResultFileContract = contracts.ResultFileSnapshotContract
	r.StopEvidenceScope = orca.AgentSessionStopScope
	r.ResultFile.View.Contract = contracts.ResultFileSnapshotContract
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	var reads atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		if !result || prepare {
			t.Error("business-end collection reinitialized a file")
		}
		return &runtimeFileChannel{state: fileState(r, true), read: func(context.Context) ([]byte, error) {
			reads.Add(1)
			return []byte("original business-end bytes"), nil
		}, close: func() error { return nil }}, nil
	}
	return e, &reads
}

func TestResultSnapshotCollectsBusinessResultWithUnknownStop(t *testing.T) {
	for _, outcome := range []string{"succeeded", "failed", "cancelled"} {
		t.Run(outcome, func(t *testing.T) {
			e, reads := snapshotExecutionFixture(t)
			if !e.finish("run", contentEvidence(e, "complete result", outcome, true)) {
				t.Fatal("original result not committed")
			}
			r := e.runs["run"]
			if r.ContentResult == nil || r.ResultFileEnd == nil || !strings.HasPrefix(r.ResultFileEnd.ID, "result:") || r.StopStatus == "stopped" {
				t.Fatal("end marker missing or stop invented", r.View(), r.ResultFileEnd)
			}
			end := *r.ResultFileEnd
			e.sealResult(context.Background(), "run")
			v := resultView(e)
			if v.Contract != contracts.ResultFileSnapshotContract || v.Version != 1 || string(snapshotBytes(t, v)) != "original business-end bytes" || reads.Load() != 1 || e.runs["run"].Revision != r.Revision {
				t.Fatal("business-end snapshot missing or changed Run", v, reads.Load())
			}
			// A later close, stale business clone, reload or query cannot refresh it.
			next := r.Clone()
			next.StopStatus = "unknown"
			next.ResultFileEnd = &runstate.ResultFileEnd{ID: "stop:later", Deadline: time.Now().Add(time.Hour)}
			if err := e.commitLocked(next, nil); err != nil {
				t.Fatal(err)
			}
			rows, err := e.store.Runs()
			if err != nil {
				t.Fatal(err)
			}
			e.runs["run"] = rows[0]
			e.workers["run"].resultCollectionStarted = false
			e.sealResult(context.Background(), "run")
			if reads.Load() != 1 || !reflect.DeepEqual(*rows[0].ResultFileEnd, end) || !reflect.DeepEqual(resultView(e), v) {
				t.Fatal("original snapshot refreshed")
			}
		})
	}
}

type snapshotUnknownStop struct {
	operationCommitPort
	result *b.TurnEvidence
}

func (p *snapshotUnknownStop) StopLifecycle(_ context.Context, request b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	p.stops++
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationUnknown, TurnEvidence: p.result}, errors.New("original session exit unverified")
}

func TestResultSnapshotRetainsNativeStopResultWithoutStageAuthority(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "foreign-turn"}[foreign], func(t *testing.T) {
			e, _ := snapshotExecutionFixture(t)
			r := e.runs["run"].Clone()
			r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_agent_session", RuntimeID: "original-runtime", WorktreeID: "original-worktree", ExecutionHost: "id:original-remote"}
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			value := contentEvidence(e, "complete native cancellation", "cancelled", true)
			value.FinalNativeSessionID, value.FinalNativeTurnID = value.SessionID, value.TurnID
			if foreign {
				value.FinalTurnID = "another-turn"
			}
			if reply := contentControl(e, "stop", "original-stop", contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision}); !reply.OK {
				t.Fatal(reply.Error)
			}
			w := e.workers["run"]
			p := &snapshotUnknownStop{result: &value}
			w.port = p
			e.stopRun("run", w, p)
			r = e.runs["run"]
			if r.Allowed || r.StopStatus != "unknown" || len(r.Workflow.StageResults) != 0 || len(r.Workflow.StageInputs) != 1 {
				t.Fatal("stop result advanced workflow or resource settlement", r.View())
			}
			if foreign {
				if r.Result != nil || r.View().Final != nil {
					t.Fatal("foreign stop result retained")
				}
				return
			}
			if r.Result == nil || r.Result.NativeOutcome != "cancelled" || r.View().Final == nil || !r.View().Final.Verified || r.View().Final.Text != "complete native cancellation" {
				t.Fatal("complete original cancellation lost", r.View())
			}
		})
	}
}

func TestResultSnapshotStopUnknownClosesCollectionWithoutChangingResult(t *testing.T) {
	e, reads := snapshotExecutionFixture(t)
	r := e.runs["run"].Clone()
	// Force an actual original-resource stop rather than the empty-ref shortcut.
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_agent_session", RuntimeID: "original-runtime", WorktreeID: "original-worktree", ExecutionHost: "id:original-remote"}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if reply := contentControl(e, "stop", "original-stop", contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision}); !reply.OK {
		t.Fatal(reply.Error)
	}
	w := e.workers["run"]
	p := &snapshotUnknownStop{}
	w.port = p
	e.stopRun("run", w, p)
	r = e.runs["run"]
	if r.StopStatus != "unknown" || r.ResultFileEnd == nil || !strings.HasPrefix(r.ResultFileEnd.ID, "stop:") || p.stops != 1 || reads.Load() != 1 || resultView(e).Version != 1 || r.Allowed {
		t.Fatal("unknown stop failed its independent snapshot contract", r.View(), p.stops, reads.Load(), resultView(e))
	}
	end := *r.ResultFileEnd
	// Existing unresolved-operation rules retain the complete original native
	// result before workflow acquisition; unknown stop cannot authorize a stage.
	_ = e.finish("run", contentEvidence(e, "late complete result", "succeeded", true))
	e.sealResult(context.Background(), "run")
	if reads.Load() != 1 || !reflect.DeepEqual(*e.runs["run"].ResultFileEnd, end) || e.runs["run"].Result == nil || e.runs["run"].Result.LastMessage != "late complete result" || e.runs["run"].Allowed {
		t.Fatal("late result replaced the first end or disappeared")
	}
}

func TestResultSnapshotDetachDoesNotStartAndRecoveryKeepsOriginalWindow(t *testing.T) {
	e, reads := snapshotExecutionFixture(t)
	r := e.runs["run"].Clone()
	r.StopStatus = "unknown"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.sealResult(context.Background(), "run")
	if reads.Load() != 0 || resultView(e).CollectionStatus != "pending" {
		t.Fatal("mere detach started snapshot")
	}
	markResultSnapshotEnd(r, "stop:original")
	r.ResultFileEnd.Deadline = time.Now().Add(-time.Second)
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := e.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	e.runs["run"] = rows[0]
	e.sealResult(context.Background(), "run")
	if v := resultView(e); reads.Load() != 0 || v.CollectionStatus != "closed" || v.Version != 0 || v.ContentBase64 != nil || v.Diagnostic != "read_failed" {
		t.Fatal("restart opened a fresh snapshot window", v, reads.Load())
	}
}

type confirmedStructuredDevelopmentStop struct {
	operationCommitPort
}

func (p *confirmedStructuredDevelopmentStop) StopLifecycle(_ context.Context, request b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}}, nil
}

func TestStructuredDevelopmentConfirmedStopRevokesResume(t *testing.T) {
	e, _ := snapshotExecutionFixture(t)
	r := e.runs["run"].Clone()
	r.Request.Workflow = contracts.Workflow{ID: "development", Version: 2}
	r.ResultFileContract = ""
	r.Request.ResultFile, r.ResultFile = nil, nil
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.AgentSessionCloseProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.AgentSessionProfileKey, ProjectID: r.Snapshot.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: "fixture-config"}, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Profile = []b.ImplementationDataEnvelope{profile}
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_agent_session", RuntimeID: "original-runtime", WorktreeID: "original-worktree", ExecutionHost: "id:original-remote"}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if reply := contentControl(e, "stop", "original-stop", contracts.RevisionInput{ExpectedRevision: r.Revision}); !reply.OK {
		t.Fatal(reply.Error)
	}
	w := e.workers["run"]
	p := &confirmedStructuredDevelopmentStop{}
	w.port = p
	e.stopRun("run", w, p)
	stopped := e.runs["run"]
	if stopped.StopStatus != "stopped" || stopped.Allowed || stopped.ResumeRequired || stopped.View().Controls.CanResume {
		t.Fatal(stopped.View())
	}
	for _, command := range []string{"resume", "stop"} {
		if reply := contentControl(e, command, "after-stopped-"+command, contracts.RevisionInput{ExpectedRevision: stopped.Revision}); reply.OK || reply.Error.Code != "invalid_state" {
			t.Fatal(command, reply)
		}
	}
	if e.runs["run"].Revision != stopped.Revision {
		t.Fatal("rejected command changed the durable stop")
	}
}
