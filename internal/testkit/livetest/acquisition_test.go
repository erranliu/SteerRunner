package livetest

import (
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func smokeAcquisitionFixture(report bool) (workflow.Snapshot, []contracts.AcquisitionView) {
	w := workflow.Snapshot{}
	views := []contracts.AcquisitionView{}
	for n, step := range []string{workflow.StepImplement, workflow.StepPublish} {
		command, final, contract := "implement", "实施及必要验证已完成。", "implementation"
		if n == 1 {
			command, final, contract = "publish", "https://github.com/erranliu/SteerlineTest/pull/1", "publication"
		}
		original := workflow.StageResult{SessionID: "session", CommandID: command, TurnID: command, AttemptID: "attempt", InputVersion: uint64(n + 1), StepID: step, Final: final, Succeeded: true, FinalVerified: true}
		if report && n == 0 {
			original.Final = "實施及必要驗證已完成."
		}
		input := workflow.StageInput{CommandID: command, StepID: step, Version: original.InputVersion, SuccessOutput: "实施及必要验证已完成。", Prompt: "implement instructions"}
		w.StageInputs = append(w.StageInputs, input)
		w.StageResults = append(w.StageResults, original)
		source := workflow.Source(original)
		a := contracts.AcquisitionView{ID: command + "-result", Contract: contract, Source: source, Method: "program", Status: "satisfied", Attempts: []ra.Attempt{{Method: "program", CommandID: command, Source: source, Status: "satisfied"}}, Fields: []ra.FieldSource{{Field: "outcome", Source: source, Method: "program"}}}
		if report && n == 0 {
			r := workflow.StageInput{CommandID: "report", StepID: workflow.StepReport, Version: 3, SourceCommandID: command, SourceTurnID: original.TurnID, SourceVersion: original.InputVersion, SourceResultID: a.ID}
			reported := workflow.StageResult{CommandID: "report", StepID: workflow.StepReport, TurnID: "report", InputVersion: 3, Succeeded: true, FinalVerified: true, Final: `{"outcome":"done","summary":"actual result"}`}
			w.StageInputs = append(w.StageInputs, r)
			w.StageResults = append(w.StageResults, reported)
			a.Method = "agent_report"
			a.Attempts[0].Status = "insufficient"
			a.Attempts = append(a.Attempts, ra.Attempt{Method: "agent_report", CommandID: "report", Source: source, Status: "satisfied"})
			a.Fields[0].Source, a.Fields[0].Method = workflow.Source(reported), "agent_report"
		}
		if contract == "implementation" {
			w.ImplementationResultID = a.ID
		} else {
			w.PublicationResultID = a.ID
		}
		views = append(views, a)
	}
	return w, views
}

func TestDevelopmentAcquisitionNativeAssertions(t *testing.T) {
	for _, tt := range []struct {
		name    string
		report  bool
		change  func(*workflow.Snapshot, []contracts.AcquisitionView)
		wantErr bool
	}{
		{name: "exact marker no extra call"},
		{name: "near marker single report", report: true},
		{name: "complete machine report no extra call", change: func(w *workflow.Snapshot, v []contracts.AcquisitionView) {
			w.StageResults[0].Final = `{"outcome":"done","summary":"actual result"}`
			v[0].Source = workflow.Source(w.StageResults[0])
			v[0].Attempts[0].Source = v[0].Source
			v[0].Fields[0].Source = v[0].Source
		}},
		{name: "exact marker unnecessary report", report: true, wantErr: true, change: func(w *workflow.Snapshot, v []contracts.AcquisitionView) {
			w.StageResults[0].Final = "实施及必要验证已完成。"
			v[0].Source = workflow.Source(w.StageResults[0])
			for i := range v[0].Attempts {
				v[0].Attempts[i].Source = v[0].Source
			}
		}},
		{name: "parser forbidden", report: true, wantErr: true, change: func(_ *workflow.Snapshot, v []contracts.AcquisitionView) { v[0].Attempts[1].Method = "llm_parser" }},
		{name: "second report forbidden", report: true, wantErr: true, change: func(w *workflow.Snapshot, _ []contracts.AcquisitionView) {
			w.StageInputs = append(w.StageInputs, w.StageInputs[1])
		}},
		{name: "wrong report original turn", report: true, wantErr: true, change: func(w *workflow.Snapshot, _ []contracts.AcquisitionView) { w.StageInputs[1].SourceTurnID = "different" }},
		{name: "wrong report field source", report: true, wantErr: true, change: func(_ *workflow.Snapshot, v []contracts.AcquisitionView) { v[0].Fields[0].Source = v[0].Source }},
		{name: "report redid work", report: true, wantErr: true, change: func(w *workflow.Snapshot, _ []contracts.AcquisitionView) { w.StageResults[1].UnexpectedWork = true }},
		{name: "native failure cannot be overwritten", report: true, wantErr: true, change: func(w *workflow.Snapshot, _ []contracts.AcquisitionView) { w.StageResults[0].Succeeded = false }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w, views := smokeAcquisitionFixture(tt.report)
			if tt.change != nil {
				tt.change(&w, views)
			}
			if err := CheckDevelopmentAcquisition(w, views); (err != nil) != tt.wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestDevelopmentAcquisitionRetainsQuestionHistory(t *testing.T) {
	w, views := smokeAcquisitionFixture(false)
	old := workflow.StageResult{SessionID: "session", CommandID: "question", TurnID: "question", AttemptID: "attempt", InputVersion: 7, StepID: workflow.StepImplement, Succeeded: true, FinalVerified: true, Final: `{"outcome":"needs_input","summary":"authorization required","question":"May I continue?"}`}
	w.StageInputs = append(w.StageInputs, workflow.StageInput{CommandID: old.CommandID, StepID: old.StepID, Version: old.InputVersion})
	w.StageResults = append(w.StageResults, old)
	views = append(views, contracts.AcquisitionView{ID: "old-question", Contract: "implementation", Source: workflow.Source(old), Method: "program", Status: "satisfied"})
	if err := CheckDevelopmentAcquisition(w, views); err != nil {
		t.Fatal("historical question invalidated current completion", err)
	}
	w.ImplementationResultID = "old-question"
	if CheckDevelopmentAcquisition(w, views) == nil {
		t.Fatal("historical incomplete source became current completion")
	}
	w.ImplementationResultID = "missing"
	if CheckDevelopmentAcquisition(w, views) == nil {
		t.Fatal("missing current source accepted")
	}
}
