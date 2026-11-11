package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type Preparation struct {
	Status     executionbackend.OperationStatus      `json:"status"`
	Inspection *executionbackend.WorkspaceInspection `json:"inspection,omitempty"`
}

func Prepare(ctx context.Context, git Git, repository Repository, root string, intent *executionbackend.WorkspaceIntent) (Preparation, error) {
	result := Preparation{Status: executionbackend.OperationUnknown}
	if intent == nil || !filepath.IsAbs(root) || !filepath.IsAbs(intent.Path) || canonical(filepath.Dir(intent.Path)) != canonical(root) {
		result.Status = executionbackend.OperationRejected
		return result, errors.New("workspace intent does not match the validated worktree root")
	}
	if err := validateOperationRoot(root); err != nil {
		result.Status = executionbackend.OperationRejected
		return result, err
	}
	if err := repository.Verify(ctx, git); err != nil {
		result.Status = executionbackend.OperationRejected
		return result, err
	}
	if !commitPattern.MatchString(intent.BaseCommit) || intent.Branch == "" {
		result.Status = executionbackend.OperationRejected
		return result, errors.New("workspace branch or pinned base is invalid")
	}
	if _, err := git.Output(ctx, repository.Path, "check-ref-format", "refs/heads/"+intent.Branch); err != nil {
		result.Status = executionbackend.OperationRejected
		return result, err
	}
	if _, err := os.Lstat(intent.Path); !os.IsNotExist(err) {
		result.Status = executionbackend.OperationRejected
		return result, fmt.Errorf("worktree path already exists or is inaccessible: %s", intent.Path)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return result, err
	}
	refs, err := git.Output(ctx, repository.Path, "for-each-ref", "--format=%(refname)", "refs/heads/"+intent.Branch)
	if err != nil {
		return result, err
	}
	if containsExactLine(string(refs), "refs/heads/"+intent.Branch) {
		result.Status = executionbackend.OperationRejected
		return result, errors.New("worktree branch already exists")
	}
	if err = os.Mkdir(intent.Path, 0700); err != nil {
		return result, err
	}
	checkout := git
	if deadline, ok := ctx.Deadline(); ok {
		// Checkout includes LFS materialization and can use the remaining
		// preparation budget. The other Git commands keep their own limits.
		checkout.Timeout = max(time.Until(deadline), time.Nanosecond)
	}
	if _, err = checkout.Output(ctx, repository.Path, "--no-replace-objects", "-c", "core.hooksPath=/dev/null", "worktree", "add", "--no-track", "-b", intent.Branch, "--", intent.Path, intent.BaseCommit); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// The owned Git invocation has exited. It did not prove checkout
			// completion, and there is no native session to reconcile.
			result.Status = executionbackend.OperationRejected
			return result, fmt.Errorf("worktree checkout timed out: %w", err)
		}
		return result, err
	}
	result.Status = executionbackend.OperationConfirmed
	// A successful creation binds the requested revision. No worktree scan
	// took place, so this receipt only reports revision provenance.
	result.Inspection = &executionbackend.WorkspaceInspection{
		InspectionScope: "revision", Repository: repository.Path, Worktree: intent.Path,
		Branch: intent.Branch, Base: intent.BaseCommit, Head: intent.BaseCommit, CheckedAt: time.Now().UTC(),
	}
	return result, nil
}

func containsExactLine(value, expected string) bool {
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}
