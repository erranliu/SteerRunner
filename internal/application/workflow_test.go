package application

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"reflect"
	"strings"
	"testing"
)

type auditPort struct{ reportPolicyPort }

func (auditPort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, ResourceAvailable: true}, nil
}

type auditRestorePort struct{ auditPort }

func (auditRestorePort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, SessionRebindable: true, ProcessAbsent: true, WritersAbsent: true}, nil
}

func (auditRestorePort) BindRecovery(_ context.Context, _, next b.LifecycleSnapshot) (b.ImplementationDataEnvelope, error) {
	return b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: 1, Purpose: b.ImplementationResourcePurpose, ImplementationKey: "test", ProjectID: "project", ExecutionID: "execution", AttemptID: next.Session.AttemptID, Generation: next.ResourceGeneration, ConfigDigest: "config"}, []byte(`{}`))
}

func TestSRFrozenInputRestoration(t *testing.T) {
	for _, scenario := range []string{"audit_answer", "business_feedback", "automatic_repair", "report_retry"} {
		t.Run(scenario, func(t *testing.T) {
			e, w := auditEngine(t)
			finishStage(t, e, "implemented")
			if scenario == "report_retry" {
				finishStage(t, e, "malformed report")
			} else {
				finishStage(t, e, `{"outcome":"done","summary":"implemented"}`)
				switch scenario {
				case "audit_answer":
					finishStage(t, e, `{"outcome":"needs_input","summary":"scope","question":"which requirement?"}`)
					if !e.feedbackValue("run", w, w.port, &runstate.Answer{CommandID: "answer", Epoch: "epoch", Text: "Keep the compatibility branch for legacy clients."}, true) {
						t.Fatal("answer was not queued")
					}
				case "business_feedback":
					finishStage(t, e, "malformed audit")
					if !e.feedbackValue("run", w, w.port, &runstate.Answer{CommandID: "feedback", Epoch: "epoch", Text: "Fix the compatibility branch for legacy clients."}, false) {
						t.Fatal("feedback was not queued")
					}
				case "automatic_repair":
					finishStage(t, e, `{"outcome":"done","summary":"repair","verdict":"changes_requested","findings":[{"severity":"P1","message":"Restore compatibility for legacy clients."}]}`)
				}
			}
			before := e.runs["run"].Clone()
			// Reload the committed input before any native start was dispatched.
			rows, err := e.store.Runs()
			if err != nil || len(rows) != 1 {
				t.Fatal("cannot reload queued input", err)
			}
			r := rows[0]
			r.Allowed, r.ResumeRequired = false, true
			r.Status, r.PendingResume = "reconciling", "resume"
			e.runs["run"] = r
			p := auditRestorePort{}
			w.port = p
			if !e.resumeRun("run", w, p) {
				t.Fatal("resume failed", e.runs["run"].Diagnostic)
			}
			after := e.runs["run"]
			input := *after.Snapshot.PendingTurn
			original := *before.Snapshot.PendingTurn
			if input.CommandID == original.CommandID || input.InputVersion <= original.InputVersion || after.Snapshot.Session.AttemptID == before.Snapshot.Session.AttemptID || after.Snapshot.Session.SessionID != before.Snapshot.Session.SessionID {
				t.Fatal("restoration did not rebind to the same session")
			}
			input.CommandID, input.InputVersion = original.CommandID, original.InputVersion
			if !reflect.DeepEqual(input, original) {
				t.Fatalf("restoration changed frozen native input:\nbefore: %+v\nafter: %+v", original, input)
			}
			prior := workflow.Input(before.Workflow, original.CommandID)
			current := workflow.Input(after.Workflow, after.Snapshot.PendingTurn.CommandID)
			if current.SourceCommandID != prior.SourceCommandID || current.SourceTurnID != prior.SourceTurnID || current.FeedbackDecisionID != prior.FeedbackDecisionID || !reflect.DeepEqual(current.RepairFindings, prior.RepairFindings) || after.Workflow.RepairCount != before.Workflow.RepairCount || after.Workflow.ReportRetry != before.Workflow.ReportRetry || after.NextStage != nil {
				t.Fatal("restoration lost source, feedback, findings or retry budgets")
			}
		})
	}
}

func auditEngine(t *testing.T) (*Engine, *worker) {
	t.Helper()
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	w := &worker{port: auditPort{}, wake: make(chan struct{}, 1)}
	e := &Engine{ctx: context.Background(), epoch: "epoch", store: s, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}}
	p, _ := workflow.Lookup(workflow.AuditedDevelopment, 1)
	r := &runstate.Record{ID: "run", Allowed: true, CommandStates: map[string]string{}, Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: p.ID, Version: p.Version}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}}}, Materials: []runstate.Material{{MaterialView: contracts.MaterialView{Path: "spec", SnapshotPath: "spec"}}, {MaterialView: contracts.MaterialView{Path: "plan", SnapshotPath: "plan"}}}}
	r.Workflow.Config = &p
	r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: "executor", ReasoningEffort: "medium"}
	r.Workflow.AuditModel = &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}
	r.Snapshot.Session = b.SessionContext{SessionID: "session", AttemptID: "attempt"}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	return e, w
}

