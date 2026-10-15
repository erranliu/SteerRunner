package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRRepositoryCacheWithLongBranchPath(t *testing.T) {
	root := t.TempDir()
	git := gitHost()
	ctx := context.Background()
	source := filepath.Join(root, "source")
	branch := "codex/debug/" + strings.Repeat("dedicated-base-", 5) + "/base"
	if _, err := git.Output(ctx, root, "init", "--quiet", "-b", branch, source); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "spec.md"), []byte("fixed material"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "fixed input"}} {
		if _, err := git.Output(ctx, source, args...); err != nil {
			t.Fatal(err)
		}
	}
	parent := filepath.Join(root, strings.Repeat("nested-", 10), strings.Repeat("r", 64))
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(parent, ".clone-1234567890")
	if len(filepath.Join(cache, ".git", "refs", "heads", filepath.FromSlash(branch))) <= 260 {
		t.Fatal("fixture does not exercise a long ref path")
	}
	if _, err := git.Output(ctx, root, "clone", "--no-checkout", "--no-hardlinks", "--origin", "origin", "--", source, cache); err != nil {
		t.Fatal(err)
	}
	if raw, err := git.Output(ctx, cache, "symbolic-ref", "HEAD"); err != nil || strings.TrimSpace(string(raw)) != "refs/heads/"+branch {
		t.Fatal(string(raw), err)
	}
	config, err := os.ReadFile(filepath.Join(cache, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(config)), "longpaths") {
		t.Fatal("scoped Git setting was persisted")
	}
}

func TestSRCompactCacheCanPublishLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	git := gitHost()
	ctx := context.Background()
	source, remote := filepath.Join(root, "source"), filepath.Join(root, "remote.git")
	for _, args := range [][]string{{"init", "--quiet", "-b", "main", source}, {"init", "--quiet", "--bare", remote}} {
		if _, err := git.Output(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "spec.md"), []byte("fixed"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base"}} {
		if _, err := git.Output(ctx, source, args...); err != nil {
			t.Fatal(err)
		}
	}
	head, err := git.Output(ctx, source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	dir := root
	for len(dir) < 110 {
		dir = filepath.Join(dir, "project-state")
	}
	input := contracts.Repository{URL: "https://example.invalid/owned/repo.git", BaseRevision: strings.TrimSpace(string(head)), LocalPath: source}
	cache := repositoryCache(dir, input)
	if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Output(ctx, root, "clone", "--no-checkout", "--no-hardlinks", "--", source, cache); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("b", 32)
	worktree := filepath.Join(dir, "workspaces", id)
	if _, err := git.Output(ctx, cache, "worktree", "add", "-b", "codex/runner-"+id, worktree, input.BaseRevision); err != nil {
		t.Fatal(err)
	}
	admin, err := git.Output(ctx, worktree, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(admin))) >= 220 {
		t.Fatal("cache layout regressed to deep native Git administration paths", string(admin))
	}
	if _, err := git.Output(ctx, worktree, "push", remote, "HEAD:refs/heads/codex/runner-"+id); err != nil {
		t.Fatal(err)
	}
	actual, err := git.Output(ctx, root, "--git-dir", remote, "rev-parse", "refs/heads/codex/runner-"+id)
	if err != nil || string(actual) != string(head) {
		t.Fatal("published head differs from the fixed worktree", string(actual), err)
	}
	other := input
	other.LocalPath += "-other"
	if repositoryCache(dir, other) == cache {
		t.Fatal("cache key lost fixed source identity")
	}
}
