package orcatransport

import (
	"context"
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

func (l *Lifecycle) InspectLifecycleAuditWorkspace(ctx context.Context, s b.LifecycleSnapshot, preHead string) (b.WorkspaceInspection, error) {
	var empty b.WorkspaceInspection
	if l.profile.Environment != "" {
		return empty, errors.New("native remote Orca does not provide helper workspace audit")
	}
	if err := l.validate(s); err != nil {
		return empty, err
	}
	return l.inspectStructuredAuditWorkspace(ctx, s, preHead)
}

func (l *Lifecycle) inspectStructuredAuditWorkspace(ctx context.Context, s b.LifecycleSnapshot, preHead string) (b.WorkspaceInspection, error) {
	var empty b.WorkspaceInspection
	state, err := l.agentState(s)
	if err != nil {
		return empty, err
	}
	if state.Worktree.ID == "" || !state.Attempted || state.CloseAttempted {
		return empty, errors.New("original structured workspace owner unavailable")
	}
	release, err := l.inspectionGate(ctx)
	if err != nil {
		return empty, err
	}
	defer release()
	if err := l.originalAgentWorkspace(ctx, state); err != nil {
		return empty, err
	}
	remote := l.setup.RemoteName
	if remote == "" {
		remote = "origin"
	}
	fact, _, err := workspace.InspectFixedAudit(ctx, l.setup.ProcessHost, state.Worktree.Path, l.setup.RepositoryURL, remote, s.Workspace, preHead)
	if err != nil {
		return fact, err
	}
	if err := l.originalAgentWorkspace(ctx, state); err != nil {
		return empty, err
	}
	return fact, nil
}
