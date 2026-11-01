package codexworker

import (
	"errors"
	"path/filepath"
	"strings"
)

// LinkedWorktreePaths returns four verified Git identity paths.
// These paths are never emitted as native permission arguments.
func LinkedWorktreePaths(objects, refs, logs, admin string) ([]string, error) {
	if objects == "" || refs == "" || logs == "" || admin == "" {
		return nil, errors.New("linked worktree Git write paths are incomplete")
	}
	return []string{objects, refs, logs, admin}, nil
}

// PrivateLinkedWorktreePaths is only for an owner-verified, single-Run
// repository. The additional common path records
// the runtime's private-repository identity.
func PrivateLinkedWorktreePaths(objects, refs, logs, admin string) ([]string, error) {
	args, err := LinkedWorktreePaths(objects, refs, logs, admin)
	if err != nil {
		return nil, err
	}
	common := filepath.Dir(objects)
	equal := func(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
	if !filepath.IsAbs(common) || !equal(objects, filepath.Join(common, "objects")) || !equal(refs, filepath.Join(common, "refs")) || !equal(logs, filepath.Join(common, "logs")) || !equal(filepath.Dir(filepath.Dir(admin)), common) || !equal(filepath.Dir(admin), filepath.Join(common, "worktrees")) {
		return nil, errors.New("private linked Git roots do not share the verified common directory")
	}
	return append(args, common), nil
}
