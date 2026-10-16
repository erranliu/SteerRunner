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

func TestCloneIsolatedTransfersObjectsWithoutRuntimeFilesAndKeepsFrozenBase(t *testing.T) {
	ctx := context.Background()
	git := Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 15 * time.Second, Start: func(cmd *exec.Cmd) (b.StopHandle, error) {
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return testProcess{cmd.Process}, nil
	}}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "clone")
	prepareGitFixture(t, git, source)
	initial, _ := git.Output(ctx, source, "rev-parse", "HEAD")
	for _, args := range [][]string{{"commit", "--allow-empty", "-q", "-m", "frozen remote-tracking base"}} {
		if _, err := git.Output(ctx, source, args...); err != nil {
			t.Fatal(err)
		}
	}
	frozen, err := git.Output(ctx, source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(frozen))
	if _, err = git.Output(ctx, source, "update-ref", "refs/remotes/origin/frozen", base); err != nil {
		t.Fatal(err)
	}
	if _, err = git.Output(ctx, source, "reset", "--hard", strings.TrimSpace(string(initial))); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(".git", "objects", ".sr-auth-runtime-canary")
	if err = os.WriteFile(filepath.Join(source, marker), []byte("not a Git object"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = CloneIsolated(ctx, git, root, source, destination, "origin", base); err != nil {
		t.Fatal(err)
	}
	if _, err = git.Output(ctx, destination, "cat-file", "-e", base+"^{commit}"); err != nil {
		t.Fatal("frozen remote-tracking commit was lost", err)
	}
	if _, err = os.Stat(filepath.Join(destination, marker)); !os.IsNotExist(err) {
		t.Fatal("source runtime artifact copied into isolated check", err)
	}
}
