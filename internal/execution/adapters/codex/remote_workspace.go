package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

type RemoteWorkspaceAuditQuery struct {
	Intent  b.WorkspaceIntent `json:"intent"`
	PreHead string            `json:"pre_head,omitempty"`
}
type RemoteWorkspaceAudit struct {
	Readiness     RemoteReadiness       `json:"readiness"`
	Inspection    b.WorkspaceInspection `json:"inspection"`
	WritersAbsent bool                  `json:"writers_absent"`
}

func (q RemoteWorkspaceAuditQuery) Validate(original string) error {
	commit := func(s string) bool { return len(s) == 40 && strings.Trim(s, "0123456789abcdef") == "" }
	if !sameWorkspace(q.Intent.Path, original) || !filepath.IsAbs(q.Intent.Path) || q.Intent.Branch == "" || strings.HasPrefix(q.Intent.Branch, "-") || strings.ContainsAny(q.Intent.Branch, "\x00\r\n") || !commit(q.Intent.BaseCommit) || q.PreHead != "" && !commit(q.PreHead) {
		return errors.New("original_workspace_query_invalid")
	}
	return nil
}

type auditGitHandle struct {
	b.StopHandle
	settled *bool
}

func (h *auditGitHandle) Close() {
	if !settleRemoteTree(h.StopHandle) {
		*h.settled = false
	}
	h.StopHandle.Close()
}

// Only fixed Git reads in the original workspace. No model, credentials,
// arbitrary argv, repository mutation or new Orca terminal is involved.
func InspectRemoteWorkspaceAudit(ctx context.Context, setup RemoteControlSetup, session string, q RemoteWorkspaceAuditQuery) (out RemoteWorkspaceAudit, result error) {
	out.WritersAbsent = true
	if q.Validate(setup.Workspace) != nil || !ValidNativeID(session) {
		return out, errors.New("original_workspace_query_invalid")
	}
	dir := RemoteRuntimeDirectory(setup.Workspace)
	raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	if err != nil || b.DecodeStrictJSON(raw, &out.Readiness, "trailing workspace readiness") != nil || !out.Readiness.Verified || out.Readiness.NativeTransport != RemoteNativeTransport || !SupportedDevelopmentPolicy(out.Readiness.Contract) || out.Readiness.SessionID != session || !sameWorkspace(out.Readiness.Workspace, setup.Workspace) || out.Readiness.RuntimeSHA256 != setup.RuntimeSHA256 {
		return out, errors.New("original_workspace_preparation_mismatch")
	}
	sha, err := RuntimeBinarySHA256()
	if err != nil || sha != out.Readiness.RuntimeSHA256 || process.Probe(&out.Readiness.Process).State != "matches" {
		return out, errors.New("original_workspace_owner_unverified")
	}
	var config RemoteAgentConfig
	raw, err = os.ReadFile(filepath.Join(dir, "configuration.json"))
	if err != nil || b.DecodeStrictJSON(raw, &config, "trailing workspace configuration") != nil || RemoteConfigurationDigest(config) != out.Readiness.ConfigurationDigest || config.RepositoryURL != out.Readiness.Repository {
		return out, errors.New("original_workspace_configuration_mismatch")
	}
	out.Inspection, out.WritersAbsent, err = InspectOwnedWorkspaceAudit(ctx, process.New(30*time.Second), setup.Workspace, config.RepositoryURL, "origin", q)
	return out, err
}

// InspectOwnedWorkspaceAudit runs the fixed Git reads after the adapter has
// authenticated the original workspace. Only its own reader processes settle here.
func InspectOwnedWorkspaceAudit(ctx context.Context, host b.ManagedProcessHost, original, repositoryURL, remoteName string, q RemoteWorkspaceAuditQuery) (fact b.WorkspaceInspection, writersAbsent bool, result error) {
	if host == nil || repositoryURL == "" || remoteName == "" || q.Validate(original) != nil {
		return fact, false, errors.New("original_workspace_query_invalid")
	}
	writersAbsent = true
	git := workspace.Git{Command: b.LaunchCommand{Executable: "git", Args: []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}}, Timeout: 30 * time.Second, Start: func(cmd *exec.Cmd) (b.StopHandle, error) {
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
	repo, err := workspace.ResolveRepository(ctx, git, original)
	if err != nil {
		return fact, writersAbsent, err
	}
	remote, err := git.Output(ctx, original, "config", "--get", "remote."+remoteName+".url")
	if err != nil || strings.TrimSpace(string(remote)) != repositoryURL {
		return fact, writersAbsent, errors.New("original_workspace_repository_mismatch")
	}
	fact, err = workspace.InspectAudit(ctx, git, repo, &q.Intent, q.PreHead)
	return fact, writersAbsent, err
}

func ValidateRemoteWorkspaceAudit(ready RemoteReadiness, q RemoteWorkspaceAuditQuery, proof RemoteWorkspaceAudit) error {
	a, _ := json.Marshal(ready)
	z, _ := json.Marshal(proof.Readiness)
	f := proof.Inspection
	if string(a) != string(z) || !proof.WritersAbsent || !f.Qualified || f.Dirty || f.InspectionScope != "working_tree" || !sameWorkspace(f.Repository, ready.Workspace) || !sameWorkspace(f.Worktree, q.Intent.Path) || f.Branch != q.Intent.Branch || f.Base != q.Intent.BaseCommit || len(f.Head) != 40 || strings.Trim(f.Head, "0123456789abcdef") != "" || len(f.Tree) != 40 || strings.Trim(f.Tree, "0123456789abcdef") != "" || f.CheckedAt.IsZero() {
		return errors.New("original_workspace_evidence_unverified")
	}
	return nil
}
