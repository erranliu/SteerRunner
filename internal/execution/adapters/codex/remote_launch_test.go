package codexworker

import (
	"os"
	"strings"
	"testing"
)

func TestRemoteNativeHookPolicyPreservesOnlyScopedTransport(t *testing.T) {
	worker := &workerRuntime{Shell: os.Args[0], Temp: t.TempDir(), Auth: testAuthentication(t), OrcaHooks: true}
	worker.Auth.broker = &authenticationBroker{address: "http://127.0.0.1:1234", caPath: "public-ca.pem", configDir: "public-config"}
	env, err := worker.environment([]string{"PATH=tools", "ORCA_AGENT_HOOK_TOKEN=pane-hook-secret", "ORCA_PAIRING_CODE=runtime-pairing-secret"})
	if err != nil {
		t.Fatal(err)
	}
	args, err := worker.configArgs(env)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, required := range []string{`GIT_CONFIG_KEY_0="maintenance.auto"`, `GIT_CONFIG_VALUE_1="0"`, `"ORCA_AGENT_HOOK_TOKEN"`, `shell_environment_policy.set.NO_PROXY="127.0.0.1,localhost,::1"`} {
		if !strings.Contains(joined, required) {
			t.Fatalf("remote native hook transport omitted %q", required)
		}
	}
	for _, forbidden := range []string{"pane-hook-secret", "runtime-pairing-secret", "ORCA_PAIRING_CODE", `"ORCA_*"`, worker.Auth.token} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("remote native hook policy exported %q", forbidden)
		}
	}
	worker.OrcaHooks = false
	args, err = worker.configArgs(env)
	if err != nil || strings.Contains(strings.Join(args, " "), "ORCA_") || !strings.Contains(strings.Join(args, " "), `shell_environment_policy.set.NO_PROXY=""`) {
		t.Fatal("ordinary local worker inherited Orca hook transport", err)
	}
}

func TestRemoteTUIUsesServerOwnedPermissionsAndRoots(t *testing.T) {
	args := remoteTUIArgs(RemoteAgentConfig{Args: []string{"--no-daemon", "--no-alt-screen"}, Model: "server-model", ReasoningEffort: "high"}, "ws://127.0.0.1:1234", "original-session")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--remote ws://127.0.0.1:1234") || !strings.HasSuffix(joined, "resume original-session") {
		t.Fatal("TUI lost original server/session")
	}
	if strings.Count(joined, "--no-alt-screen") != 1 {
		t.Fatal("inline TUI must preserve the preparation receipt without duplicate flags")
	}
	for _, forbidden := range []string{"--no-daemon", "--add-dir", "sandbox_", "approval_", "shell_environment_policy", "server-model", "model_reasoning_effort"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("TUI tried to override server-owned setting %q", forbidden)
		}
	}
}
