package orcatransport

import (
	"context"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"reflect"
	"strings"
)

// VerifyStopped constructs only a frozen reader, never an observer or execution.
func VerifyStopped(ctx context.Context, setup b.LifecycleSetup) error {
	p, err := NewLifecycle(setup)
	if err != nil {
		return err
	}
	l := p.(*Lifecycle)
	if l.structured() {
		if err := l.validateStructured(setup.Snapshot); err != nil {
			return err
		}
		return l.verifyAgentSessionStopped(setup.Snapshot)
	}
	return l.verifyStopped(ctx, setup.Snapshot)
}

func (l *Lifecycle) verifyStopped(ctx context.Context, s b.LifecycleSnapshot) error {
	if l.localCodexScope() {
		return l.verifyLocalCodexStopped(ctx, s)
	}
	state, err := l.state(s)
	if err != nil {
		return err
	}
	if state.Process.Kind != "orca_terminal" || state.Process != s.Ref.External || state.Process.ExecutionHost != l.profile.Target().ExecutionHost || state.Process.Executable != l.profile.Executable {
		return errors.New("original Orca resource binding is incomplete")
	}
	if err = VerifyTerminalStopped(ctx, l.client, state.Process); err != nil {
		return err
	}
	if l.profile.nativeTerminalContract() {

		return ctx.Err()
	}
	if l.profile.Terminal == nil {
		return errors.New("local Codex helper scope is unavailable; PTY exit is insufficient")
	}
	if state.RemoteReadiness != nil {
		return errors.New("native terminal helper has incompatible Codex resource evidence")
	}
	if state.Control != nil {
		return errors.New("native terminal helper has unexpected control resource")
	}
	owner, e := l.readTerminalOwner()
	if e != nil || state.TerminalOwner == nil || !reflect.DeepEqual(state.TerminalOwner, &owner) || !l.terminalExitVerified(owner) {
		return errors.Join(e, errors.New("original native terminal exit cleanup is unverified"))
	}
	return ctx.Err()
}

func (l *Lifecycle) codexReadiness(state State, workspace string) (codex.RemoteReadiness, error) {
	if l.profile.Environment != "" {
		return codex.RemoteReadiness{}, errors.New("helper writer-tree verification is local only")
	}
	r := state.RemoteReadiness
	if r == nil || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.RuntimeSHA256 == "" || r.RuntimeSHA256 != l.profile.RuntimeSHA256 || r.ConfigurationDigest == "" || r.ConfigurationDigest != l.profile.RuntimeConfigurationDigest || !codex.ValidNativeID(r.SessionID) || r.Workspace == "" || !strings.EqualFold(strings.ReplaceAll(r.Workspace, "\\", "/"), strings.ReplaceAll(workspace, "\\", "/")) || r.Repository == "" || r.Repository != l.setup.RepositoryURL || r.Process.Kind != "os" || r.Process.PID <= 0 || r.Process.Created == "" || r.Process.Executable == "" || r.Owner.Kind != "os" || r.Owner.PID <= 0 || r.Owner.Created == "" || r.Owner.Executable == "" || r.CheckedAt.IsZero() || state.TerminalOwner != nil || state.TerminalReadiness != nil {
		return codex.RemoteReadiness{}, errors.New("original Codex native exit binding is incomplete; PTY exit is insufficient")
	}
	return *r, nil
}

func (l *Lifecycle) originalForStop(ctx context.Context, s b.LifecycleSnapshot) error {
	if !l.usesControl() {
		_, err := l.original(ctx, s)
		return err
	}
	t, err := InspectBoundTerminal(ctx, l.client, s.Ref.External)
	if err != nil {
		return err
	}
	if t.WorktreeID != s.Ref.External.WorktreeID || !strings.EqualFold(strings.ReplaceAll(t.WorktreePath, "\\", "/"), strings.ReplaceAll(s.Workspace.Path, "\\", "/")) || strings.TrimPrefix(t.Branch, "refs/heads/") != s.Workspace.Branch {
		return errors.New("original Codex stop workspace or branch changed")
	}
	return nil
}
