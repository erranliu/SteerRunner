package orcatransport

import (
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Historical reader fixtures start from saved facts, never a second legacy
// executor. Its Lifecycle methods accept only the current structured protocol.
func frozenFixtureLifecycle(setup b.LifecycleSetup) (*Lifecycle, error) {
	r, err := decodeFrozenRecord(setup)
	if err != nil {
		return nil, err
	}
	l := &Lifecycle{setup: r.setup, profile: r.profile, profileVersion: r.profileVersion, agentDevelopment: r.agentDevelopment, agentHostID: r.agentHostID, probeProcess: r.probeProcess, clock: func() time.Time { return time.Now().UTC() }}
	l.client = native.New(native.Config{CommandTimeout: "40s", Output: orcaCLIOutput(r.setup, r.profile, r.agentHostID, func() bool { return false })})
	return l, nil
}

// Frozen facts committed by the original workspace preparation, without startup.
func frozenWorkspaceFixture(t *testing.T, l *Lifecycle, f *fakeHost, s b.LifecycleSnapshot) b.LifecycleSnapshot {
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
