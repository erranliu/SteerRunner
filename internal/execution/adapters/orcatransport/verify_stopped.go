package orcatransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
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
	if l.profile.Terminal == nil {
		if err = l.verifyCodexExit(ctx, state, s.Workspace.Path); err != nil {
			return err
		}
		return l.verifyInspectionsStopped(ctx, state)
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

func (l *Lifecycle) codexReadiness(state State, workspace string) (codex.RemoteReadiness, error) {
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

// A settled original helper may return to its shell before its PTY is closed.
// Native exit is proven separately; keep every PTY/workspace binding here.
func (l *Lifecycle) originalCodexStopTerminal(ctx context.Context, s b.LifecycleSnapshot) error {
	t, err := l.client.InspectBoundTerminal(ctx, l.profile.Target(), s.Ref.External)
	if err != nil {
		return err
	}
	if t.WorktreeID != s.Ref.External.WorktreeID || t.WorktreePath != s.Workspace.Path || strings.TrimPrefix(t.Branch, "refs/heads/") != s.Workspace.Branch {
		return errors.New("original Codex stop workspace or branch changed")
	}
	return nil
}

// The caller's existing journal covers creations not yet copied into the Run
// snapshot. Missing/unknown ownership cannot be repaired by starting a reader.
func (l *Lifecycle) verifyInspectionsStopped(ctx context.Context, state State) error {
	j, err := l.loadInspections(state)
	if err != nil {
		return err
	}
	originals := []b.ProcessIdentity{}
	seen := map[b.ProcessIdentity]bool{}
	check := func(a *NativeInspection) error {
		if a == nil {
			return nil
		}
		if !a.Attempted || a.Process == nil || !a.NativeSettled || a.At.IsZero() {
			return errors.New("original readonly native tree settlement remains unverified")
		}
		p := a.Process
		if p.Kind != "orca_original_turn_reader" || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == state.Process.Terminal || p.Terminal == "" || p.Incarnation == "" {
			return errors.New("original readonly resource identity differs")
		}
		if !seen[*p] {
			originals = append(originals, *p)
			seen[*p] = true
		}
		return nil
	}
	for _, entry := range j.Entries {
		if err = check(&entry.Inspection); err != nil {
			return err
		}
	}
	for _, turn := range state.Turns {
		a := turn.Inspection
		for _, entry := range j.Entries {
			if a != nil && a.Process != nil && entry.Inspection.Process != nil && *a.Process == *entry.Inspection.Process {
				copy := entry.Inspection
				a = &copy
			}
		}
		if err = check(a); err != nil {
			return err
		}
	}
	if len(originals) == 0 {
		return ctx.Err()
	}
	return l.client.VerifyTerminalsStopped(ctx, originals)
}
