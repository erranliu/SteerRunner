package contracts

import (
	"encoding/json"
	"testing"
)

func TestRemoteAuditedAndAppReferencesRequireManagedRuntime(t *testing.T) {
	var request map[string]any
	_ = json.Unmarshal(validRun(), &request)
	request["workflow"] = map[string]any{"id": "development-audited", "version": 1, "audit_model": map[string]any{"model": "reviewer", "reasoning_effort": "high"}}
	request["execution"] = map[string]any{"adapter": "orca-terminal", "target": map[string]any{"kind": "orca_environment", "environment": "paired"}, "settings": map[string]any{"agent_launch": map[string]any{"executable": "codex.exe", "runtime_executable": "C:/trusted/helper.exe", "model": "writer", "reasoning_effort": "high"}, "github_auth": map[string]any{"app_id": 3, "installation_id": 7, "private_key_env": "REMOTE_APP_KEY"}}}
	wire, _ := json.Marshal(request)
	if fault := ValidateRun(wire); fault != nil {
		t.Fatal("valid remote configuration rejected", fault)
	}
	for _, mode := range []string{"no-helper", "implicit-writer", "same-model", "inline-key", "invalid-reference", "invalid-app", "invalid-installation"} {
		t.Run(mode, func(t *testing.T) {
			var copy map[string]any
			_ = json.Unmarshal(wire, &copy)
			s := copy["execution"].(map[string]any)["settings"].(map[string]any)
			launch := s["agent_launch"].(map[string]any)
			auth := s["github_auth"].(map[string]any)
			switch mode {
			case "no-helper":
				delete(launch, "runtime_executable")
			case "implicit-writer":
				delete(launch, "model")
			case "same-model":
				launch["model"] = "reviewer"
			case "inline-key":
				auth["private_key"] = "private-key-must-not-enter-config"
			case "invalid-reference":
				auth["private_key_env"] = "key with spaces"
			case "invalid-app":
				auth["app_id"] = 0
			case "invalid-installation":
				auth["installation_id"] = 0
			}
			raw, _ := json.Marshal(copy)
			if ValidateRun(raw) == nil {
				t.Fatal("invalid remote audit/authentication accepted", mode)
			}
		})
	}
}
