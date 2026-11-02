package application

import (
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestSRResumeWorkflowRetainsOriginalReportSource(t *testing.T) {
	input := workflow.StageInput{CommandID: "old-report", AttemptID: "old-attempt", StepID: workflow.StepReport, Prompt: "old frozen prompt", SourceCommandID: "implementation", SourceTurnID: "implementation-turn", ReportToolPolicy: "original-policy"}
	var err error
	input.Prompt, input.Budget, err = m.FreezeInput(m.InputPart{Source: "workflow.body", Owner: m.CallerInput, Text: input.Prompt})
	if err != nil {
		t.Fatal(err)
	}
	r := &runstate.Record{Allowed: true, PromptBodies: map[string]string{workflow.StepReport: "old frozen prompt"}, Materials: []runstate.Material{{MaterialView: contracts.MaterialView{SnapshotPath: "spec.md"}}, {MaterialView: contracts.MaterialView{SnapshotPath: "plan.md"}}}}
	r.Request.Workflow = contracts.Workflow{ID: "development", Version: 2}
	r.Request.Execution.Adapter = "codex-app-server"
	r.Workflow = workflow.Snapshot{StepID: workflow.StepReport, StageInputs: []workflow.StageInput{{CommandID: "implementation", StepID: workflow.StepImplement}, input}, StageResults: []workflow.StageResult{{CommandID: "implementation", StepID: workflow.StepImplement, TurnID: "implementation-turn", Succeeded: true, FinalVerified: true, Final: "verified original result"}}}
	r.Snapshot.PendingTurn = &b.TurnInput{CommandID: "old-report"}
	preservePendingStage(r)
	if r.NextStage == nil || r.NextStage.CommandID != "" || r.NextStage.Prompt != input.Prompt {
		t.Fatal("pending report lost its frozen prompt or retained a stale command")
	}
	// The same original attempt remains available; only its unsent successor
	// receives a fresh command identity.
	r.Snapshot.Session = b.SessionContext{SessionID: "original-session", AttemptID: "old-attempt"}
	r.Snapshot.PendingTurn = nil
	bindTestWorkflow(r)
	if err := resumeWorkflow(r); err != nil {
		t.Fatal(err)
	}
	queued := r.Workflow.StageInputs[len(r.Workflow.StageInputs)-1]
	if queued.CommandID == "old-report" || queued.CommandID == "" || queued.AttemptID != "old-attempt" || queued.StepID != workflow.StepReport || queued.SourceCommandID != input.SourceCommandID || queued.SourceTurnID != input.SourceTurnID || queued.ReportToolPolicy != input.ReportToolPolicy || !reflect.DeepEqual(queued.Budget, input.Budget) || queued.Prompt != input.Prompt || r.NextStage != nil {
		t.Fatal("resumed report lost its original source or native policy", queued)
	}
	if r.Snapshot.Session.SessionID != "original-session" || !reflect.DeepEqual(r.Workflow.StageInputs[1], input) || !r.Allowed || r.ResumeRequired {
		t.Fatal("business continuation changed native identity, original input or control authorization")
	}
}
