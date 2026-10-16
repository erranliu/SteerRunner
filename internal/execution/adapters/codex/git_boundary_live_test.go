//go:build windows && live

package codexworker

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestLinkedGitNativeCommit(t *testing.T) {
	if *authenticationNative == "" {
		t.Fatal("explicit -sandbox-auth-native is required")
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
	backend := Backend{Config: Config{Executable: *authenticationNative, Workspace: work, RunDir: root, RunID: strings.Repeat("a", 32), ServerDir: filepath.Join(root, "server"), Sandbox: "workspace-write", WindowsSandbox: "mxc", WorkerRuntime: true, Environment: func(env []string) []string { return env }}, Process: ProcessPort{Start: host.StartManaged, Inspect: host.InspectProcess, NewGeneration: func() string { return "git-boundary" }, WriteAtomic: func(path string, data []byte) error { return os.WriteFile(path, data, 0600) }}}
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
	for _, mode := range []string{"loose-head", "packed-head"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "packed-head" {
				git("-C", repo, "pack-refs", "--all")
			}
			writable, _ := WritableRoots(roots)
			policy, _ := TurnStartParams(work, "probe", "probe", "probe", "workspace-write", "", "", writable)
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
	outside := filepath.Join(root, "peer.git")
	if err = os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(outside, "config")
	if err = os.WriteFile(canary, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	writable, _ := WritableRoots(roots)
	policy, _ := TurnStartParams(work, "probe", "probe", "probe", "workspace-write", "", "", writable)
	var denied struct {
		ExitCode int `json:"exitCode"`
	}
	err = h.client.Call(ctx, "command/exec", map[string]any{"command": []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `[IO.File]::WriteAllText(` + psLiteral(canary) + `,'changed')`}, "cwd": work, "sandboxPolicy": policy["sandboxPolicy"], "timeoutMs": 25000}, &denied)
	body, readErr := os.ReadFile(canary)
	if err != nil || denied.ExitCode == 0 || readErr != nil || string(body) != "unchanged" {
		t.Fatal("private Git authorization escaped into another repository")
	}
	proof, _ := json.MarshalIndent(map[string]any{"loose_and_packed_head_commits": !t.Failed(), "peer_write_denied": true, "business_inputs": 0, "git_common": common}, "", "  ")
	if err = os.WriteFile(filepath.Join(root, "git-boundary-result.json"), proof, 0600); err != nil {
		t.Fatal(err)
	}
}
