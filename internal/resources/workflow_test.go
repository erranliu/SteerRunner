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
	if len(catalog) != 2 || len(schema.Workflow.Properties.ID.Enum) != 2 || data["workflow"] == nil {
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
	}
}
