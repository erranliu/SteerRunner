package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDevelopmentPermissionsRequireExplicitFullAccess(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		for _, settings := range []string{`{}`, `{"access_mode":""}`, `{"access_mode":null}`, `{"access_mode":false}`, `{"access_mode":1}`, `{"access_mode":"sandbox"}`, `{"access_mode":"workspace-write"}`} {
			e := Execution{Adapter: adapter, Target: Target{Kind: "local"}, Settings: json.RawMessage(settings)}
			f := ValidateExecution(e)
			if f == nil || f.Exit != 2 || f.Detail.Code != "invalid_input" || !strings.Contains(f.Error(), "access_mode") {
				t.Fatalf("%s %s: %v", adapter, settings, f)
			}
		}
		e := Execution{Adapter: adapter, Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"full_access"}`)}
		if f := ValidateExecution(e); f != nil {
			t.Fatal(f)
		}
	}
	local := Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"read_only"}`)}
	if f := ValidateExecution(local); f != nil {
		t.Fatal("read_only lost", f)
	}
	raw, _ := json.Marshal(RunConfiguration{Execution: local, Workflow: Workflow{ID: "development", Version: 2}})
	if f := ValidateRunConfiguration(raw); f == nil {
		t.Fatal("read_only accepted for development")
	}
}
