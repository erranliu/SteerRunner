package workspace

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

// InspectAudit uses the existing fixed Git owner. It does not modify the
// index, normalize unsupported worktrees or create a snapshot service.
func InspectAudit(ctx context.Context, git Git, repo Repository, intent *b.WorkspaceIntent, preHead string) (b.WorkspaceInspection, error) {
	fact, err := Inspect(ctx, git, repo, intent)
	if err != nil {
		return fact, err
	}
	sparse, err := git.Output(ctx, intent.Path, "config", "--bool", "--default=false", "--get", "core.sparseCheckout")
	if err != nil {
		return fact, err
	}
	if strings.TrimSpace(string(sparse)) != "false" {
		return fact, errors.New("sparse worktree is not qualified for audited provenance")
	}
	index, err := git.Output(ctx, intent.Path, "ls-files", "-v", "-z")
	if err != nil {
		return fact, err
	}
	for _, entry := range strings.Split(string(index), "\x00") {
		if entry != "" && (entry[0] == 'S' || entry[0] >= 'a' && entry[0] <= 'z') {
			return fact, errors.New("hidden index flags are not qualified for audited provenance")
		}
	}
	stages, err := git.Output(ctx, intent.Path, "ls-files", "--stage", "-z")
	if err != nil {
		return fact, err
	}
	for _, entry := range strings.Split(string(stages), "\x00") {
		if strings.HasPrefix(entry, "160000 ") {
			return fact, errors.New("gitlinks/submodules are not qualified for audited provenance")
		}
	}
	tree, err := git.Output(ctx, intent.Path, "rev-parse", fact.Head+"^{tree}")
	if err != nil {
		return fact, err
	}
	fact.Tree = strings.TrimSpace(string(tree))
	if preHead != "" {
		if _, err := git.Output(ctx, intent.Path, "merge-base", "--is-ancestor", preHead, fact.Head); err != nil {
			return fact, errors.New("audit revision is not a descendant of the frozen implementation")
		}
	}
	if err := Verify(ctx, git, repo, intent, false); err != nil {
		return fact, err
	}
	head, err := git.Output(ctx, intent.Path, "rev-parse", "HEAD")
	if err != nil {
		return fact, err
	}
	if strings.TrimSpace(string(head)) != fact.Head {
		return fact, errors.New("HEAD changed during workspace inspection")
	}
	if fact.Dirty {
		return fact, errors.New("audited completion requires a clean committed worktree")
	}
	fact.Qualified = true
	return fact, nil
}
