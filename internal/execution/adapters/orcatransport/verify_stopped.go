package orcatransport

import (
	"context"
	"errors"
	"fmt"
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
	// Generic native helpers additionally own a Job/authentication broker outside
	// the PTY. Reuse their bound exit proof; PTY absence alone is insufficient.
	if l.profile.Terminal != nil {
		owner, e := l.readTerminalOwner()
		if e != nil || state.TerminalOwner == nil || !reflect.DeepEqual(state.TerminalOwner, &owner) || !l.terminalExitVerified(owner) {
			return errors.Join(e, errors.New("original native terminal exit/authentication cleanup is unverified"))
		}
	}
	verifyReader := func(a *NativeInspection) error {
		if a == nil {
			return nil
		}
		if !a.Attempted || a.At.IsZero() || a.Process == nil {
			return errors.New("original readonly creation identity is unverified")
		}
		p := *a.Process
		if p.Kind != "orca_original_turn_reader" || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == "" || p.Incarnation == "" || p.Terminal == state.Process.Terminal {
			return errors.New("original readonly resource binding changed")
		}
		if a.Closed && !a.Unknown {
			return nil
		}
		if e := l.client.VerifyTerminalStopped(ctx, p); e != nil {
			return fmt.Errorf("original readonly cleanup is unverified: %w", e)
		}
		return nil
	}
	for _, turn := range state.Turns {
		if err = verifyReader(turn.Inspection); err != nil {
			return err
		}
	}
	if state.RemoteReadiness != nil {
		journal, e := l.loadInspections(state)
		if e != nil {
			return e
		}
		for _, entry := range journal.Entries {
			if err = verifyReader(&entry.Inspection); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
