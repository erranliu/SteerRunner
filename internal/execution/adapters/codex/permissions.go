package codexworker

import (
	"errors"
	"path/filepath"
	"strings"
)

// LinkedWorktreeWriteArgs preserves the frozen encoding of four verified Git
// paths used by runtime/authentication identity checks. New full_access launches
// never emit these legacy arguments as native write grants.
func LinkedWorktreeWriteArgs(objects, refs, logs, admin string) ([]string, error) {
	if objects == "" || refs == "" || logs == "" || admin == "" {
		return nil, errors.New("linked worktree Git write paths are incomplete")
	}
	return []string{"--add-dir", objects, "--add-dir", refs, "--add-dir", logs, "--add-dir", admin}, nil
}

// PrivateLinkedWorktreeWriteArgs is only for an owner-verified, single-Run
// repository. The additional common path retains legacy frozen data and the
// runtime's private-repository identity check; it is no longer a native grant.
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
