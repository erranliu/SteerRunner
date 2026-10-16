package codexworker

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGitPermissionLinkedWorktreeWriteArgsCompilesOnlyResolvedDirectories(t *testing.T) {
	want := []string{"--add-dir", "repo/objects", "--add-dir", "repo/refs", "--add-dir", "repo/logs", "--add-dir", "repo/worktrees/run-one"}
	got, err := LinkedWorktreeWriteArgs("repo/objects", "repo/refs", "repo/logs", "repo/worktrees/run-one")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%q err=%v", got, err)
	}
	if _, err := LinkedWorktreeWriteArgs("repo/objects", "repo/refs", "repo/logs", ""); err == nil {
		t.Fatal("incomplete write resource list was accepted")
	}
}

func TestPrivateGitRootsRejectForeignCommonDirectory(t *testing.T) {
	common := filepath.Join(t.TempDir(), ".git")
	objects, refs, logs, admin := filepath.Join(common, "objects"), filepath.Join(common, "refs"), filepath.Join(common, "logs"), filepath.Join(common, "worktrees", "worker")
	args, err := PrivateLinkedWorktreeWriteArgs(objects, refs, logs, admin)
	if err != nil || len(args) != 10 || args[9] != common {
		t.Fatalf("args=%v err=%v", args, err)
	}
	if _, err = PrivateLinkedWorktreeWriteArgs(objects, refs, logs, filepath.Join(t.TempDir(), "worktrees", "other")); err == nil {
		t.Fatal("foreign admin accepted")
	}
	if _, err = PrivateLinkedWorktreeWriteArgs(objects, filepath.Join(t.TempDir(), "refs"), logs, admin); err == nil {
		t.Fatal("foreign refs accepted")
	}
}
