package workflow

import (
	"encoding/json"
	"errors"
)

type Snapshot struct {
	Config               *Profile       `json:"config,omitempty"`
	ExecutionModel       *ModelSettings `json:"execution_model,omitempty"`
	AuditModel           *ModelSettings `json:"audit_model,omitempty"`
	AuditReportCommandID string         `json:"audit_report_command_id,omitempty"`
	RepairCount          uint64         `json:"repair_count,omitempty"`
	Audits               []AuditRecord  `json:"audits,omitempty"`
	StepID               string
	ReportRetry          uint64
	PRURL                string
	StageInputs          []StageInput
	StageResults         []StageResult
}

func Input(w Snapshot, command string) *StageInput {
	for i := range w.StageInputs {
		if w.StageInputs[i].CommandID == command {
			return &w.StageInputs[i]
		}
	}
	return nil
}
func Result(w Snapshot, command string) *StageResult {
	for i := range w.StageResults {
		if w.StageResults[i].CommandID == command {
			return &w.StageResults[i]
		}
	}
	return nil
}
func MaterialFacts(w Snapshot) json.RawMessage {
	for _, input := range w.StageInputs {
		if len(input.MaterialFacts) > 0 {
			return append(json.RawMessage(nil), input.MaterialFacts...)
		}
	}
	return nil
}
func firstNonempty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func clip(s string, limit int) string {
	if len(s) > limit {
		return s[:limit]
	}
	return s
}

func RenderInput(w Snapshot, input StageInput) (string, error) {
	return RenderInputWithBody(w, input, "")
}
func RenderInputWithBody(w Snapshot, input StageInput, body string) (string, error) {
	facts := map[string]any{}
	switch input.StepID {
	case StepImplement:
		materials := input.MaterialFacts
		if len(materials) == 0 {
			materials = MaterialFacts(w)
		}
		facts["materials"] = materials
	case StepAuditSpec, StepAuditCode:
		if !AuditSource(w, input) {
			return "", errors.New("audit lacks its current successful implementation report")
		}
		facts["materials"] = input.MaterialFacts
		facts["implementation_report_command_id"] = input.SourceCommandID
		facts["implementation_report_turn_id"] = input.SourceTurnID
		facts["implementation_report"] = Result(w, input.SourceCommandID).Final
	case StepReport, StepPublishReport:
		source := Result(w, input.SourceCommandID)
		original := Input(w, input.SourceCommandID)
		sourceStep := StepImplement
		if input.StepID == StepPublishReport {
			sourceStep = StepPublish
		}
		if source == nil || original == nil || original.StepID != sourceStep || source.StepID != sourceStep || source.TurnID != input.SourceTurnID {
			return "", errors.New("report lacks its original execution facts")
		}
		// A retry's preceding reply is the failed report, not its source execution.
		for _, prior := range w.StageInputs {
			if prior.StepID == input.StepID && prior.SourceCommandID == input.SourceCommandID {
				if source.FinalVerified && source.Final != "" {
					facts["original_result"] = clip(source.Final, 16*1024)
					if len(source.Final) > 16*1024 {
						facts["evidence_gap"] = "Original result is truncated."
					}
				}
				break
			}
		}
		if !source.Succeeded {
			facts["execution_failed"] = true
		}
		if source.Failure != "" {
			facts["failure"] = source.Failure
		}
		if !source.FinalVerified || source.Final == "" {
			facts["evidence_gap"] = firstNonempty(source.FinalDiagnostic, "Original final reply is unavailable or unverified.")
		}

	case StepPublish:
	default:
		return "", errors.New("no automatic prompt for this step")
	}
	prompt, err := RenderPromptWithBody(input.StepID, facts, body)
	if input.StepID == StepImplement && len(input.RepairFindings) > 0 {
		raw, marshalErr := json.Marshal(input.RepairFindings)
		if marshalErr != nil {
			return "", marshalErr
		}
		prompt += "\nRepair the following verified audit findings in the original work, run necessary checks, and do not publish:\n" + string(raw)
	}
	if input.StepID == StepPublish && w.Config != nil {
		prompt += "\nPublish the work that passed the current audits. Do not make further implementation changes during publication."
	}
	if w.PRURL != "" && (input.StepID == StepImplement || input.StepID == StepPublish) {
		prompt += "\nExisting PR: " + w.PRURL + "\nContinue on the original branch and update this same PR; do not create another PR."
	}
	return prompt, err
}

// queueStage changes the one business authority and freezes the next input
// intent inside the same worker commit as the accepted source result.
