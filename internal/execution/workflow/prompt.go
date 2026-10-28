package workflow

import (
	"encoding/json"
	"errors"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

type FileReference struct {
	Path string `json:"path"`
}

const ImplementPrompt = "读取所列 Spec/Plan/材料，实施并完成必要验证。"
const reportPrompt = "仅整理原实施结果，不补做；保留依据、失败、未知、缺失与冲突。"
const publishPrompt = "发布当前分支并创建 PR；用 git branch --show-current 确定 head。返回真实 PR URL；结果未知时说明并停止，不重复发布。"
const PublishReportPrompt = "仅整理原发布结果与 PR URL，不重发；保留依据、失败、未知、缺失与冲突。"
const FeedbackLabel = "\n反馈：\n"
const SuccessLabel = "\n成功时末行："
const CommitInstruction = "\n将完成的代码改动提交到当前分支。"
const FactsLabel = "\nSource:\n"

func RenderPrompt(step string, facts any) (string, error) {
	return RenderPromptWithBody(step, facts, "")
}
func RenderPromptWithBody(step string, facts any, body string) (string, error) {
	states := developmentStates(AuditStep(step))
	s, ok := states[step]
	if !ok {
		return "", errors.New("no prompt for state")
	}
	return renderPrompt(s, facts, body)
}
func renderPrompt(state State, facts any, body string) (string, error) {
	parts, err := promptParts(state, facts, body, body != "")
	if err != nil {
		return "", err
	}
	text, _, err := m.FreezeInput(parts...)
	return text, err
}
func promptParts(state State, facts any, body string, caller bool) ([]m.InputPart, error) {
	text := state.Prompt
	if body != "" {
		text = body
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		return nil, err
	}
	owner := m.RunnerInput
	if caller {
		owner = m.CallerInput
	}
	parts := []m.InputPart{{Source: "workflow.body", Owner: owner, Text: text}}
	if state.Contract == "implementation" && state.Kind == "execute" {
		var input struct {
			Materials struct {
				Spec, Plan FileReference
				References []FileReference
			} `json:"materials"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, err
		}
		parts = append(parts, m.InputPart{Source: "materials", Owner: m.RunnerInput, Text: "\nSpec: " + input.Materials.Spec.Path + "\nPlan: " + input.Materials.Plan.Path})
		seen := map[string]bool{input.Materials.Spec.Path: true, input.Materials.Plan.Path: true}
		for _, ref := range input.Materials.References {
			if ref.Path != "" && !seen[ref.Path] {
				parts = append(parts, m.InputPart{Source: "materials", Owner: m.RunnerInput, Text: "\nReference: " + ref.Path})
				seen[ref.Path] = true
			}
		}
	} else if string(raw) != "{}" && string(raw) != "null" {
		parts = append(parts, m.InputPart{Source: "source_facts", Owner: m.RunnerInput, Text: FactsLabel + string(raw)})
	}
	if state.Kind == "report" {
		parts = append(parts, m.InputPart{Source: "result_format", Owner: m.RunnerInput, Text: "\n" + state.ResultInstruction})
	}
	if state.CommitWork {
		parts = append(parts, m.InputPart{Source: "commit", Owner: m.RunnerInput, Text: CommitInstruction})
	}
	if state.SuccessOutput != "" {
		parts = append(parts, m.InputPart{Source: "success", Owner: m.RunnerInput, Text: SuccessLabel + state.SuccessOutput})
	}
	return parts, nil
}
