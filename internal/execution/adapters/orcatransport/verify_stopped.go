package orcatransport

import (
	"context"
	"errors"
	"reflect"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// VerifyStopped constructs only a frozen reader, never an observer or execution.
func VerifyStopped(ctx context.Context, setup b.LifecycleSetup) error {
	p, err := NewLifecycle(setup)
	if err != nil {
		return err
	}
	return p.(*Lifecycle).verifyStopped(ctx, setup.Snapshot)
}

func (l *Lifecycle) verifyStopped(ctx context.Context, s b.LifecycleSnapshot) error {
	state, err := l.state(s)
	if err != nil {
		return err
	}
	if state.Process.Kind != "orca_terminal" || state.Process != s.Ref.External || state.Process.ExecutionHost != l.profile.Target().ExecutionHost || state.Process.Executable != l.profile.Executable {
		return errors.New("original Orca resource binding is incomplete")
	}
	if err = l.client.VerifyTerminalStopped(ctx, state.Process); err != nil {
		return err
	}
	// Inventory absence proves only PTY exit. Codex readiness records a root,
	// but has no bound owner/tree/authentication exit proof. Do not substitute
	// a local PID probe for a remote execution-host fact or invent new proof.
	if l.profile.Terminal == nil {
		return errors.New("original Codex native writer-tree and authentication settlement remains unverified; PTY exit is insufficient")
	}
	if state.RemoteReadiness != nil {
		return errors.New("native terminal helper has incompatible Codex resource evidence")
	}
	for _, turn := range state.Turns {
		if turn.Inspection != nil {
			return errors.New("native terminal helper has unexpected readonly resources")
		}
	}
	owner, e := l.readTerminalOwner()
	if e != nil || state.TerminalOwner == nil || !reflect.DeepEqual(state.TerminalOwner, &owner) || !l.terminalExitVerified(owner) {
		return errors.Join(e, errors.New("original native terminal exit/authentication cleanup is unverified"))
	}
	return ctx.Err()
}
