package workspace

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// ValidateFixedAuditQuery checks the input against the path supplied by its owner.
func ValidateFixedAuditQuery(original string, intent b.WorkspaceIntent, preHead string) error {
	commit := func(s string) bool { return len(s) == 40 && strings.Trim(s, "0123456789abcdef") == "" }
	if !SamePath(intent.Path, original) || !filepath.IsAbs(intent.Path) || intent.Branch == "" || strings.HasPrefix(intent.Branch, "-") || strings.ContainsAny(intent.Branch, "\x00\r\n") || !commit(intent.BaseCommit) || preHead != "" && !commit(preHead) {
		return errors.New("original_workspace_query_invalid")
	}
	return nil
}

// InspectFixedAudit only reads an already authenticated workspace. The
// settlement result applies to Git readers opened by this call.
func InspectFixedAudit(ctx context.Context, host b.ManagedProcessHost, original, repositoryURL, remoteName string, intent b.WorkspaceIntent, preHead string) (fact b.WorkspaceInspection, writersAbsent bool, result error) {
	if host == nil || repositoryURL == "" || remoteName == "" || ValidateFixedAuditQuery(original, intent, preHead) != nil {
		return fact, false, errors.New("original_workspace_query_invalid")
	}
	writersAbsent = true
	git := Git{Command: b.LaunchCommand{Executable: "git", Args: []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}}, Timeout: 30 * time.Second, Start: func(cmd *exec.Cmd) (b.StopHandle, error) {
		h, err := host.StartManaged(cmd)
		if err != nil {
			return nil, err
		}
		return &auditGitHandle{StopHandle: h, settled: &writersAbsent}, nil
	}}
	defer func() {
		if !writersAbsent {
			fact.Qualified = false
			result = errors.Join(result, errors.New("original_workspace_reader_tree_unverified"))
		}
	}()
	repo, err := ResolveRepository(ctx, git, original)
	if err != nil {
		return fact, writersAbsent, err
	}
	remote, err := git.Output(ctx, original, "config", "--get", "remote."+remoteName+".url")
	if err != nil || strings.TrimSpace(string(remote)) != repositoryURL {
		return fact, writersAbsent, errors.New("original_workspace_repository_mismatch")
	}
	fact, err = InspectAudit(ctx, git, repo, &intent, preHead)
	return fact, writersAbsent, err
}

type auditGitHandle struct {
	b.StopHandle
	settled *bool
}

func (h *auditGitHandle) Close() {
	if !settleAuditGitTree(h.StopHandle) {
		*h.settled = false
	}
	h.StopHandle.Close()
}

// Check the original tree and process identity before releasing the handle.
func settleAuditGitTree(handle b.StopHandle) bool {
	if handle == nil {
		return true
	}
	handle.Kill()
	tree, ok := handle.(b.ProcessTreeHandle)
	if !ok {
		return false
	}
	until := time.Now().Add(3 * time.Second)
	identity := handle.Identity()
	absent := func() bool { return tree.WritersAbsent() && process.Probe(&identity).State == "absent" }
	for !absent() && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	return absent()
}
