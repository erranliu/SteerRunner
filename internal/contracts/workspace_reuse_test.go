package contracts

import (
	"encoding/json"
	"testing"
)

func TestWorktreeReuseAdmissionAndExitCodes(t *testing.T) {
	var input map[string]any
	_ = json.Unmarshal(validRun(), &input)
	input["reuse_worktree_from_run_id"] = "source"
	raw, _ := json.Marshal(input)
	if ValidateRun(raw) == nil {
		t.Fatal("reuse accepted outside execute")
	}
	delete(input, "publication")
	input["workflow"] = map[string]any{"id": "execute", "version": 1, "prompt_overrides": map[string]string{"execute": "finish retained work"}}
	raw, _ = json.Marshal(input)
	if err := ValidateRun(raw); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{nil, "", "bad\nsource", map[string]string{"path": "C:/writable"}} {
		input["reuse_worktree_from_run_id"] = value
		raw, _ = json.Marshal(input)
		if ValidateRun(raw) == nil {
			t.Fatal("invalid source accepted", value)
		}
	}
	for _, code := range []string{"workspace_not_settled", "workspace_already_reused", "workspace_transferred", "workspace_identity_unavailable", "workspace_identity_conflict"} {
		if ExitCode(Rejected("", Fail(5, code, "test"))) != 5 {
			t.Fatal("wrong state conflict exit", code)
		}
	}
}
