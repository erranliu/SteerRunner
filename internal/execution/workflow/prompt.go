package workflow

import (
	"encoding/json"
	"errors"
)

type FileReference struct {
	Path string `json:"path"`
}

const ImplementPrompt = "Implement the following Spec and Plan."
const reportPrompt = "Format the preceding implementation result as JSON. Do not change files, run checks or publish."
const publishPrompt = "Push and create or reuse a PR using the current checked-out branch. Read its exact name with git branch --show-current and use that same name for the remote branch and PR head. Do not reconstruct the branch name from the task ID, workflow run ID, workspace path or previous messages."
const PublishReportPrompt = "Format the preceding publication result as JSON. Do not change files, run checks or publish."

func RenderPrompt(step string, facts any) (string, error) {
	return RenderPromptWithBody(step, facts, "")
}
func RenderPromptWithBody(step string, facts any, body string) (string, error) {
	text := ""
	switch step {
	case StepImplement:
		text = ImplementPrompt
	case StepReport:
		text = reportPrompt
	case StepPublish:
		text = publishPrompt
	case StepPublishReport:
		text = PublishReportPrompt
	case StepAuditSpec:
		text = "Audit the current implementation against the frozen Spec and Plan. Report only evidenced deviations. Do not repair, commit or publish."
	case StepAuditCode:
		text = "Audit the current code for correctness, regressions, edge cases, tests and evidenced security issues. Do not repair, commit or publish."
	default:
		return "", errors.New("no automatic prompt for this step")
	}
	if body != "" {
		text = body
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	if step == StepImplement {
		var input struct {
			Materials struct {
				Spec       FileReference   `json:"spec"`
				Plan       FileReference   `json:"plan"`
				References []FileReference `json:"references"`
			} `json:"materials"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return "", err
		}
		text += "\nSpec: " + input.Materials.Spec.Path + "\nPlan: " + input.Materials.Plan.Path
		seen := map[string]bool{input.Materials.Spec.Path: true, input.Materials.Plan.Path: true}
		for _, ref := range input.Materials.References {
			if ref.Path != "" && !seen[ref.Path] {
				text += "\nReference: " + ref.Path
				seen[ref.Path] = true
			}
		}
	} else if string(raw) != "{}" && string(raw) != "null" {
		text += "\nDiagnostics:\n" + string(raw)
	}
	if step == StepReport || PublicationStep(step) || AuditStep(step) {
		text += "\n" + ResultInstruction(step)
	}
	return text, nil
}
