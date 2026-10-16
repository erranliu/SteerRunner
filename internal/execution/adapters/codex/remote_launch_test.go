package codexworker

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestRemoteAuthenticationSocketBindsWorkspaceAndConfiguration(t *testing.T) {
	state, err := os.MkdirTemp("", "sr-remote-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(state) })
	t.Setenv("LOCALAPPDATA", state)
	cfg := RemoteAgentConfig{Executable: "codex.exe", RepositoryURL: "https://github.com/owner/repo.git", Model: "writer"}
	workspace := filepath.Join(state, "workspaces", strings.Repeat("w", 100))
	first, err := remoteAuthenticationTransport(cfg, workspace)
	if err != nil || first.validate() != nil || len(filepath.Join(first.Directory, "s")) > authenticationSocketLimit {
		t.Fatal("deep workspace needs a short, bound socket", first, err)
	}
	again, err := remoteAuthenticationTransport(cfg, workspace)
	if err != nil || *first != *again {
		t.Fatal("same frozen input changed authentication identity", err)
	}
	peer, err := remoteAuthenticationTransport(cfg, workspace+"-peer")
	if err != nil || peer.Directory == first.Directory {
		t.Fatal("peer workspace shares authentication lease", err)
	}
	cfg.Model = "other-writer"
	changed, err := remoteAuthenticationTransport(cfg, workspace)
	if err != nil || changed.Directory == first.Directory {
		t.Fatal("changed frozen configuration shares authentication lease", err)
	}
}

func TestRemoteServerGrantsOnlyBoundAuthenticationChild(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows launch contract")
	}
	plan, _ := socketFixture(t)
	roots := []string{"objects", "refs", "logs", "admin"}
	config := LaunchConfig{Executable: os.Args[0], Workspace: plan.Workspace, ServerDir: t.TempDir(), Sandbox: "workspace-write", WindowsSandbox: "mxc", WritableRoots: roots, AuthenticationTransport: plan}
	captured := errors.New("captured launch")
	_, _, _, err := LaunchServer(config, func(cmd *exec.Cmd) (int, error) {
		var got []string
		for _, arg := range cmd.Args {
			if raw, ok := strings.CutPrefix(arg, "sandbox_workspace_write.writable_roots="); ok {
				if err := json.Unmarshal([]byte(raw), &got); err != nil {
					t.Fatal(err)
				}
			}
		}
		want := append(append([]string(nil), roots...), plan.Directory)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("native server granted unexpected roots", got, want)
		}
		return 0, captured
	}, func(env []string) []string { return env })
	if !errors.Is(err, captured) {
		t.Fatal("bound launch not captured", err)
	}
	for _, mutation := range []string{"foreign-workspace", "parent-directory", "extra-root"} {
		t.Run(mutation, func(t *testing.T) {
			copy := config
			lease := *plan
			copy.AuthenticationTransport = &lease
			switch mutation {
			case "foreign-workspace":
				copy.Workspace += "-foreign"
			case "parent-directory":
				lease.Directory = filepath.Dir(lease.Directory)
			case "extra-root":
				copy.WritableRoots = append(append([]string(nil), roots...), plan.StateDirectory)
			}
			called := false
			_, _, _, err := LaunchServer(copy, func(*exec.Cmd) (int, error) { called = true; return 0, captured }, func(env []string) []string { return env })
			if called || err == nil {
				t.Fatal("unbound root reached process start", err)
			}
		})
	}
}

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
