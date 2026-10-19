package contracts

import (
	"encoding/json"
	"testing"
)

func TestRemoteAuditedModelsRequireManagedRuntime(t *testing.T) {
	var request map[string]any
	_ = json.Unmarshal(validRun(), &request)
	request["workflow"] = map[string]any{"id": "development-audited", "version": 1, "audit_model": map[string]any{"model": "reviewer", "reasoning_effort": "high"}}
	request["execution"] = map[string]any{"adapter": "orca-terminal", "target": map[string]any{"kind": "orca_environment", "environment": "paired"}, "settings": map[string]any{"agent_launch": map[string]any{"executable": "codex.exe", "runtime_executable": "C:/trusted/helper.exe", "model": "writer", "reasoning_effort": "high"}}}
	wire, _ := json.Marshal(request)
	if fault := ValidateRun(wire); fault != nil {
		t.Fatal("valid remote configuration rejected", fault)
	}
	for _, mode := range []string{"no-helper", "implicit-writer", "same-model"} {
		t.Run(mode, func(t *testing.T) {
			var copy map[string]any
			_ = json.Unmarshal(wire, &copy)
			s := copy["execution"].(map[string]any)["settings"].(map[string]any)
			launch := s["agent_launch"].(map[string]any)
			switch mode {
			case "no-helper":
				delete(launch, "runtime_executable")
			case "implicit-writer":
				delete(launch, "model")
			case "same-model":
				launch["model"] = "reviewer"
			}
			raw, _ := json.Marshal(copy)
			if ValidateRun(raw) == nil {
				t.Fatal("invalid remote audit/authentication accepted", mode)
			}
		})
	}
}

func TestTerminalAgentsCannotUseCodexAuditModels(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		t.Run(agent, func(t *testing.T) {
			execution := Execution{Adapter: "orca-terminal", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"agent":"` + agent + `","agent_launch":{"runtime_executable":"helper.exe","model":"writer"}}`)}
			if fault := ValidateExecution(execution); fault != nil {
				t.Fatal(fault)
			}
			if _, err := ExecutionModel(execution); err == nil {
				t.Fatal("native terminal agent admitted to Codex model switching")
			}
		})
	}
}

// Removed authentication settings must never silently choose a different actor.
func TestGitHubAppConfigurationRejectedBeforeRunOrCheck(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		for _, legacy := range []string{`null`, `{}`, `{"app_id":3,"installation_id":7,"private_key_env":"OLD_APP_KEY"}`} {
			t.Run(adapter+legacy, func(t *testing.T) {
				settings := json.RawMessage(`{"github_auth":` + legacy + `}`)
				execution := Execution{Adapter: adapter, Target: Target{Kind: "local"}, Settings: settings}
				var request map[string]any
				_ = json.Unmarshal(validRun(), &request)
				request["execution"] = execution
				raw, _ := json.Marshal(request)
				if fault := ValidateRun(raw); fault == nil || fault.Detail.Code != "invalid_input" {
					t.Fatal("legacy App Run accepted", fault)
				}
				check := checkInputFixture()
				check.Execution = execution
				raw, _ = json.Marshal(check)
				if fault := ValidateEnvironmentCheck(raw); fault == nil || fault.Detail.Code != "invalid_input" {
					t.Fatal("legacy App check accepted", fault)
				}
			})
		}
	}
}
