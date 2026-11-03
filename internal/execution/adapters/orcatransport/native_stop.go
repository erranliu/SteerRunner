package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const NativeStopDiagnostic = "Stopped the original Orca terminal; detached background processes and execution-host credentials were not verified or revoked."

// Scope describes the frozen contract, including when its proof is unavailable.
// The Run saves it as a contract fact, never as authority to resume or resend.
func (l *Lifecycle) StopEvidenceScope() string {
	if l.structured() {
		return AgentSessionStopScope
	}
	if l.profile.Environment != "" && l.profileVersion != NativeRemoteProfileVersion && l.profileVersion != NativeProfileVersion {
		return ""
	}
	return StopScope(l.profile, l.setup.Snapshot.Ref.Agent.Name)
}

func StopEvidenceScope(setup b.LifecycleSetup) (string, error) {
	p, err := NewLifecycle(setup)
	if err != nil {
		return "", err
	}
	l := p.(*Lifecycle)
	if l.structured() {
		return AgentSessionStopScope, l.validateStructured(setup.Snapshot)
	}
	if err = l.validate(setup.Snapshot); err != nil {
		return "", err
	}
	scope := l.StopEvidenceScope()
	if scope == LocalCodexStopScope {
		_, err = l.resourceState(setup.Snapshot)
	} else if scope != "" {
		_, err = l.state(setup.Snapshot)
	}
	return scope, err
}

func (l *Lifecycle) nativeStopResult(ctx context.Context, r b.LifecycleRequest, closeOriginal bool) (receipt b.LifecycleReceipt, err error) {
	receipt = b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	progress := stopProgress{Version: 1, Phase: "attempted"}
	defer func() { receipt.Operation.Receipt, _ = json.Marshal(progress) }()
	if err = r.Validate(); err != nil {
		return receipt, err
	}
	state, err := l.state(r.Snapshot)
	if err != nil {
		return receipt, err
	}
	p := r.Snapshot.Ref.External
	if state.Process != p || p.Kind != "orca_terminal" || p.Executable != l.profile.Executable || p.ExecutionHost != l.profile.Target().ExecutionHost {
		return receipt, errors.New("original remote Orca resource binding is incomplete")
	}
	var closeFailure error
	if VerifyTerminalStopped(ctx, l.client, p) != nil && closeOriginal {
		progress.Phase = "not-attempted"
		t, inspectErr := InspectBoundTerminal(ctx, l.client, p)
		if inspectErr != nil {
			return receipt, inspectErr
		}
		// Original startup can be accepted before Orca publishes branch metadata.
		// This never authorizes input: only an initializing, zero-turn terminal
		// may omit branch, while its exact bound worktree/path still must match.
		initializing := !state.LaunchConfirmed && len(state.Turns) == 0
		branchMatches := strings.TrimPrefix(t.Branch, "refs/heads/") == r.Snapshot.Workspace.Branch
		if t.WorktreeID != p.WorktreeID || t.WorktreePath != r.Snapshot.Workspace.Path || !branchMatches && !(initializing && t.Branch == "") {
			return receipt, errors.New("original remote Orca stop workspace changed")
		}
		progress.Phase = "attempted"
		// A lost close response is reconciled only through the same inventory.
		// This function never repeats close; absence does not invent a receipt.
		proof, closeErr := CloseTerminalChecked(ctx, l.client, p.Terminal, p.RuntimeID)
		if native.IsCLIQueueError(closeErr) {
			// The shared CLI queue proves no command process was invoked. Keep
			// the existing first-close authorization instead of making a safe
			// retry indistinguishable from a lost remote close response.
			progress.Phase = "not-attempted"
			return receipt, closeErr
		}
		closeFailure = closeErr
		var identity *b.BackendError
		if errors.As(closeErr, &identity) && identity.Code == b.BackendIdentityMismatch {
			return receipt, closeErr
		}
		if closeErr == nil && proof.Disposition != "confirmed" {
			receipt.Disposition = b.OperationAccepted
			return receipt, errors.New("original terminal close accepted but not confirmed; reconcile the original inventory")
		}
	}
	until := time.Now().Add(3 * time.Second)
	for {
		err = VerifyTerminalStopped(ctx, l.client, p)
		if err == nil {
			break
		}
		if ctx.Err() != nil || time.Now().After(until) {
			return receipt, errors.Join(closeFailure, err)
		}
		select {
		case <-ctx.Done():
			return receipt, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	receipt.State.WorkerAbsent = true

	progress.Phase = "confirmed"
	receipt.Disposition = b.OperationConfirmed
	receipt.State.Stopped = true
	receipt.Diagnostic = NativeStopDiagnostic
	return receipt, nil
}

// ConfirmedTerminalStopReceipt reads the existing durable operation receipt.
// A terminal proved absent cannot return in the same incarnation. Later Orca
// restarts or worktree removal do not invalidate that completed stop. This is
// only an Orca terminal proof, never a helper/writer-tree or credential proof.
func ConfirmedTerminalStopReceipt(r b.LifecycleRequest, current b.LifecycleSnapshot) (b.LifecycleReceipt, bool) {
	var saved b.LifecycleReceipt
	var progress stopProgress
	if r.Operation.Action != string(b.LifecycleStopExecution) ||
		r.Snapshot.Ref != current.Ref || r.Snapshot.Session.ProjectID != current.Session.ProjectID ||
		r.Snapshot.Session.ExecutionID != current.Session.ExecutionID || r.Snapshot.Session.AttemptID != current.Session.AttemptID ||
		r.Snapshot.Session.SessionID != current.Session.SessionID || r.Snapshot.Session.Generation != current.Session.Generation ||
		r.Snapshot.ConfigurationDigest != current.ConfigurationDigest || r.Snapshot.ResourceGeneration != current.ResourceGeneration ||
		r.Operation.RunID != current.Ref.RunID || r.Operation.AttemptID != current.Ref.AttemptID ||
		r.Operation.Ref == nil || *r.Operation.Ref != current.Ref ||
		current.Ref.External.Kind != "orca_terminal" || current.Ref.External.RuntimeID == "" ||
		current.Ref.External.WorktreeID == "" || current.Ref.External.Terminal == "" || current.Ref.External.Incarnation == "" ||
		json.Unmarshal(r.Operation.Receipt, &saved) != nil || saved.ValidateFor(r) != nil ||
		saved.Disposition != b.OperationConfirmed || !saved.State.Stopped || !saved.State.WorkerAbsent ||
		saved.State.WorkerLive || saved.State.ResourcePending ||
		json.Unmarshal(saved.Operation.Receipt, &progress) != nil || progress.Version != 1 || progress.Phase != "confirmed" {
		return b.LifecycleReceipt{}, false
	}
	return saved, true
}

func (l *Lifecycle) reconcileNativeStop(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if l.StopEvidenceScope() == "orca_terminal" {
		if saved, ok := ConfirmedTerminalStopReceipt(r, l.setup.Snapshot); ok {
			return saved, nil, nil
		}
	}
	var saved b.LifecycleReceipt
	var progress stopProgress
	firstClose := json.Unmarshal(r.Operation.Receipt, &saved) == nil && saved.ValidateFor(r) == nil && json.Unmarshal(saved.Operation.Receipt, &progress) == nil && progress.Version == 1 && progress.Phase == "not-attempted" && !saved.State.Stopped
	receipt, err := l.nativeStopResult(ctx, r, firstClose)
	return receipt, nil, err
}
