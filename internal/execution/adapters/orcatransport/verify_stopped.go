package orcatransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"reflect"
	"strings"
	"time"
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
	if l.profile.Environment != "" {
		if l.StopEvidenceScope() == "" {
			return errors.New("legacy remote helper settlement cannot be established by terminal absence")
		}
		return ctx.Err()
	}
	if l.profile.Terminal == nil {
		if err = l.verifyCodexExit(ctx, state, s.Workspace.Path); err != nil {
			return err
		}
		return l.verifyControlStopped(ctx, state)
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
	if _, err := codex.RemoteExitPath(*r); err != nil {
		return *r, err
	}
	return *r, nil
}

func (l *Lifecycle) verifyCodexExit(ctx context.Context, state State, workspace string) error {
	ready, err := l.codexReadiness(state, workspace)
	if err != nil {
		return err
	}
	path, _ := codex.RemoteExitPath(ready)
	raw, err := l.readStopArtifact(ctx, l.profile.Environment, state.Process.RuntimeID, state.Process.WorktreeID, path)
	if err != nil {
		return errors.Join(err, errors.New("original Codex native exit artifact unavailable"))
	}
	return codex.VerifyRemoteExit(raw, ready)
}

func (l *Lifecycle) stopCodexNative(ctx context.Context, state State, workspace string) error {
	ready, err := l.codexReadiness(state, workspace)
	if err != nil {
		return err
	}
	if l.verifyCodexExit(ctx, state, workspace) == nil {
		return nil
	}
	path, _ := codex.RemoteStopPath(ready)
	raw, _ := json.Marshal(ready)
	if err = native.UploadWorkspaceArtifact(ctx, l.profile.Environment, state.Process.RuntimeID, "id:"+state.Process.WorktreeID, path, bytes.NewReader(raw)); err != nil {
		return err
	}
	// Preserve the helper until it finishes its original Job/authentication
	// cleanup. Closing its PTY first can destroy the only available proof.
	for {
		if err = l.verifyCodexExit(ctx, state, workspace); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), err)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// The original checkpoint is transport evidence, never a replacement Run state.
func (l *Lifecycle) verifyControlStopped(ctx context.Context, state State) error {
	if err := l.validateControl(l.setup.Snapshot, state, false); err != nil {
		return err
	}
	if err := l.client.VerifyTerminalStopped(ctx, state.Control.Process); err != nil {
		return err
	}
	j, err := l.loadControlCheckpoint(state)
	if err != nil || !j.Closed {
		return errors.Join(err, errors.New("original control closure unverified"))
	}
	return nil
}

func (l *Lifecycle) originalForStop(ctx context.Context, s b.LifecycleSnapshot) error {
	if !l.usesControl() {
		_, err := l.original(ctx, s)
		return err
	}
	t, err := l.client.InspectBoundTerminal(ctx, l.profile.Target(), s.Ref.External)
	if err != nil {
		return err
	}
	if t.WorktreeID != s.Ref.External.WorktreeID || !strings.EqualFold(strings.ReplaceAll(t.WorktreePath, "\\", "/"), strings.ReplaceAll(s.Workspace.Path, "\\", "/")) || strings.TrimPrefix(t.Branch, "refs/heads/") != s.Workspace.Branch {
		return errors.New("original Codex stop workspace or branch changed")
	}
	return nil
}
