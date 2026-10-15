package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
