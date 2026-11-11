package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"slices"
	"testing"
)

func TestSRWorkflowSelection(t *testing.T) {
	data := Capabilities().(map[string]any)
	catalog := data["workflows"].([]workflow.Profile)
	var schema struct {
		Workflow struct {
			Properties struct {
				ID struct {
					Enum []string `json:"enum"`
				} `json:"id"`
			} `json:"properties"`
		} `json:"workflow"`
	}
	if err := json.Unmarshal(Read("settings.schema.json"), &schema); err != nil {
		t.Fatal(err)
	}
	var prompts map[string]string
	if err := json.Unmarshal(Read("prompts.json"), &prompts); err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 6 || len(schema.Workflow.Properties.ID.Enum) != 5 || data["workflow"] == nil {
		t.Fatal("discovery mismatch")
	}
	for _, p := range catalog {
		if !slices.Contains(schema.Workflow.Properties.ID.Enum, p.ID) {
			t.Fatal("schema missing workflow")
		}
		for _, step := range p.PromptSteps {
			if prompts[step] == "" {
				t.Fatal("missing prompt", step)
			}
		}
		for _, state := range p.States {
			expected := prompts[state.PromptKey]
			// Plain and audited development intentionally share a step key but
			// have different existing default sources. The published legacy asset
			// contains the audited default; it cannot stand for both workflows.
			if p.ID == "development" && state.PromptKey == workflow.StepImplement {
				expected = workflow.ImplementPrompt
				if expected == prompts[workflow.StepImplement] {
					t.Fatal("plain and audited implementation defaults unexpectedly collapsed")
				}
			}
			if state.PromptKey != "" && expected != state.Prompt {
				t.Fatal("published prompt differs from the frozen execution default", state.PromptKey)
			}
		}
	}
}
