package workspace

import (
	"bytes"
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

// CurrentGit samples commit and dirtiness together, twice. No cached base/head
// is a current observation. --no-optional-locks avoids refreshing the index.
func CurrentGit(ctx context.Context, git Git, repo Repository, intent *b.WorkspaceIntent) (string, bool, error) {
	if err := Verify(ctx, git, repo, intent, false); err != nil {
		return "", false, err
	}
	sample := func() ([]byte, error) {
		return git.Output(ctx, intent.Path, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all", "--ignore-submodules=none")
	}
	first, err := sample()
	if err != nil {
		return "", false, err
	}
	if err := Verify(ctx, git, repo, intent, false); err != nil {
		return "", false, err
	}
	second, err := sample()
	if err != nil {
		return "", false, err
	}
	if !bytes.Equal(first, second) {
		return "", false, errors.New("workspace changed during Git observation")
	}
	commit, clean := "", true
	headers := true
	for _, entry := range strings.Split(string(second), "\x00") {
		if entry == "" {
			continue
		}
		if headers && strings.HasPrefix(entry, "# ") {
			if strings.HasPrefix(entry, "# branch.oid ") {
				commit = strings.TrimPrefix(entry, "# branch.oid ")
			}
			continue
		}
		// A rename includes a second NUL-delimited pathname. It may look
		// like a header, but cannot replace the actual branch.oid.
		headers, clean = false, false
	}
	if !commitPattern.MatchString(commit) {
		return "", false, errors.New("current commit was not reported by Git")
	}
	// The observation ends at the second sample; no later Git query may
	// extend that interval while leaving its clean value from an older sample.
	return commit, clean, nil
}
