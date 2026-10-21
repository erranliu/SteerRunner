package workflow

import (
	"encoding/json"
	"errors"
)

type FileReference struct {
	Path string `json:"path"`
}

const ImplementPrompt = "读取当前 worktree 中以下路径的 Spec、Plan 和参考材料，完成实现及必要验证。"
const reportPrompt = "根据已有执行上下文整理指定实施结果；只报告，不补做工作。注明依据，保留失败、未知情况、缺失和冲突。"
const publishPrompt = "将当前分支 push 到发布目标并创建 PR。用 git branch --show-current 取得实际分支，作为远端分支和 PR head。结果未知时说明情况，不重复创建。成功后给出 PR URL。"
const PublishReportPrompt = "根据已有上下文整理指定发布的结果和 PR URL；只报告，不重新发布。注明依据，保留失败、未知情况、缺失和冲突。"

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
	text := state.Prompt
	if body != "" {
		text = body
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	if state.Contract == "implementation" && state.Kind == "execute" {
		var input struct {
			Materials struct {
				Spec, Plan FileReference
				References []FileReference
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
		text += "\nSource facts:\n" + string(raw)
	}
	if state.Kind == "report" {
		text += "\n" + state.ResultInstruction
	}
	if state.CommitWork {
		text += "\n将完成的代码改动提交到当前分支。"
	}
	if state.SuccessOutput != "" {
		text += "\n完成后，在最终回复末尾单独写一行：" + state.SuccessOutput
	}
	return text, nil
}
