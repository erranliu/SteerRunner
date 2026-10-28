package application

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"strings"
	"testing"
)

func budgetRecord() *runstate.Record {
	r := &runstate.Record{Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "development", Version: 2}, Execution: contracts.Execution{Adapter: "codex-app-server"}, Documents: contracts.Documents{Spec: contracts.Document{Path: "spec.md"}, Plan: contracts.Document{Path: "plan.md"}}}, PromptBodies: map[string]string{workflow.StepImplement: workflow.ImplementPrompt}}
	bindTestWorkflow(r)
	return r
}
func TestBudgetCallerOwnershipAndFeedback(t *testing.T) {
	for _, override := range []bool{false, true} {
		r := budgetRecord()
		if override {
			r.Request.Workflow.PromptOverrides = map[string]string{workflow.StepImplement: workflow.ImplementPrompt}
		}
		if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, "回答\n😀"); err != nil {
			t.Fatal(err)
		}
		b := r.Snapshot.PendingTurn.Budget
		want := len("回答\n😀")
		if override {
			want += len(workflow.ImplementPrompt)
		}
		if b.CallerBytes != want || b.RunnerBytes+b.CallerBytes != len(r.Snapshot.PendingTurn.Prompt) {
			t.Fatal("wrong ownership", b)
		}
		original := r.Workflow.StageInputs[0]
		original.Budget = nil
		// Restore a frozen input with no feedback decision: old saved source can
		// only establish all of the suffix as Runner, so use its no-feedback form.
		old := budgetRecord()
		if override {
			old.Request.Workflow.PromptOverrides = r.Request.Workflow.PromptOverrides
		}
		if err := queueInput(old, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
			t.Fatal(err)
		}
		original = old.Workflow.StageInputs[0]
		text := original.Prompt
		original.Budget = nil
		if err := ensureStageBudget(old, &original); err != nil || original.Prompt != text {
			t.Fatal("legacy input rewritten", err)
		}
	}
	r := budgetRecord()
	r.PromptBodies[workflow.StepImplement] = strings.Repeat("c", 2000)
	r.Request.Workflow.PromptOverrides = map[string]string{workflow.StepImplement: r.PromptBodies[workflow.StepImplement]}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, "x"); err == nil || r.Snapshot.PendingTurn != nil || len(r.Workflow.StageInputs) != 0 {
		t.Fatal("caller overflow queued input", err)
	}
	r = budgetRecord()
	r.Request.Documents.Spec.Path = strings.Repeat("p", 1100)
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err == nil || r.Snapshot.PendingTurn != nil {
		t.Fatal("Runner path overflow queued", err)
	}
}
func TestBudgetFileGuidesIsolation(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		r := budgetRecord()
		if mask&1 != 0 {
			r.Request.ProgressFile = &contracts.ProgressFileInput{}
		}
		if mask&2 != 0 {
			r.Request.ResultFile = &contracts.ResultFileInput{}
		}
		if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
			t.Fatal(mask, err)
		}
		input := r.Workflow.StageInputs[0]
		if strings.Contains(input.Prompt, workflow.ProgressInstruction) != (mask&1 != 0) || strings.Contains(input.Prompt, workflow.ResultFileInstruction) != (mask&2 != 0) {
			t.Fatal(mask, "missing write guide")
		}
		r.Snapshot.Session.SessionID = "same"
		r.Workflow.StageResults = []workflow.StageResult{{CommandID: input.CommandID, InputVersion: input.Version, TurnID: "turn", SessionID: "same", FinalVerified: true, Final: "unstructured original"}}
		report := workflow.StageInput{StepID: workflow.StepReport, SourceCommandID: input.CommandID, SourceVersion: input.Version, SourceTurnID: "turn", SourceResultID: "result", ReportToolPolicy: "native_permissions"}
		source := workflow.Source(r.Workflow.StageResults[0])
		r.Workflow.Results = []ra.Result{{ID: "result", Source: source}}
		if err := queueInput(r, report, ""); err != nil {
			t.Fatal(mask, err)
		}
		got := r.Snapshot.PendingTurn.Prompt
		if strings.Contains(got, "STEERLINE_PROGRESS_FILE") || strings.Contains(got, "STEERLINE_RESULT_FILE") {
			t.Fatal("report got producer guide")
		}
		if r.Snapshot.PendingTurn.Budget.RunnerBytes > m.RunnerInputBytes {
			t.Fatal("report exceeds hard budget")
		}
	}
}
func TestBudgetParserOverflowRetainsReportAndLongFinal(t *testing.T) {
	e, _ := auditEngine(t)
	e.runs["run"].AcquisitionConfig = ra.FrozenConfig{Config: ra.DefaultConfig()}
	e.runs["run"].AcquisitionConfig.Config.CodexParser.Enabled = true
	finishStage(t, e, "实施及必要验证已完成。")
	final := "Audit incomplete: " + strings.Repeat("unknown evidence ", 300)
	finishStage(t, e, final)
	r := e.runs["run"]
	if parserIndex(r) >= 0 || r.Workflow.StepID != workflow.StepAuditReport || r.Snapshot.PendingTurn == nil || r.Workflow.StageResults[len(r.Workflow.StageResults)-1].Final != final {
		t.Fatal("overflow lost original final/report fallback", r.Workflow.StepID, r.Diagnostic)
	}
	if len(r.Workflow.StageInputs) != 3 {
		t.Fatal("extra acquisition/input")
	}
}

