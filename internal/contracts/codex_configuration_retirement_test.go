package contracts

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCodexConfigurationRetainsModelValidation(t *testing.T) {
	configuration := RunConfiguration{Execution: Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}}, Workflow: Workflow{ID: "development", Version: 2}}
	for _, model := range []string{"chosen-model", " bad-model "} {
		configuration.Execution.Settings, _ = json.Marshal(CodexSettings{AccessMode: "full_access", Executable: "must-not-run", Model: model})
		raw, err := json.Marshal(configuration)
		if err != nil {
			t.Fatal(err)
		}
		fault := ValidateRunConfiguration(raw)
		if (fault == nil) != (model == "chosen-model") {
			t.Fatalf("model %q: %v", model, fault)
		}
	}
}

func TestCodexConfigurationRejectsRetiredOverrides(t *testing.T) {
	configuration := RunConfiguration{Execution: Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}}, Workflow: Workflow{ID: "development", Version: 2}}
	configuration.Execution.Settings = json.RawMessage(`{"access_mode":"full_access","executable":"must-not-run"}`)
	base, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	if fault := ValidateRunConfiguration(base); fault != nil {
		t.Fatal("invalid positive fixture", fault)
	}
	for _, field := range []string{"args", "profile", "windows_sandbox", "sandbox", "sandbox_mode", "approval_policy", "approvals_reviewer"} {
		for i, value := range []any{nil, "", []string{}, map[string]any{}} {
			t.Run(fmt.Sprintf("%s/value-%d", field, i), func(t *testing.T) {
				settings := map[string]any{"access_mode": "full_access", "executable": "must-not-run", field: value}
				configuration.Execution.Settings, err = json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(configuration)
				if err != nil {
					t.Fatal(err)
				}
				if fault := ValidateRunConfiguration(raw); fault == nil || fault.Exit != 2 || fault.Detail.Code != "invalid_input" {
					t.Fatalf("retired override accepted: %s: %v", raw, fault)
				}
			})
		}
	}
}
