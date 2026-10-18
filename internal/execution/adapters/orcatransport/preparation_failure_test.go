package orcatransport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestNativePreparationFailureIsBoundAndNeverReady(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		t.Run(agent, func(t *testing.T) {
			l, _, _ := fixture(t)
			c := &TerminalConfig{Agent: agent, StateDirectory: t.TempDir(), ExecutionID: strings.Repeat("a", 32), SessionID: "original", ConfigurationDigest: "config", AccessMode: "sandbox"}
			l.profile.Terminal = c
			l.profile.RuntimeExecutable, l.profile.RuntimeSHA256 = filepath.Join(c.StateDirectory, "host.exe"), "frozen-runtime"
			owner := TerminalOwner{RunID: c.ExecutionID, SessionID: c.SessionID, ConfigurationDigest: c.ConfigurationDigest, AccessMode: c.AccessMode, RuntimeSHA256: l.profile.RuntimeSHA256, ExtensionSHA256: TerminalExtensionDigest()}
			owner.Job.Handle = 17
			owner.Job.Owner = b.ProcessIdentity{Kind: "os", PID: 1, Created: "0123456789abcdef", Executable: strings.ToLower(l.profile.RuntimeExecutable)}
			owner.Job.Worker = b.ProcessIdentity{Kind: "os", PID: 2}
			body, _ := json.Marshal(owner)
			if err := os.WriteFile(filepath.Join(c.StateDirectory, "owner.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			fact := map[string]any{"version": 1, "run_id": c.ExecutionID, "session_id": c.SessionID, "extension_bundle_sha256": TerminalExtensionDigest(), "pid": 2, "failure": "native_tool_preparation_failed", "probe_stage": "network_timeout"}
			write := func() {
				t.Helper()
				body, _ := json.Marshal(fact)
				if err := os.WriteFile(filepath.Join(c.StateDirectory, "preparation-failure.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := State{}
			if err := l.readTerminalReadiness(&state, true); !os.IsNotExist(err) {
				t.Fatal("missing readiness/failure must remain pending", err)
			}
			write()
			if err := l.readTerminalReadiness(&state, true); err == nil || os.IsNotExist(err) || !strings.Contains(err.Error(), "native_tool_preparation_failed") {
				t.Fatal("failed initializer silently remained pending", err)
			}
			if state.TerminalReadiness != nil || state.TerminalOwner != nil {
				t.Fatal("failure established readiness or ownership authority")
			}
			state.LaunchConfirmed = true
			if err := l.readTerminalReadiness(&state, true); !os.IsNotExist(err) {
				t.Fatal("old startup failure replaced missing active-session evidence", err)
			}
			state.LaunchConfirmed = false
			for key, wrong := range map[string]any{"run_id": "other", "session_id": "replacement", "extension_bundle_sha256": "replacement", "pid": 3, "failure": "private-message"} {
				original := fact[key]
				fact[key] = wrong
				write()
				if err := l.readTerminalReadiness(&state, true); err == nil || strings.Contains(err.Error(), "private-message") || strings.Contains(err.Error(), "original native initialization failed") {
					t.Fatal("foreign/unknown failure was trusted", key, err)
				}
				fact[key] = original
			}
		})
	}
}
