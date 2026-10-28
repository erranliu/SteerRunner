package resources

import (
	"encoding/json"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"strings"
	"testing"
)

func TestInputContractDiscoveryMatchesOwners(t *testing.T) {
	c := Capabilities().(map[string]any)
	raw, err := json.Marshal(c["model_input_contract"])
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{m.InputContract, workflow.ProgressInstruction, workflow.ResultFileInstruction, workflow.ContentRepositoryInstruction, workflow.PolicyReadTemplate} {
		quoted, _ := json.Marshal(text)
		if !strings.Contains(string(raw), string(quoted)) {
			t.Fatal("discovery omits execution definition", text)
		}
	}
	for _, slot := range workflow.PolicySlots() {
		quoted, _ := json.Marshal(slot.Path)
		if !strings.Contains(string(raw), string(quoted)) {
			t.Fatal("discovery omits fixed policy path", slot.Path)
		}
	}
	if len(c["workflows"].([]workflow.Profile)) != 4 {
		t.Fatal("workflow removed")
	}
	var published struct {
		UTF8Bytes map[string]*int `json:"utf8_bytes"`
	}
	if err := json.Unmarshal(raw, &published); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"caller", "total"} {
		if value, ok := published.UTF8Bytes[key]; !ok || value != nil {
			t.Fatal("unlimited budget must be published as null", key)
		}
	}
	if value := published.UTF8Bytes["runner"]; value == nil || *value != m.RunnerInputBytes {
		t.Fatal("Runner limit changed")
	}
}
