package workflow

import (
	"encoding/json"
	"errors"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
)

type Snapshot struct {
	ProjectPolicies        *ProjectPolicies `json:"project_policies,omitempty"`
	PublicationResultID    string           `json:"publication_result_id,omitempty"`
	Config                 *Profile         `json:"config"`
	ExecutionModel         *ModelSettings   `json:"execution_model,omitempty"`
	AuditModel             *ModelSettings   `json:"audit_model,omitempty"`
	ImplementationResultID string           `json:"implementation_result_id,omitempty"`
	AuditResultID          string           `json:"audit_result_id,omitempty"`
	Results                []ra.Result      `json:"results,omitempty"`
	Audits                 []AuditRecord    `json:"audits,omitempty"`
	StepID                 string
	PRURL                  string
	StageInputs            []StageInput
	StageResults           []StageResult
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
func RenderInput(w Snapshot, input StageInput) (string, error) {
	return RenderInputWithBody(w, input, "")
}
func RenderInputWithBody(w Snapshot, input StageInput, body string) (string, error) {
	parts, err := InputParts(w, input, body, body != "")
	if err != nil {
		return "", err
	}
	text, _, err := m.FreezeInput(parts...)
	return text, err
}
func InputParts(w Snapshot, input StageInput, body string, caller bool) ([]m.InputPart, error) {
	state, ok := w.State(input.StepID)
	if !ok || state.Prompt == "" {
		return nil, errors.New("no prompt in frozen state definition")
	}
	facts := map[string]any{}
	if state.Kind == "execute" && state.Contract == "implementation" {
		facts["materials"] = input.MaterialFacts
	}
	if state.Kind == "report" {
		source, original := Result(w, input.SourceCommandID), Input(w, input.SourceCommandID)
		if source == nil || original == nil || source.TurnID != input.SourceTurnID || source.InputVersion != input.SourceVersion {
			return nil, errors.New("report lacks exact original business source")
		}
		facts["source"] = source.CommandID
		if source.Failure != "" {
			facts["failure"] = source.Failure
		}
	} else if state.Contract == "audit" {
		if !AuditSource(w, input) {
			return nil, errors.New("audit lacks current implementation result")
		}
	}
	if body == "" {
		body = state.Prompt
	}
	parts, err := promptParts(state, facts, body, caller)
	if err != nil {
		return nil, err
	}
	return append(parts, policyParts(w, state)...), nil
}
