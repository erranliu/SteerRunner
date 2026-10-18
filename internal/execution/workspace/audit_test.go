package workspace

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRAWorkspaceSource(t *testing.T) {
	ctx := context.Background()
	g := Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 15 * time.Second, Start: func(cmd *exec.Cmd) (b.StopHandle, error) {
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return testProcess{cmd.Process}, nil
	}}
	path := filepath.Join(t.TempDir(), "repo")
	prepareGitFixture(t, g, path)
	repo, err := ResolveRepository(ctx, g, path)
	if err != nil {
		t.Fatal(err)
	}
	call := func(args ...string) string {
		t.Helper()
		out, err := g.Output(ctx, path, args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	head := call("rev-parse", "HEAD")
	intent := &b.WorkspaceIntent{Path: path, Branch: "main", BaseCommit: head}
	before, err := InspectAudit(ctx, g, repo, intent, "")
	if err != nil || !before.Qualified || before.Tree == "" {
		t.Fatal(before, err)
	}
	if err := os.WriteFile(filepath.Join(path, "base.txt"), []byte("repair\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if f, err := InspectAudit(ctx, g, repo, intent, head); err == nil || !f.Dirty || f.Qualified {
		t.Fatal("dirty checkpoint accepted", f, err)
	}
	call("add", "base.txt")
	call("commit", "-m", "repair")
	after, err := InspectAudit(ctx, g, repo, intent, head)
	if err != nil || !after.Qualified || after.Tree == before.Tree || after.Head == head {
		t.Fatal(after, err)
	}
	for _, flag := range []string{"--assume-unchanged", "--skip-worktree"} {
		call("update-index", flag, "base.txt")
		if f, err := InspectAudit(ctx, g, repo, intent, head); err == nil || f.Qualified {
			t.Fatal("hidden index qualified", flag, f)
		}
		call("update-index", "--no-assume-unchanged", "--no-skip-worktree", "base.txt")
	}
	call("config", "core.sparseCheckout", "true")
	if f, err := InspectAudit(ctx, g, repo, intent, head); err == nil || f.Qualified {
		t.Fatal("sparse checkout qualified", f)
	}
	call("config", "core.sparseCheckout", "false")
	call("update-index", "--add", "--cacheinfo", "160000,"+head+",nested")
	if f, err := InspectAudit(ctx, g, repo, intent, head); err == nil || f.Qualified {
		t.Fatal("gitlink qualified", f)
	}
}