func TestBudgetAuditedFilesRemainUsable(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		e, _ := auditEngine(t, mask)
		checkWrappedBudget(t, e.runs["run"].Snapshot.PendingTurn)
		finishStage(t, e, "实施及必要验证已完成。")
		r := e.runs["run"]
		if r.Snapshot.PendingTurn == nil {
			t.Fatal(mask, "audit unavailable", r.Diagnostic)
		}
		text := r.Snapshot.PendingTurn.Prompt
		checkWrappedBudget(t, r.Snapshot.PendingTurn)
		if strings.Contains(text, workflow.ContinueProgressInstruction) != (mask&1 != 0) || strings.Contains(text, workflow.ContinueResultInstruction) != (mask&2 != 0) || strings.Contains(text, workflow.ProgressInstruction) || strings.Contains(text, workflow.ResultFileInstruction) {
			t.Fatal(mask, "original session rules lost/repeated")
		}
		finishStage(t, e, `{"outcome":"done","summary":"audited and validated; no repairs","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`)
		r = e.runs["run"]
		if r.Snapshot.PendingTurn == nil || r.Workflow.StepID != workflow.StepPublish {
			t.Fatal(mask, "publication unavailable", r.Diagnostic)
		}
		checkWrappedBudget(t, r.Snapshot.PendingTurn)
		if strings.Contains(r.Snapshot.PendingTurn.Prompt, "progress") || strings.Contains(r.Snapshot.PendingTurn.Prompt, "STEERLINE_RESULT_FILE") {
			t.Fatal(mask, "publication received write guide")
		}
	}
}
func checkWrappedBudget(t *testing.T, input *b.TurnInput) {
	t.Helper()
	parts, err := m.InputParts(input.Prompt, input.Budget)
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := m.DescribeInput(m.NativeInputParts(parts)...)
	if err != nil {
		t.Fatal(err)
	}
	wire := codex.RemoteModelPrompt(input.CommandID, input.Budget.SHA256, input.StepID, "gpt-6.1-sol", "xhigh", input.Prompt)
	if !strings.HasSuffix(wire, body) {
		t.Fatal("wire body changed")
	}
	parts = append([]m.InputPart{{Source: "native.header", Owner: m.RunnerInput, Text: strings.TrimSuffix(wire, body)}}, m.NativeInputParts(parts)...)
	_, budget, err := m.FreezeInput(parts...)
	if err != nil {
		t.Fatal(input.StepID, err)
	}
	t.Logf("audited %s model bridge Runner=%d", input.StepID, budget.RunnerBytes)
}
func TestBudgetLegacyDefinitionKeepsTextButNotRoutingChanges(t *testing.T) {
	p, _ := workflow.Lookup("development", 2)
	raw := canonical(p)
	var old workflow.Profile
	_ = json.Unmarshal(raw, &old)
	state := old.States[workflow.StepReport]
	state.Prompt = "old body"
	state.ResultInstruction = "old format"
	old.States[workflow.StepReport] = state
	if !compatibleWorkflow(p, old, "codex-app-server") {
		t.Fatal("old frozen text made workflow unsupported")
	}
	state.Next = "foreign"
	old.States[workflow.StepReport] = state
	if compatibleWorkflow(p, old, "codex-app-server") {
		t.Fatal("changed routing accepted")
	}
}
