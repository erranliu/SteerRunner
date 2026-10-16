package codexworker

import (
	"errors"
	"path/filepath"
	"strings"
)

// LinkedWorktreeWriteArgs compiles four previously verified, fixed Git
// directories into Codex writable-path arguments. Resource discovery belongs
// to workspace; this package only owns argv generation.
func LinkedWorktreeWriteArgs(objects, refs, logs, admin string) ([]string, error) {
	if objects == "" || refs == "" || logs == "" || admin == "" {
		return nil, errors.New("linked worktree Git write paths are incomplete")
	}
	return []string{"--add-dir", objects, "--add-dir", refs, "--add-dir", logs, "--add-dir", admin}, nil
}

// PrivateLinkedWorktreeWriteArgs is only for an owner-verified, single-Run
// repository. Git's files backend locks packed-refs even when deleting an
// absent AUTO_MERGE after a successful commit. File-only MXC grants cannot
// authorize creation of that lock; the private common directory is required.
func PrivateLinkedWorktreeWriteArgs(objects, refs, logs, admin string) ([]string, error) {
	args, err := LinkedWorktreeWriteArgs(objects, refs, logs, admin)
	if err != nil {
		return nil, err
	}
	common := filepath.Dir(objects)
	equal := func(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
	if !filepath.IsAbs(common) || !equal(objects, filepath.Join(common, "objects")) || !equal(refs, filepath.Join(common, "refs")) || !equal(logs, filepath.Join(common, "logs")) || !equal(filepath.Dir(filepath.Dir(admin)), common) || !equal(filepath.Dir(admin), filepath.Join(common, "worktrees")) {
		return nil, errors.New("private linked Git roots do not share the verified common directory")
	}
	return append(args, "--add-dir", common), nil
}

// LocalSandboxArgs fixes the native Windows implementation without expanding
// the turn's workspace-write roots or initiating administrator setup.
