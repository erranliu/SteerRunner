package contracts

import (
	"encoding/json"
	"testing"
)

func TestRAPublicContract(t *testing.T) {
	var value map[string]any
	_ = json.Unmarshal(validRun(), &value)
	w := value["workflow"].(map[string]any)
	w["version"] = 1
	raw, _ := json.Marshal(value)
	if f := ValidateRun(raw); f == nil || f.Detail.Code != "unsupported_schema" {
		t.Fatal("workflow/1 accepted", f)
	}
	w["version"] = 2
	w["prompt_overrides"] = map[string]string{"audit.spec": "obsolete"}
	raw, _ = json.Marshal(value)
	if f := ValidateRun(raw); f == nil {
		t.Fatal("Spec audit override accepted")
	}
	w["prompt_overrides"] = map[string]string{"publish": "publish now"}
	raw, _ = json.Marshal(value)
	if f := ValidateRun(raw); f != nil {
		t.Fatal("retained publish override rejected", f)
	}
}
