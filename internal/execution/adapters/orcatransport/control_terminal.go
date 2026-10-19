package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"time"
)

const controlPurpose = "native-control"

type ControlTerminal struct {
	Process b.ProcessIdentity         `json:"process"`
	Ready   *codex.RemoteControlSetup `json:"ready,omitempty"`
}

func (l *Lifecycle) usesControl() bool {
	return l.profile.Environment == "" && l.profile.Terminal == nil && l.profile.RuntimeExecutable != ""
}

func (l *Lifecycle) controlSetup(s b.LifecycleSnapshot, state State) codex.RemoteControlSetup {
	workspace := s.Workspace.Path
	if state.RemoteReadiness != nil {
		workspace = state.RemoteReadiness.Workspace
	}
	if workspace == "" && state.Control != nil && state.Control.Ready != nil {
		workspace = state.Control.Ready.Workspace
	}
	raw, _ := json.Marshal(struct {
		Project, Execution, Attempt, Generation, Configuration, Helper string
		Process                                                        b.ProcessIdentity
		Workspace                                                      string
	}{s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest, l.profile.RuntimeSHA256, state.Process, strings.ToLower(strings.ReplaceAll(workspace, "\\", "/"))})
	return codex.RemoteControlSetup{Binding: digest(raw), Workspace: workspace, RuntimeSHA256: l.profile.RuntimeSHA256}
}

func (l *Lifecycle) controlIntent(s b.LifecycleSnapshot, state State) b.ResourceIntent {
	raw, _ := json.Marshal(l.controlSetup(s, state))
	command := "& '" + strings.ReplaceAll(l.profile.RuntimeExecutable, "'", "''") + "' '--native-control' '" + base64.RawURLEncoding.EncodeToString(raw) + "'"
	return b.ResourceIntent{Purpose: controlPurpose, Name: "original-native-control", Objective: command}
}

func (l *Lifecycle) validateControl(s b.LifecycleSnapshot, state State, ready bool) error {
	c := state.Control
	if c == nil {
		return errors.New("original native control identity unavailable; no replacement created")
	}
	p := c.Process
	if p.Kind != "orca_native_control" || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == "" || p.Terminal == state.Process.Terminal || p.Incarnation == "" {
		return errors.New("original native control resource changed")
	}
	expected := l.controlSetup(s, state)
	if c.Ready != nil && (c.Ready.Binding != expected.Binding || c.Ready.RuntimeSHA256 != expected.RuntimeSHA256 || !strings.EqualFold(strings.ReplaceAll(c.Ready.Workspace, "\\", "/"), strings.ReplaceAll(expected.Workspace, "\\", "/"))) || ready && c.Ready == nil {
		return errors.New("original native control readiness unavailable or changed")
	}
	return nil
}

func (l *Lifecycle) prepareControl(ctx context.Context, r b.LifecycleRequest, reconcile bool) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	state, err := l.state(r.Snapshot)
	if err != nil {
		return receipt, err
	}
	intent := l.controlIntent(r.Snapshot, state)
	if !l.usesControl() || r.Input.Resource == nil || *r.Input.Resource != intent || state.LaunchAttempted || len(state.Turns) != 0 {
		return receipt, errors.New("native control preparation differs from original resource intent")
	}
	if state.Control == nil {
		if reconcile {
			return receipt, errors.New("original native control creation outcome unknown; zero business input and no replacement")
		}
		result, e := l.client.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+state.Process.WorktreeID, "--title", "SteerRunner native control", "--command", intent.Objective, "--json")
		var value struct{ Terminal native.Terminal }
		if e != nil || result.CallUncertain || result.RuntimeID != state.Process.RuntimeID || json.Unmarshal(result.Payload, &value) != nil || value.Terminal.Handle == "" || value.Terminal.Handle == state.Process.Terminal || value.Terminal.IncarnationID == "" || value.Terminal.WorktreeID != state.Process.WorktreeID {
			return receipt, errors.New("original native control creation unverified; zero business input and no replacement")
		}
		state.Control = &ControlTerminal{Process: b.ProcessIdentity{Kind: "orca_native_control", RuntimeID: result.RuntimeID, Terminal: value.Terminal.Handle, Incarnation: value.Terminal.IncarnationID, WorktreeID: state.Process.WorktreeID}}
	}
	finish := func() {
		receipt.Process = &state.Process
		receipt.Implementation, err = l.envelope(r.Snapshot, state)
		receipt.State.ResourcePending = state.Control.Ready == nil
		if err == nil {
			receipt.Disposition = b.OperationAccepted
		}
		if err == nil && state.Control.Ready != nil {
			receipt.Disposition = b.OperationConfirmed
		}
	}
	if e := l.validateControl(r.Snapshot, state, false); e != nil {
		finish()
		return receipt, e
	}
	lines, e := l.controlLines(ctx, state)
	if e != nil {
		finish()
		return receipt, e
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, codex.RemoteControlReadyPrefix) {
			continue
		}
		raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteControlReadyPrefix))
		var ready codex.RemoteControlSetup
		if e != nil || b.DecodeStrictJSON(raw, &ready, "trailing native control readiness") != nil || ready != l.controlSetup(r.Snapshot, state) {
			finish()
			return receipt, errors.New("original native control helper binding changed")
		}
		state.Control.Ready = &ready
	}
	finish()
	return receipt, err
}

func (l *Lifecycle) controlLines(ctx context.Context, state State) ([]string, error) {
	if state.Control == nil {
		return nil, errors.New("original native control missing")
	}
	p := state.Control.Process
	if _, err := l.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, p); err != nil {
		return nil, err
	}
	r, err := l.client.MetaCall(ctx, "terminal", "read", "--terminal", p.Terminal, "--limit", "1000", "--json")
	var value struct {
		Terminal struct {
			Handle string
			Tail   []string
		}
	}
	if err != nil || r.CallUncertain || r.RuntimeID != p.RuntimeID || json.Unmarshal(r.Payload, &value) != nil || value.Terminal.Handle != p.Terminal {
		return nil, errors.New("original native control output identity changed")
	}
	for i := range value.Terminal.Tail {
		value.Terminal.Tail[i] = strings.TrimSpace(value.Terminal.Tail[i])
	}
	return value.Terminal.Tail, nil
}

func (l *Lifecycle) closeControl(ctx context.Context, state State) error {
	if !l.usesControl() {
		return nil
	}
	release, err := l.inspectionGate(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := l.validateControl(l.setup.Snapshot, state, false); err != nil {
		return err
	}
	proof, err := l.client.CloseReadonlyTerminal(ctx, state.Control.Process)
	if err != nil || proof.Disposition != "confirmed" {
		return errors.Join(err, errors.New("original native control closure unverified"))
	}
	if state.RemoteReadiness != nil {
		j, err := l.loadControlCheckpoint(state)
		if err != nil {
			return err
		}
		if j.Request.ID != "" {
			j.Closed = true
			return l.saveControlCheckpoint(state, j)
		}
	}
	return nil
}

// Polling sleeps stay cancellable; the fixed actor owns any in-flight query.
func controlPause(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}
