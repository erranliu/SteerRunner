package workflow

import (
	"fmt"

	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

const PolicyReadTemplate = "\n阅读项目策略：%s。"

type PolicySlot struct {
	Slot string `json:"slot"`
	Path string `json:"path"`
}

// PolicySlots is the complete built-in list; callers cannot supply paths.
func PolicySlots() []PolicySlot {
	return []PolicySlot{
		{Slot: "git", Path: "SteerlineFlow/runner-policies/git.md"},
		{Slot: "command", Path: "SteerlineFlow/runner-policies/command.md"},
		{Slot: "testing", Path: "SteerlineFlow/runner-policies/testing.md"},
	}
}

type PolicyFile struct {
	PolicySlot
	Exists bool   `json:"exists"`
	SHA256 string `json:"sha256,omitempty"`
}

type ProjectPolicies struct {
	Commit string       `json:"commit"`
	Files  []PolicyFile `json:"files"`
}

func policyParts(w Snapshot, state State) []m.InputPart {
	var parts []m.InputPart
	if w.ProjectPolicies == nil || w.Config.PolicyReadTemplate == "" {
		return parts
	}
	for _, slot := range state.PolicySlots {
		for _, file := range w.ProjectPolicies.Files {
			if file.Slot == slot && file.Exists {
				// The only appended text is the frozen public workflow template.
				parts = append(parts, m.InputPart{Source: "project_policy." + slot, Owner: m.RunnerInput, Text: fmt.Sprintf(w.Config.PolicyReadTemplate, file.Path)})
			}
		}
	}
	return parts
}
