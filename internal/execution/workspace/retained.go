package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// CaptureRetainedIdentity reads filesystem incarnations as well as Git paths.
// Recreating a checkout at the same path is not the same managed workspace.
func CaptureRetainedIdentity(ctx context.Context, git Git, repository Repository, intent *b.WorkspaceIntent) (*b.WorkspaceIdentity, error) {
	if err := VerifyFileSource(ctx, git, repository, intent, false); err != nil {
		return nil, err
	}
	roots, err := linkedResources(ctx, git, repository, intent.Path)
	if err != nil {
		return nil, err
	}
	directory, err := DirectoryIdentity(intent.Path)
	if err != nil {
		return nil, err
	}
	admin, err := DirectoryIdentity(roots.Admin)
	if err != nil {
		return nil, err
	}
	return &b.WorkspaceIdentity{Directory: directory, Admin: admin, AdminDirectory: roots.Admin}, nil
}

func VerifyRetained(ctx context.Context, git Git, repository Repository, intent *b.WorkspaceIntent, identity *b.WorkspaceIdentity) error {
	if identity == nil {
		return errors.New("managed workspace incarnation unavailable")
	}
	actual, err := CaptureRetainedIdentity(ctx, git, repository, intent)
	if err != nil {
		return err
	}
	if *actual != *identity {
		return errors.New("managed workspace was removed, replaced or redirected")
	}
	branch, branchErr := git.Output(ctx, intent.Path, "symbolic-ref", "--quiet", "HEAD")
	want := "refs/heads/" + intent.Branch
	if branchErr == nil && strings.TrimSpace(string(branch)) == want {
		return nil
	}
	// A rebase temporarily detaches HEAD; its original branch remains explicit.
	if branchErr != nil {
		for _, state := range []string{"rebase-merge", "rebase-apply"} {
			name, err := os.ReadFile(filepath.Join(identity.AdminDirectory, state, "head-name"))
			if err == nil && strings.TrimSpace(string(name)) == want {
				return nil
			}
		}
	}
	return errors.New("managed workspace branch changed")
}
