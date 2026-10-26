package workspace

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResultFileWorkspaceProofRetainsOriginalChecksWithBoundedCalls(t *testing.T) {
	calls := 0
	git := Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 15 * time.Second, Start: func(cmd *exec.Cmd) (b.StopHandle, error) {
		calls++
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return testProcess{cmd.Process}, nil
	}}
	ctx := context.Background()
	root := t.TempDir()
	original, foreign := filepath.Join(root, "original"), filepath.Join(root, "foreign")
	prepareGitFixture(t, git, original)
	prepareGitFixture(t, git, foreign)
	repository, err := ResolveRepository(ctx, git, original)
	if err != nil {
		t.Fatal(err)
	}
	head, err := git.Output(ctx, original, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	branch, err := git.Output(ctx, original, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	intent := b.WorkspaceIntent{Path: repository.Path, Branch: strings.TrimSpace(string(branch)), BaseCommit: strings.TrimSpace(string(head))}
	calls = 0
	if err := VerifyFileSource(ctx, git, repository, &intent); err != nil || calls > 2 {
		t.Fatal("original proof exceeded bounded Git calls", err, calls)
	}
	for _, name := range []string{"valid", "wrong-branch", "foreign-worktree", "wrong-common-directory"} {
		repo, candidate := repository, intent
		switch name {
		case "wrong-branch":
			candidate.Branch = "foreign-branch"
		case "foreign-worktree":
			candidate.Path = foreign
		case "wrong-common-directory":
			repo.CommonDir = filepath.Join(foreign, ".git")
		}
		oldErr := Verify(ctx, git, repo, &candidate, false)
		newErr := VerifyFileSource(ctx, git, repo, &candidate)
		if (oldErr == nil) != (newErr == nil) || (newErr == nil) != (name == "valid") {
			t.Fatal("workspace boundary changed", name, oldErr, newErr)
		}
	}
	if _, err := git.Output(ctx, original, "checkout", "--detach", "--quiet", intent.BaseCommit); err != nil {
		t.Fatal(err)
	}
	if VerifyFileSource(ctx, git, repository, &intent) == nil || Verify(ctx, git, repository, &intent, false) == nil {
		t.Fatal("detached worktree accepted")
	}
}
