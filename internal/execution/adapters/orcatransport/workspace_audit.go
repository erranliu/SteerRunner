package orcatransport

import (
	"context"
	"errors"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func (l *Lifecycle) InspectLifecycleAuditWorkspace(ctx context.Context, s b.LifecycleSnapshot, preHead string) (b.WorkspaceInspection, error) {
	var empty b.WorkspaceInspection
	if l.profile.Environment != "" {
		return empty, errors.New("native remote Orca does not provide helper workspace audit")
	}
	if err := l.validate(s); err != nil {
		return empty, err
	}
	if l.structured() {
		return l.inspectStructuredAuditWorkspace(ctx, s, preHead)
	}
	state, err := l.state(s)
	if err != nil {
		return empty, err
	}
	if !l.usesControl() || state.RemoteReadiness == nil {
		return empty, errors.New("original managed Codex workspace owner unavailable")
	}
	if _, err = l.original(ctx, s); err != nil {
		return empty, err
	}
	q := codex.RemoteWorkspaceAuditQuery{Intent: s.Workspace, PreHead: preHead}
	if err = q.Validate(state.RemoteReadiness.Workspace); err != nil {
		return empty, err
	}
	release, err := l.inspectionGate(ctx)
	if err != nil {
		return empty, err
	}
	defer release()
	response, err := l.callControlWithAudit(ctx, state, "workspace-audit", "", &q)
	if err != nil {
		return empty, err
	}
	if response.Error != "" || response.Workspace == nil {
		return empty, errors.New("original workspace audit unverified")
	}
	if err = codex.ValidateRemoteWorkspaceAudit(*state.RemoteReadiness, q, *response.Workspace); err != nil {
		return empty, err
	}
	return response.Workspace.Inspection, nil
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
	q := codex.RemoteWorkspaceAuditQuery{Intent: s.Workspace, PreHead: preHead}
	fact, _, err := codex.InspectOwnedWorkspaceAudit(ctx, l.setup.ProcessHost, state.Worktree.Path, l.setup.RepositoryURL, remote, q)
	if err != nil {
		return fact, err
	}
	if err := l.originalAgentWorkspace(ctx, state); err != nil {
		return empty, err
	}
	return fact, nil
}
