package contracts

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"testing"
)

func TestSRWorkflowSelection(t *testing.T) {
	var base map[string]any
	if err := json.Unmarshal(validRun(), &base); err != nil {
		t.Fatal(err)
	}
	base["workflow"] = map[string]any{"id": workflow.AuditedDevelopment, "version": 2, "audit_model": map[string]any{"model": "reviewer"}, "prompt_overrides": map[string]any{"audit.code": "custom audit"}}
	base["execution"].(map[string]any)["settings"] = map[string]any{"access_mode": "full_access", "model": "executor"}
	data, _ := json.Marshal(base)
	if fault := ValidateRun(data); fault != nil {
		t.Fatal(fault)
	}
	for name, edit := range map[string]func(map[string]any){
		"missing_model": func(v map[string]any) { delete(v["workflow"].(map[string]any), "audit_model") },
		"null":          func(v map[string]any) { v["workflow"].(map[string]any)["audit_model"] = nil },
		"blank": func(v map[string]any) {
			v["workflow"].(map[string]any)["audit_model"].(map[string]any)["model"] = " reviewer "
		},
		"effort_null": func(v map[string]any) {
			v["workflow"].(map[string]any)["audit_model"].(map[string]any)["reasoning_effort"] = nil
		},
		"control": func(v map[string]any) {
			v["workflow"].(map[string]any)["audit_model"].(map[string]any)["model"] = "review\ter"
		},
		"executable": func(v map[string]any) {
			v["workflow"].(map[string]any)["audit_model"].(map[string]any)["executable"] = "other"
		},
		"implicit_execution": func(v map[string]any) {
			v["execution"].(map[string]any)["settings"] = map[string]any{"access_mode": "full_access"}
		},
		"wrong_step": func(v map[string]any) {
			v["workflow"].(map[string]any)["prompt_overrides"] = map[string]any{"repair.execute": "fix"}
		},
		"plain_audit": func(v map[string]any) { v["workflow"].(map[string]any)["id"] = "development" },
		"unknown":     func(v map[string]any) { v["workflow"].(map[string]any)["id"] = "unknown" },
		"orca":        func(v map[string]any) { v["execution"].(map[string]any)["adapter"] = "orca-terminal" },
	} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(data, &v)
			edit(v)
			raw, _ := json.Marshal(v)
			if ValidateRun(raw) == nil {
				t.Fatal("invalid choice accepted")
			}
		})
	}
	if fault := ValidateRun(validRun()); fault != nil {
		t.Fatal("legacy request rejected", fault)
	}
}
