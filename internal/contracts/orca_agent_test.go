package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOrcaAgentSettings(t *testing.T) {
	for _, launch := range []string{`{"runtime_executable":""}`, `{"runtime_executable":"helper.exe"}`, `{"runtime_sha256":""}`, `{"runtime_sha256":"` + strings.Repeat("a", 64) + `"}`} {
		execution := Execution{Adapter: "orca-terminal", Target: Target{Kind: "orca_environment", Environment: "original"}, Settings: json.RawMessage(`{"access_mode":"full_access","agent_launch":` + launch + `}`)}
		if f := ValidateExecution(execution); f == nil || f.Detail.Code != "invalid_input" || !strings.Contains(f.Error(), "remove") {
			t.Fatal("remote helper key was accepted", launch, f)
		}
	}
	for _, tc := range []struct {
		settings string
		agent    string
		code     string
	}{
		{`{"access_mode":"full_access"}`, "codex", ""},
		{`{"access_mode":"full_access","agent":"codex"}`, "codex", ""},
		{`{"access_mode":"full_access","agent":"pi"}`, "pi", ""},
		{`{"agent":"dsh","access_mode":"full_access"}`, "dsh", ""},
		{`{"access_mode":"full_access","agent":"future-agent_2"}`, "future-agent_2", ""},
		{`{"access_mode":"full_access","agent":""}`, "", "invalid_input"},
		{`{"access_mode":"full_access","agent":null}`, "", "invalid_input"},
		{`{"access_mode":"full_access","agent":1}`, "", "invalid_input"},
		{`{"access_mode":"full_access","agent":"pi --unsafe"}`, "", "invalid_input"},
		{`{"access_mode":"full_access","agent":"../pi"}`, "", "invalid_input"},
		{`{"access_mode":"full_access","agent":"pi\n"}`, "", "invalid_input"},
		{`{"agent":"` + strings.Repeat("a", 65) + `"}`, "", "invalid_input"},
		{`{"Agent":"pi"}`, "", "invalid_input"},
		{`{"access_mode":"full_access","agent":"pi","agent_launch":{"model":"m"}}`, "pi", ""},
		{`{"access_mode":"full_access","agent":"dsh","agent_launch":{"reasoning_effort":"high"}}`, "dsh", ""},
		{`{"access_mode":"full_access","agent":"pi","agent_launch":{"args":["--no-daemon"]}}`, "", "unsupported_capability"},
		{`{"access_mode":"full_access","agent":"dsh","agent_launch":{"runtime_executable":"helper.exe"}}`, "dsh", ""},
		{`{"access_mode":"full_access","agent":"codex","agent_launch":{"model":"m","reasoning_effort":"high","args":["--no-daemon"]}}`, "codex", ""},
	} {
		t.Run(tc.settings, func(t *testing.T) {
			execution := Execution{Adapter: "orca-terminal", Target: Target{Kind: "local"}, Settings: json.RawMessage(tc.settings)}
			fault := ValidateExecution(execution)
			if tc.code != "" {
				if fault == nil || fault.Detail.Code != tc.code {
					t.Fatalf("wanted %s, got %v", tc.code, fault)
				}
				return
			}
			if fault != nil {
				t.Fatal(fault)
			}
			var settings OrcaSettings
			if err := DecodeStrict(execution.Settings, &settings); err != nil || settings.AgentName() != tc.agent {
				t.Fatal("agent selection changed", settings, err)
			}
		})
	}
	var legacy OrcaSettings
	if err := DecodeStrict([]byte(`{"transport":{"executable":"orca.exe"}}`), &legacy); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(legacy)
	if err != nil || strings.Contains(string(raw), `"agent":`) || legacy.AgentName() != "codex" {
		t.Fatal("decoding a legacy configuration materialized the agent default", string(raw), err)
	}
}
