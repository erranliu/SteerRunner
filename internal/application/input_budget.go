package application

import (
	"errors"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"strings"
)

func priorFileInstruction(r *runstate.Record, source string) bool {
	for _, i := range r.Workflow.StageInputs {
		if i.Budget == nil {
			continue
		}
		result := workflow.Result(r.Workflow, i.CommandID)
		if result == nil || result.SessionID == "" || result.SessionID != r.Snapshot.Session.SessionID || !result.FinalVerified {
			continue
		}
		for _, p := range i.Budget.Fragments {
			if p.Source == source {
				return true
			}
		}
	}
	return false
}

// Reconstruct ownership only from the original frozen body/answer. Never
// render current defaults over an old input or guess that everything is caller.
func ensureStageBudget(r *runstate.Record, i *workflow.StageInput) error {
	if i.Budget == nil {
		state, ok := r.Workflow.State(i.StepID)
		if !ok {
			return errors.New("frozen input state unavailable")
		}
		if state.Contract == "final" {
			for _, entry := range r.History {
				if entry.RequestCommandID == i.FeedbackDecisionID && i.FeedbackDecisionID != "" && entry.Role == "user" && entry.Text == i.Prompt {
					_, i.Budget, _ = m.DescribeInput(m.InputPart{Source: "caller.message", Owner: m.CallerInput, Text: i.Prompt})
					break
				}
			}
			if i.Budget == nil && i.FeedbackDecisionID == "" {
				instruction, _, _ := r.Request.ContentInput()
				prefix := "Instruction:\n" + instruction + "\nFrozen submitted materials:\n"
				materials := []map[string]string{}
				for _, v := range r.Materials {
					materials = append(materials, map[string]string{"path": v.Path, "sha256": v.Digest, "content": v.Content})
				}
				raw := string(canonical(materials))
				if !strings.HasPrefix(i.Prompt, prefix+raw) {
					return errors.New("legacy content source unavailable; candidate not sent")
				}
				parts := []m.InputPart{{Source: "legacy.content.label", Owner: m.RunnerInput, Text: "Instruction:\n"}, {Source: "caller.instruction", Owner: m.CallerInput, Text: instruction}, {Source: "legacy.content.label", Owner: m.RunnerInput, Text: "\nFrozen submitted materials:\n"}}
				offset := 0
				for _, v := range r.Materials {
					encoded := string(canonical(v.Content))
					marker := `"content":` + encoded
					at := strings.Index(raw[offset:], marker)
					if at < 0 {
						return errors.New("legacy material source unavailable")
					}
					start := offset + at + len(`"content":`) + 1
					parts = append(parts, m.InputPart{Source: "legacy.content.frame", Owner: m.RunnerInput, Text: raw[offset:start]}, m.InputPart{Source: "caller.material", Owner: m.CallerInput, Text: encoded[1 : len(encoded)-1]})
					offset = start + len(encoded) - 2
				}
				parts = append(parts, m.InputPart{Source: "legacy.content.frame", Owner: m.RunnerInput, Text: raw[offset:]})
				rest := i.Prompt[len(prefix)+len(raw):]
				if r.Request.Generate != nil && len(r.Request.Generate.OutputSchema) > 0 {
					schema := string(r.Request.Generate.OutputSchema)
					if !strings.HasSuffix(rest, schema) {
						return errors.New("legacy schema source unavailable")
					}
					parts = append(parts, m.InputPart{Source: "legacy.content.wrapper", Owner: m.RunnerInput, Text: rest[:len(rest)-len(schema)]}, m.InputPart{Source: "caller.schema", Owner: m.CallerInput, Text: schema})
				} else {
					parts = append(parts, m.InputPart{Source: "legacy.content.wrapper", Owner: m.RunnerInput, Text: rest})
				}
				_, budget, err := m.DescribeInput(parts...)
				if err != nil {
					return err
				}
				i.Budget = budget
			}
			if i.Budget == nil {
				return errors.New("legacy content input provenance unavailable; candidate not sent")
			}
		} else {
			body := r.PromptBodies[state.PromptKey]
			if body == "" || !strings.HasPrefix(i.Prompt, body) {
				return errors.New("legacy frozen body does not match its source; candidate not sent")
			}
			owner := m.RunnerInput
			if _, ok := r.Request.Workflow.PromptOverrides[state.PromptKey]; ok {
				owner = m.CallerInput
			}
			parts := []m.InputPart{{Source: "workflow.body", Owner: owner, Text: body}}
			rest := strings.TrimPrefix(i.Prompt, body)
			if i.FeedbackDecisionID != "" {
				answer := ""
				for _, a := range r.AnswerHistory {
					if a.CommandID == i.FeedbackDecisionID {
						answer = a.Text
					}
				}
				for _, a := range []*runstate.Answer{r.PendingAnswer, r.PendingFeedback} {
					if a != nil && a.CommandID == i.FeedbackDecisionID {
						answer = a.Text
					}
				}
				label := "\nUser feedback for the original session:\n"
				at := strings.Index(rest, label)
				if at < 0 {
					label = workflow.FeedbackLabel
					at = strings.Index(rest, label)
				}
				if answer == "" || at < 0 || !strings.HasPrefix(rest[at+len(label):], answer) {
					return errors.New("legacy feedback source unavailable; candidate not sent")
				}
				parts = append(parts, m.InputPart{Source: "legacy.wrapper", Owner: m.RunnerInput, Text: rest[:at+len(label)]}, m.InputPart{Source: "caller.feedback", Owner: m.CallerInput, Text: answer})
				rest = rest[at+len(label)+len(answer):]
			}
			parts = append(parts, m.InputPart{Source: "legacy.wrapper", Owner: m.RunnerInput, Text: rest})
			_, budget, err := m.DescribeInput(parts...)
			if err != nil {
				return err
			}
			i.Budget = budget
		}
	}
	parts, err := m.InputParts(i.Prompt, i.Budget)
	if err != nil {
		return err
	}
	if r.Request.Execution.Adapter == "orca-terminal" {
		parts = m.NativeInputParts(parts)
	}
	_, _, err = m.FreezeInput(parts...)
	return err
}
