package codexworker

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGitPermissionLinkedWorktreePathsCompilesOnlyResolvedDirectories(t *testing.T) {
	want := []string{"repo/objects", "repo/refs", "repo/logs", "repo/worktrees/run-one"}
	got, err := LinkedWorktreePaths("repo/objects", "repo/refs", "repo/logs", "repo/worktrees/run-one")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%q err=%v", got, err)
	}
	if _, err := LinkedWorktreePaths("repo/objects", "repo/refs", "repo/logs", ""); err == nil {
		t.Fatal("incomplete write resource list was accepted")
	}
}

func TestPrivateGitRootsRejectForeignCommonDirectory(t *testing.T) {
	common := filepath.Join(t.TempDir(), ".git")
	objects, refs, logs, admin := filepath.Join(common, "objects"), filepath.Join(common, "refs"), filepath.Join(common, "logs"), filepath.Join(common, "worktrees", "worker")
	args, err := PrivateLinkedWorktreePaths(objects, refs, logs, admin)
	if err != nil || len(args) != 5 || args[4] != common {
		t.Fatalf("args=%v err=%v", args, err)
	}
	if _, err = PrivateLinkedWorktreePaths(objects, refs, logs, filepath.Join(t.TempDir(), "worktrees", "other")); err == nil {
		t.Fatal("foreign admin accepted")
	}
	if _, err = PrivateLinkedWorktreePaths(objects, filepath.Join(t.TempDir(), "refs"), logs, admin); err == nil {
		t.Fatal("foreign refs accepted")
	}
}
