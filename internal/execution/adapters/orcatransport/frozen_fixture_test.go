package orcatransport

import (
	"encoding/json"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Only test code can encode an old terminal state; production retains decoding.
func (r *frozenRecord) envelope(s b.LifecycleSnapshot, state State) ([]b.ImplementationDataEnvelope, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: r.stateVersion(), Purpose: b.ImplementationResourcePurpose, ImplementationKey: StateKey, ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}, raw)
	if err != nil {
		return nil, err
	}
	return []b.ImplementationDataEnvelope{e}, nil
}

// Frozen facts committed by the original workspace preparation, without startup.
func frozenWorkspaceFixture(t *testing.T, l *frozenRecord, f *fakeHost, s b.LifecycleSnapshot) b.LifecycleSnapshot {
	t.Helper()
	path := f.root
	if f.nativePath != "" {
		path = f.nativePath
	}
	p := b.ProcessIdentity{Kind: "orca_terminal", Executable: l.profile.Executable, RuntimeID: "runtime", Terminal: "terminal", Incarnation: f.incarnation, WorktreeID: "repo::" + path, ExecutionHost: l.profile.Target().ExecutionHost}
	s.Ref.External = p
	s.Workspace = b.WorkspaceIntent{Path: path, Branch: "worker", BaseCommit: s.WorkspacePlan.BaseCommit}
	s.Ref.WorkspacePath, s.Ref.WorkspaceBranch = path, "worker"
	s.Session.SessionID, s.Session.Generation = SessionID(p), p.Incarnation
	f.workspace = map[string]any{"id": p.WorktreeID, "instanceId": "original-workspace", "repoId": "repo", "path": path, "branch": "worker", "head": s.WorkspacePlan.BaseCommit, "baseRef": s.WorkspacePlan.BaseCommit, "displayName": workspaceName(s)}
	var err error
	s.Implementation, err = l.envelope(s, State{Process: p})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
