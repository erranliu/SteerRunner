package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"runtime"
	"testing"
)

func TestRAPublicContract(t *testing.T) {
	c := Capabilities().(map[string]any)
	if c["resource_version"] != 3 || c["protocol"] != "steerrunner-cli/v1" {
		t.Fatal(c)
	}
	for _, p := range workflow.Catalog() {
		if p.Version != 2 && !(workflow.Content(p.ID) && p.Version == 1) {
			t.Fatal(p)
		}
	}
	if _, ok := workflow.Lookup("development", 1); ok {
		t.Fatal("old workflow accepted")
	}
	var prompts map[string]string
	_ = json.Unmarshal(Read("prompts.json"), &prompts)
	if _, ok := prompts["audit.spec"]; ok {
		t.Fatal("Spec audit still published")
	}
	if prompts["publish"] == "" || prompts["audit.code.report"] == "" {
		t.Fatal("prompt catalog incomplete")
	}
	if c["parser_support"].(map[string]any)["supported"] != (runtime.GOOS == "windows") || c["parser_support"].(map[string]any)["empty_tool_guarantee"] != false {
		t.Fatal("minimal parser profile boundary misrepresented")
	}
}
