package orcatransport

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// ManagedWorkspaceIdentity uses the original native incarnation, not its name.
func ManagedWorkspaceIdentity(setup b.LifecycleSetup) (string, error) {
	l, err := newFrozenReader(setup)
	if err != nil {
		return "", err
	}
	state, err := l.agentState(setup.Snapshot)
	if err != nil || state.Worktree.InstanceID == "" {
		return "", errors.Join(err, errors.New("managed Orca workspace identity unavailable"))
	}
	return "orca:" + digest([]byte(l.agentHostID+"\x00"+state.RuntimeID+"\x00"+state.Worktree.ID+"\x00"+state.Worktree.InstanceID)), nil
}

// ReuseManagedWorkspace retains only workspace facts. Session, input, close,
// history and shells belong to the old Run and are never copied to the new one.
func ReuseManagedWorkspace(ctx context.Context, setup b.LifecycleSetup, next *b.LifecycleSnapshot) error {
	p, err := NewLifecycle(setup)
	if err != nil {
		return err
	}
	l := p.(*Lifecycle)
	return l.reuseManagedWorkspace(ctx, next)
}

func (l *Lifecycle) reuseManagedWorkspace(ctx context.Context, next *b.LifecycleSnapshot) error {
	source := l.setup.Snapshot
	state, err := l.agentState(source)
	if err != nil {
		return err
	}
	if err = l.verifyAgentSessionStopped(source); err != nil {
		return err
	}
	if err = l.originalAgentWorkspace(ctx, state); err != nil {
		return err
	}
	fresh := agentSessionState{RuntimeID: state.RuntimeID, Worktree: state.Worktree, SessionID: agentSessionID(*next)}
	next.Ref.External = source.Ref.External
	next.Ref.Target = source.Ref.Target
	next.Session.Target = next.Ref.Target
	next.Implementation, err = l.agentEnvelope(*next, fresh)
	return err
}
