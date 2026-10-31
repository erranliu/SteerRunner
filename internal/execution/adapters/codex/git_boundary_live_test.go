//go:build windows && live

package codexworker

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

var hostNativeExecutable = flag.String("host-native-executable", "", "native Codex executable for explicit no-model probes")

func TestLinkedGitNativeCommit(t *testing.T) {
	if *hostNativeExecutable == "" {
		t.Fatal("explicit -host-native-executable is required")
	}
	root, err := os.MkdirTemp("", "sr-git-boundary-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("preserved Git boundary:", root)
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "worker")
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("Git: %v: %s", err, out)
		}
	}
	git("init", "-q", "-b", "main", repo)
	git("-C", repo, "-c", "user.name=Boundary", "-c", "user.email=boundary@example.invalid", "commit", "--allow-empty", "-qm", "base")
	git("-C", repo, "pack-refs", "--all")
	git("-C", repo, "worktree", "add", "-qb", "boundary", work)
	common := filepath.Join(repo, ".git")
	roots, err := PrivateLinkedWorktreeWriteArgs(filepath.Join(common, "objects"), filepath.Join(common, "refs"), filepath.Join(common, "logs"), filepath.Join(common, "worktrees", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	host := process.New(30 * time.Second)
	backend := Backend{Config: Config{Executable: *hostNativeExecutable, Workspace: work, RunDir: root, RunID: strings.Repeat("a", 32), ServerDir: filepath.Join(root, "server"), Sandbox: "danger-full-access", WindowsSandbox: "", WorkerRuntime: true, Environment: func(env []string) []string { return env }}, Process: ProcessPort{Start: host.StartManaged, Inspect: host.InspectProcess, NewGeneration: func() string { return "git-boundary" }, WriteAtomic: func(path string, data []byte) error { return os.WriteFile(path, data, 0600) }}}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	h, _, err := backend.PrepareSession(ctx, Binding{}, b.TurnInput{CommandID: strings.Repeat("b", 32), Prompt: "no business input", LinkedWorktreeArgs: roots})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		identity := h.process.Identity()
		h.process.Kill()
		h.Close()
		deadline := time.Now().Add(10 * time.Second)
		for host.InspectProcess(&identity) != "process absent" && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if host.InspectProcess(&identity) != "process absent" {
			t.Error("native Git probe process exit was not confirmed")
		}
	})
	outside := filepath.Join(root, "peer.git")
	if err = os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(outside, "config")
	if err = os.WriteFile(canary, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"loose-head", "packed-head"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "packed-head" {
				git("-C", repo, "pack-refs", "--all")
			}
			writable, _ := WritableRoots(roots)
			policy, _ := TurnStartParams(work, "probe", "probe", "probe", "danger-full-access", "", "", writable)
			var result struct {
				ExitCode int    `json:"exitCode"`
				Stdout   string `json:"stdout"`
				Stderr   string `json:"stderr"`
			}
			err := h.client.Call(ctx, "command/exec", map[string]any{"command": []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `git -c user.name=Boundary -c user.email=boundary@example.invalid commit --allow-empty -m boundary`}, "cwd": work, "sandboxPolicy": policy["sandboxPolicy"], "timeoutMs": 25000}, &result)
			t.Logf("err=%v exit=%d stdout=%s stderr=%s", err, result.ExitCode, result.Stdout, result.Stderr)
			if err != nil || result.ExitCode != 0 || strings.TrimSpace(result.Stderr) != "" {
				t.Fatal("native commit must succeed without hidden stderr errors")
			}
		})
	}
	body, readErr := os.ReadFile(canary)
	if readErr != nil || string(body) != "unchanged" {
		t.Fatal("normal commits touched the independent peer repository")
	}
	proof, _ := json.MarshalIndent(map[string]any{"loose_and_packed_head_commits": !t.Failed(), "peer_unchanged_by_commit": true, "access_mode": "full_access", "business_inputs": 0, "git_common": common}, "", "  ")
	if err = os.WriteFile(filepath.Join(root, "git-boundary-result.json"), proof, 0600); err != nil {
		t.Fatal(err)
	}
}

// Qualifies the unchanged read-only permission compiler against a real native
// command/exec without model input or account mutation.
func TestHostNativeReadOnlyCommand(t *testing.T) {
	if *hostNativeExecutable == "" {
		t.Fatal("explicit -host-native-executable is required")
	}
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	native, err := NativeExecutable(*hostNativeExecutable)
	if err != nil {
		t.Fatal(err)
	}
	host := process.New(30 * time.Second)
	if err := os.Mkdir(filepath.Join(root, "server"), 0700); err != nil {
		t.Fatal(err)
	}
	owned, _, endpoint, err := LaunchServer(LaunchConfig{Executable: native, Workspace: workspace, ServerDir: filepath.Join(root, "server"), Sandbox: "read-only", WindowsSandbox: "mxc"}, host.StartManaged, func(env []string) []string { return env })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		identity := owned.Identity()
		owned.Kill()
		owned.Close()
		deadline := time.Now().Add(5 * time.Second)
		for host.InspectProcess(&identity) != "process absent" && time.Now().Before(deadline) {
			time.Sleep(25 * time.Millisecond)
		}
		if host.InspectProcess(&identity) != "process absent" {
			t.Error("read-only native owner remains unverified")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, err := ConnectAppServer(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	params, err := TurnStartParams(workspace, "probe", "probe", "probe", "read-only", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(workspace, "must-not-exist")
	var result struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
	}
	err = client.Call(ctx, "command/exec", map[string]any{"command": []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; Get-Item -LiteralPath '.' | Out-Null; Write-Output 'SR_READ_ONLY_OK'`}, "cwd": workspace, "sandboxPolicy": params["sandboxPolicy"], "timeoutMs": 20000}, &result)
	if err != nil || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "SR_READ_ONLY_OK" {
		t.Fatal("native read-only command unavailable", err)
	}
	script := `$ErrorActionPreference='Stop'; Get-Item -LiteralPath '.' | Out-Null; [IO.File]::WriteAllText('` + strings.ReplaceAll(marker, "'", "''") + `','denied')`
	err = client.Call(ctx, "command/exec", map[string]any{"command": []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script}, "cwd": workspace, "sandboxPolicy": params["sandboxPolicy"], "timeoutMs": 20000}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode == 0 {
		t.Fatal("read-only command permitted a write")
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("read-only command wrote a file", err)
	}
	t.Log("native read-only command rejected the write; no model input")
}
