package workflow

import (
	"encoding/json"
	"errors"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
)

type Snapshot struct {
	PublicationResultID    string         `json:"publication_result_id,omitempty"`
	Config                 *Profile       `json:"config"`
	ExecutionModel         *ModelSettings `json:"execution_model,omitempty"`
	AuditModel             *ModelSettings `json:"audit_model,omitempty"`
	ImplementationResultID string         `json:"implementation_result_id,omitempty"`
	AuditResultID          string         `json:"audit_result_id,omitempty"`
	Results                []ra.Result    `json:"results,omitempty"`
	Audits                 []AuditRecord  `json:"audits,omitempty"`
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
	state, ok := w.State(input.StepID)
	if !ok || state.Prompt == "" {
		return "", errors.New("no prompt in frozen state definition")
	}
	facts := map[string]any{"materials": input.MaterialFacts}
	if state.Kind == "report" {
		source, original := Result(w, input.SourceCommandID), Input(w, input.SourceCommandID)
		if source == nil || original == nil || source.TurnID != input.SourceTurnID || source.InputVersion != input.SourceVersion {
			return "", errors.New("report lacks exact original business source")
		}
		facts["source_binding"] = Source(*source)
		facts["native_succeeded"] = source.Succeeded
		facts["final_verified"] = source.FinalVerified
		facts["failure"] = source.Failure
		facts["workspace"] = source.Workspace
		facts["implementation_result_id"] = input.ImplementationResultID
	} else if state.Contract == "audit" {
		if !AuditSource(w, input) {
			return "", errors.New("audit lacks current implementation result")
		}
		facts["implementation_result"] = Acquired(w, input.ImplementationResultID)
	} else if state.Contract == "publication" {
		facts["implementation_result"] = Acquired(w, input.ImplementationResultID)
		facts["audit_result"] = Acquired(w, input.AuditResultID)
	}
	if body == "" {
		body = state.Prompt
	}
	text, err := renderPrompt(state, facts, body)
	return text, err
}
