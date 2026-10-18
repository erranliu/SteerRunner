package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOrcaAgentSettings(t *testing.T) {
	for _, tc := range []struct {
		settings string
		agent    string
		code     string
	}{
		{`{}`, "codex", ""},
		{`{"agent":"codex"}`, "codex", ""},
		{`{"agent":"pi"}`, "pi", ""},
		{`{"agent":"dsh","access_mode":"full_access"}`, "dsh", ""},
		{`{"agent":"future-agent_2"}`, "future-agent_2", ""},
		{`{"agent":""}`, "", "invalid_input"},
		{`{"agent":null}`, "", "invalid_input"},
		{`{"agent":1}`, "", "invalid_input"},
		{`{"agent":"pi --unsafe"}`, "", "invalid_input"},
		{`{"agent":"../pi"}`, "", "invalid_input"},
		{`{"agent":"pi\n"}`, "", "invalid_input"},
		{`{"agent":"` + strings.Repeat("a", 65) + `"}`, "", "invalid_input"},
		{`{"Agent":"pi"}`, "", "invalid_input"},
		{`{"agent":"pi","agent_launch":{"model":"m"}}`, "pi", ""},
		{`{"agent":"dsh","agent_launch":{"reasoning_effort":"high"}}`, "dsh", ""},
		{`{"agent":"pi","agent_launch":{"args":["--no-daemon"]}}`, "", "unsupported_capability"},
		{`{"agent":"dsh","agent_launch":{"runtime_executable":"helper.exe"}}`, "dsh", ""},
		{`{"agent":"codex","agent_launch":{"model":"m","reasoning_effort":"high","args":["--no-daemon"]}}`, "codex", ""},
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
