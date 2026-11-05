package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
	return workspace.ValidateFixedAuditQuery(original, q.Intent, q.PreHead)
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
	out.Inspection, out.WritersAbsent, err = workspace.InspectFixedAudit(ctx, process.New(30*time.Second), setup.Workspace, config.RepositoryURL, "origin", q.Intent, q.PreHead)
	return out, err
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
