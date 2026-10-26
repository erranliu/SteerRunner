package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// VerifyFileSource checks the same original repository/worktree facts as
// Verify(initial=false), batching rev-parse outputs to fit the existing finite
// file-preparation window. It neither prepares a workspace nor permits new roots.
func VerifyFileSource(ctx context.Context, git Git, repository Repository, intent *b.WorkspaceIntent) error {
	if intent == nil || !filepath.IsAbs(intent.Path) || intent.Branch == "" || !commitPattern.MatchString(intent.BaseCommit) || !filepath.IsAbs(repository.Path) || !filepath.IsAbs(repository.CommonDir) {
		return errors.New("incomplete workspace intent")
	}
	real, err := filepath.EvalSymlinks(intent.Path)
	if err != nil || canonical(real) != canonical(intent.Path) {
		return errors.New("worktree path was redirected")
	}
	for _, check := range []struct{ directory, root, branch string }{
		{repository.Path, repository.Path, ""},
		{intent.Path, intent.Path, "refs/heads/" + intent.Branch},
	} {
		args := []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir"}
		if check.branch != "" {
			args = append(args, "--symbolic-full-name", "HEAD")
		}
		raw, err := git.Output(ctx, check.directory, args...)
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		expected := 2
		if check.branch != "" {
			expected++
		}
		if len(lines) != expected {
			return errors.New("original workspace identity response is incomplete")
		}
		for i, want := range []string{check.root, repository.CommonDir} {
			got, err := resolveGitPath(strings.TrimSpace(lines[i]))
			if err != nil {
				return err
			}
			if canonical(got) != canonical(want) {
				return errors.New("original repository or worktree differs from its frozen identity")
			}
		}
		if check.branch != "" && strings.TrimSpace(lines[2]) != check.branch {
			return errors.New("worktree branch differs from its frozen identity")
		}
	}
	// Match Verify's directory redirection check even if it changed during Git IO.
	if info, err := os.Stat(intent.Path); err != nil || !info.IsDir() {
		return errors.New("original worktree unavailable")
	}
	return nil
}
