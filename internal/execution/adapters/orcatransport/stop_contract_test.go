package orcatransport

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestStopContractMarkersPreserveOriginalClassification(t *testing.T) {
	marker := func(key string, version int, payload string) b.ImplementationDataEnvelope {
		return b.ImplementationDataEnvelope{ImplementationKey: key, CodecVersion: version, Payload: json.RawMessage(payload)}
	}
	close := marker("orca.agent-session-profile", 2, `{}`)
	native := marker("orca.terminal-profile", 9, `{}`)
	for _, tc := range []struct {
		name     string
		profiles []b.ImplementationDataEnvelope
		close    bool
		native   bool
	}{
		{"missing", nil, false, false},
		{"minimal-close", []b.ImplementationDataEnvelope{close}, true, false},
		{"earlier-close", []b.ImplementationDataEnvelope{marker("orca.agent-session-profile", 1, `{}`)}, false, false},
		{"future-close", []b.ImplementationDataEnvelope{marker("orca.agent-session-profile", 3, `{}`)}, false, false},
		{"wrong-key", []b.ImplementationDataEnvelope{marker("other", 2, `{}`)}, false, false},
		{"bad-payload", []b.ImplementationDataEnvelope{marker("orca.agent-session-profile", 2, `{`)}, true, false},
		{"duplicate", []b.ImplementationDataEnvelope{close, close}, true, false},
		{"native", []b.ImplementationDataEnvelope{native}, false, true},
		{"other-native-version", []b.ImplementationDataEnvelope{marker("orca.terminal-profile", 8, `{}`)}, false, false},
		{"mixed", []b.ImplementationDataEnvelope{native, close, marker("orca.agent-session-profile", -1, "")}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := make([]b.ImplementationDataEnvelope, len(tc.profiles))
			for i, profile := range tc.profiles {
				before[i] = profile
				before[i].Payload = bytes.Clone(profile.Payload)
			}
			for _, protocol := range []b.ExecutionProtocol{b.ProtocolOrcaAgentSessionV1, b.ProtocolOrcaTerminalV3, b.ProtocolLocalCodexAppServer, ""} {
				if got := DevelopmentCloseContract(protocol, tc.profiles); got != (protocol == b.ProtocolOrcaAgentSessionV1 && tc.close) {
					t.Fatal("close classification changed", protocol, got)
				}
			}
			for _, scope := range []string{"", "orca_terminal", "orca_agent_session", "orca_local_codex_resources", "other"} {
				if got := RequiresFrozenStopScope(tc.profiles, scope); got != (tc.native || scope == "orca_local_codex_resources") {
					t.Fatal("scope classification changed", scope, got)
				}
			}
			if len(tc.profiles) != 0 && !reflect.DeepEqual(before, tc.profiles) {
				t.Fatal("classification mutated the original envelopes")
			}
		})
	}
}

func TestConfirmedStopDiagnosticUsesSavedScope(t *testing.T) {
	for _, tc := range []struct{ scope, want string }{
		{"orca_local_codex_resources", "Stopped the registered original Orca terminals and local Codex helper/root processes; all descendant processes and execution-host credentials were not verified or revoked."},
		{"orca_agent_session", "Closed the original Orca session; detached processes and additional resource cleanup are not verified."},
		{"orca_terminal", "Stopped the original Orca terminal; detached background processes and execution-host credentials were not verified or revoked."},
		{"", "Stopped the original Orca terminal; detached background processes and execution-host credentials were not verified or revoked."},
		{"other", "Stopped the original Orca terminal; detached background processes and execution-host credentials were not verified or revoked."},
	} {
		if got := ConfirmedStopDiagnostic(tc.scope); got != tc.want {
			t.Fatal(tc.scope, got)
		}
	}
}
