package workspace

import (
	"context"
	"fmt"
)

// CloneIsolated transfers Git objects instead of copying the source object
// directory. Live credential sockets and other runtime files are not objects.
// A frozen commit may exist only under a remote-tracking ref in the source;
// fetch that exact object when the normal advertised heads did not include it.
func CloneIsolated(ctx context.Context, git Git, directory, source, destination, remote, base string) error {
	if _, err := git.Output(ctx, directory, "clone", "--no-checkout", "--no-local", "--no-hardlinks", "--origin", remote, "--", source, destination); err != nil {
		return fmt.Errorf("clone_snapshot_failed: %w", err)
	}
	if _, err := git.Output(ctx, destination, "cat-file", "-e", base+"^{commit}"); err == nil {
		return nil
	}
	if _, err := git.Output(ctx, destination, "fetch", "--no-tags", "--", source, base); err != nil {
		return fmt.Errorf("clone_base_fetch_failed: %w", err)
	}
	return nil
}
