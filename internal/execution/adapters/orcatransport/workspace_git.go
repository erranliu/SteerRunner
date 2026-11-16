package orcatransport

import (
	"context"
	"errors"
	"strings"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// ObservePairedWorkspaceGit uses the frozen managed incarnation, including
// after a Run has stopped. It has no dependency on the original agent session.
func ObservePairedWorkspaceGit(ctx context.Context, setup b.LifecycleSetup, expectedIdentity string) (string, bool, error) {
	r, err := decodeFrozenRecord(setup)
	if err != nil {
		return "", false, err
	}
	s := r.setup.Snapshot
	state, err := r.agentState(s)
	if err != nil {
		return "", false, err
	}
	if !r.structured() || r.profile.Environment == "" || expectedIdentity == "" ||
		managedOrcaWorkspaceIdentity(r, state) != expectedIdentity || s.Ref.Target.WorktreeID != state.Worktree.ID ||
		s.Workspace.Branch == "" || strings.TrimPrefix(state.Worktree.Branch, "refs/heads/") != s.Workspace.Branch {
		return "", false, errors.New("original paired workspace binding is unavailable")
	}
	return native.PairedWorkspaceGit(ctx, r.profile.Environment, r.agentHostID, state.RuntimeID, state.Worktree)
}
