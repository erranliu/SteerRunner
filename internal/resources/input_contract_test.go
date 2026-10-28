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
	for _, text := range []string{m.InputContract, workflow.ProgressInstruction, workflow.ResultFileInstruction, workflow.ContentRepositoryInstruction} {
		quoted, _ := json.Marshal(text)
		if !strings.Contains(string(raw), string(quoted)) {
			t.Fatal("discovery omits execution definition", text)
		}
	}
	if len(c["workflows"].([]workflow.Profile)) != 4 {
		t.Fatal("workflow removed")
	}
}
