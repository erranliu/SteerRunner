package livetest

import (
	"fmt"

	"github.com/erranliu/SteerRunner/internal/contracts"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

// CheckDevelopmentAcquisition checks the actual frozen original and any single
// report fallback. An agent's failure to repeat an exact marker is not itself a
// product failure; unnecessary calls and a changed source remain failures.
func CheckDevelopmentAcquisition(w workflow.Snapshot, views []contracts.AcquisitionView) error {
	if len(views) != 2 {
		return fmt.Errorf("expected implementation and publication results")
	}
	seen := map[string]bool{}
	for _, a := range views {
		step, reportStep := workflow.StepImplement, workflow.StepReport
		if a.Contract == "publication" {
			step, reportStep = workflow.StepPublish, workflow.StepPublishReport
		} else if a.Contract != "implementation" {
			return fmt.Errorf("unexpected contract %s", a.Contract)
		}
		if a.ID == "" || seen[a.Contract] || a.Status != "satisfied" || len(a.Missing) != 0 || len(a.Conflicts) != 0 {
			return fmt.Errorf("duplicate or incomplete %s", a.Contract)
		}
		seen[a.Contract] = true
		input, original := workflow.Input(w, a.Source.CommandID), workflow.Result(w, a.Source.CommandID)
		if input == nil || original == nil || input.StepID != step || original.StepID != step ||
			input.Version != original.InputVersion || !original.Succeeded || !original.FinalVerified || original.Failure != "" || workflow.Source(*original) != a.Source {
			return fmt.Errorf("%s original source mismatch", a.Contract)
		}
		candidate, decodeErr := ra.DecodeReport(original.Final, a.Contract)
		programReady := decodeErr == nil && candidate.Outcome == "done"
		if decodeErr != nil && ra.RecognizeSuccessOutput(original.Final, input.SuccessOutput, input.Prompt) {
			programReady = true
		}
		if a.Contract == "publication" {
			// A unique clue can enter provider verification without a marker.
			programReady = len(ra.PublicationClues(original.Final, original.PRCandidates)) == 1
		}
		var reports []workflow.StageInput
		for _, in := range w.StageInputs {
			if in.SourceCommandID == original.CommandID && in.StepID == reportStep {
				reports = append(reports, in)
			}
		}
		fieldSource := a.Source
		if programReady {
			if a.Method != "program" || len(reports) != 0 || len(a.Attempts) != 1 {
				return fmt.Errorf("%s sufficient original used an unnecessary model call", a.Contract)
			}
		} else {
			if a.Method != "agent_report" || len(reports) != 1 || len(a.Attempts) != 2 {
				return fmt.Errorf("%s requires exactly one report fallback", a.Contract)
			}
			r := reports[0]
			result := workflow.Result(w, r.CommandID)
			if r.SourceTurnID != a.Source.TurnID || r.SourceVersion != a.Source.InputVersion || r.SourceResultID != a.ID ||
				result == nil || result.StepID != reportStep || result.InputVersion != r.Version || !result.Succeeded || !result.FinalVerified || result.Failure != "" || result.UnexpectedWork {
				return fmt.Errorf("%s report source or native evidence mismatch", a.Contract)
			}
			value, err := ra.DecodeReport(result.Final, a.Contract)
			if err != nil || value.Outcome != "done" {
				return fmt.Errorf("%s report did not establish completion", a.Contract)
			}
			fieldSource = workflow.Source(*result)
		}
		for n, attempt := range a.Attempts {
			method, command, status := "program", original.CommandID, "satisfied"
			if !programReady {
				status = "insufficient"
				if n == 1 {
					method, command, status = "agent_report", reports[0].CommandID, "satisfied"
				}
			}
			if attempt.Method != method || attempt.CommandID != command || attempt.Source != a.Source || attempt.Status != status || attempt.OperationID != "" {
				return fmt.Errorf("%s unexpected acquisition attempt", a.Contract)
			}
		}
		if len(a.Fields) == 0 {
			return fmt.Errorf("%s missing field provenance", a.Contract)
		}
		for _, f := range a.Fields {
			// The provider may independently verify the original PR clue.
			if (f.Source != fieldSource || f.Method != a.Method) && !(a.Contract == "publication" && f.Field == "outputs.pr_url" && f.Method == "program" && f.Source == a.Source) {
				return fmt.Errorf("%s field source mismatch", a.Contract)
			}
		}
	}
	return nil
}