func finishStage(t *testing.T, e *Engine, body string) {
	t.Helper()
	r := e.runs["run"].Clone()
	input := r.Snapshot.PendingTurn
	if input == nil {
		t.Fatal("no pending turn")
	}
	r.Snapshot.Session.CurrentCommandID = input.CommandID
	r.Snapshot.Session.CurrentTurnID = "turn-" + input.CommandID
	e.runs["run"] = r
	e.finish("run", b.TurnEvidence{CommandID: input.CommandID, SessionID: "session", TurnID: r.Snapshot.Session.CurrentTurnID, Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "item", LastMessage: body})
}

func TestSRAuditLifecycle(t *testing.T) {
	e, w := auditEngine(t)
	finishStage(t, e, "implemented")
	finishStage(t, e, `{"outcome":"done","summary":"implementation report"}`)
	r := e.runs["run"]
	if r.Workflow.StepID != workflow.StepAuditSpec || r.Snapshot.PendingTurn.Model != "reviewer" {
		t.Fatal("audit not selected")
	}
	reportSource := r.Workflow.AuditReportCommandID
	finishStage(t, e, `{"outcome":"needs_input","summary":"clarify scope","question":"is this required?"}`)
	e.feedbackValue("run", w, w.port, &runstate.Answer{CommandID: "answer", Epoch: "epoch", Text: "yes"}, true)
	r = e.runs["run"]
	if r.Workflow.StepID != workflow.StepAuditSpec || r.Snapshot.PendingTurn.Model != "reviewer" || !strings.Contains(r.Snapshot.PendingTurn.Prompt, "yes") || r.Workflow.AuditReportCommandID != reportSource {
		t.Fatal("answer left original audit", r.Diagnostic)
	}
	for count := uint64(1); count <= 2; count++ {
		finishStage(t, e, `{"outcome":"done","summary":"fix","verdict":"changes_requested","findings":[{"severity":"P1","message":"defect"}]}`)
		r = e.runs["run"]
		if r.Workflow.RepairCount != count || r.Workflow.StepID != workflow.StepImplement || r.Snapshot.PendingTurn.Model != "executor" || !strings.Contains(r.Snapshot.PendingTurn.Prompt, "defect") || len(r.Workflow.Audits) != 0 {
			t.Fatal("repair did not reuse implementation", r.Diagnostic)
		}
		finishStage(t, e, "fixed")
		finishStage(t, e, `{"outcome":"done","summary":"fixed report"}`)
	}
	finishStage(t, e, `{"outcome":"done","summary":"still fails","verdict":"changes_requested","findings":[{"severity":"P1","message":"remaining defect"}]}`)
	r = e.runs["run"]
	if r.Status != "blocked" || r.Workflow.RepairCount != 2 || r.Snapshot.PendingTurn != nil || r.Diagnostic != "audit_repair_limit" {
		t.Fatal("budget not bounded", r.Diagnostic)
	}
	// Explicit feedback authorizes an implementation, but not more automatic repairs.
	e.feedbackValue("run", w, w.port, &runstate.Answer{CommandID: "feedback", Epoch: "epoch", Text: "fix manually"}, false)
	if e.runs["run"].Workflow.RepairCount != 2 || e.runs["run"].Workflow.StepID != workflow.StepImplement {
		t.Fatal("feedback reset budget")
	}
	finishStage(t, e, "fixed")
	finishStage(t, e, `{"outcome":"done","summary":"fixed report"}`)
	pass := `{"outcome":"done","summary":"checked","verdict":"pass","findings":[]}`
	finishStage(t, e, pass)
	// Stop's committed revocation must park the following publish without sending it.
	r = e.runs["run"].Clone()
	r.Allowed = false
	r.ResumeRequired = true
	e.runs["run"] = r
	finishStage(t, e, pass)
	r = e.runs["run"]
	if r.Snapshot.PendingTurn != nil || r.NextStage == nil || r.NextStage.StepID != workflow.StepPublish || !workflow.AuditsPassed(r.Workflow) {
		t.Fatal("revocation bypassed or audits lost")
	}
	loaded, err := e.store.Runs()
	if err != nil || len(loaded) != 1 || loaded[0].Workflow.RepairCount != 2 || loaded[0].NextStage == nil {
		t.Fatal("durable state lost", err)
	}
}

func TestSRWorkflowLegacyRecord(t *testing.T) {
	raw := []byte(`{"id":"legacy","request":{"workflow":{"id":"development","version":1}},"workflow":{"StepID":"implement.execute"},"snapshot":{"PendingTurn":{"CommandID":"old-command","Prompt":"old frozen prompt","Purpose":"business"}}}`)
	var r runstate.Record
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r.Snapshot.PendingTurn)
	if err := validateWorkflow(&r); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r.Revision = 1
	if err := s.Commit(&r, 0, nil, "legacy"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Runs()
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(rows[0].Snapshot.PendingTurn)
	if !bytes.Equal(before, after) || rows[0].View().Audit != nil || rows[0].View().Execution.RequestedModel != "" {
		t.Fatal("legacy input was filled from defaults")
	}
	old := &reliable.SavedIntent{Request: before, PayloadDigest: reliable.Digest(before)}
	if err := reliable.ValidateSavedIntent(old, b.LifecycleStartTurn, b.LifecycleInput{Turn: rows[0].Snapshot.PendingTurn}); err != nil {
		t.Fatal("old original intent changed", err)
	}
	r.Request.Workflow.ID = workflow.AuditedDevelopment
	if validateWorkflow(&r) == nil {
		t.Fatal("missing audited configuration silently became ordinary")
	}
}
