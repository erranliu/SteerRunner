//go:build windows

package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestPrepareLFSWorktreeStaysCleanWithoutRunningHooks(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Fatal("this Windows Git LFS regression requires git-lfs:", err)
	}
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "0")
	ctx := context.Background()
	git := Git{Command: executionbackend.LaunchCommand{Executable: "git"}, Timeout: 30 * time.Second, Start: func(cmd *exec.Cmd) (executionbackend.StopHandle, error) {
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return testProcess{cmd.Process}, nil
	}}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	prepareGitFixture(t, git, source)
	call := func(dir string, args ...string) string {
		t.Helper()
		out, err := git.Output(ctx, dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	call(source, "lfs", "install", "--local")
	const contents = "local LFS worktree regression\n"
	for name, body := range map[string]string{".gitattributes": "*.bin filter=lfs diff=lfs merge=lfs -text\n", "sample.bin": contents} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	call(source, "add", ".gitattributes", "sample.bin")
	call(source, "-c", "core.hooksPath="+os.DevNull, "commit", "-m", "LFS fixture")
	if pointer := call(source, "show", "HEAD:sample.bin"); !strings.HasPrefix(pointer, "version https://git-lfs.github.com/spec/v1") {
		t.Fatal("fixture did not store an LFS pointer:", pointer)
	}
	repository, err := ResolveRepository(ctx, git, source)
	if err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(repository.CommonDir, "hooks", "post-checkout")
	hook := []byte("#!/bin/sh\nprintf 'unexpected hook' > hook-ran.txt\n")
	if err := os.WriteFile(hookPath, hook, 0700); err != nil {
		t.Fatal(err)
	}
	worktrees := filepath.Join(root, "worktrees")
	intent := &executionbackend.WorkspaceIntent{Path: filepath.Join(worktrees, "lfs-run"), Branch: "codex/lfs", BaseCommit: call(source, "rev-parse", "HEAD")}
	result, err := Prepare(ctx, git, repository, worktrees, intent)
	if err != nil || result.Status != executionbackend.OperationConfirmed {
		t.Fatalf("prepare: %+v, %v", result, err)
	}
	if body, err := os.ReadFile(filepath.Join(intent.Path, "sample.bin")); err != nil || string(body) != contents {
		t.Fatalf("LFS content was not materialized: %q, %v", body, err)
	}
	if dirty := call(intent.Path, "status", "--porcelain"); dirty != "" {
		t.Fatal("new LFS worktree is dirty:", dirty)
	}
	if head := call(intent.Path, "rev-parse", "HEAD"); head != intent.BaseCommit {
		t.Fatal("checkout changed the pinned revision:", head)
	}
	if body, err := os.ReadFile(hookPath); err != nil || string(body) != string(hook) {
		t.Fatalf("source hook changed: %q, %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(intent.Path, "dev")); !os.IsNotExist(err) {
		t.Fatal("Unix device path polluted worktree:", err)
	}
	if _, err := os.Lstat(filepath.Join(intent.Path, "hook-ran.txt")); !os.IsNotExist(err) {
		t.Fatal("source checkout hook executed:", err)
	}
}
