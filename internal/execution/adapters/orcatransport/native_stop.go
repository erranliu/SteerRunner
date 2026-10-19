package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const NativeStopDiagnostic = "Stopped the original Orca terminal; detached background processes and execution-host credentials were not verified or revoked."

// Scope describes the frozen contract, including when its proof is unavailable.
// It is never persisted or used as authorization to resume or resend input.
func (l *Lifecycle) StopEvidenceScope() string {
	if l.profile.Environment != "" && l.profileVersion == NativeRemoteProfileVersion {
		return "orca_terminal"
	}
	return ""
}

func StopEvidenceScope(setup b.LifecycleSetup) (string, error) {
	p, err := NewLifecycle(setup)
	if err != nil {
		return "", err
	}
	l := p.(*Lifecycle)
	if err = l.validate(setup.Snapshot); err != nil {
		return "", err
	}
	scope := l.StopEvidenceScope()
	if scope != "" {
		_, err = l.state(setup.Snapshot)
	}
	return scope, err
}

func (l *Lifecycle) remoteStopResult(ctx context.Context, r b.LifecycleRequest, closeOriginal bool) (receipt b.LifecycleReceipt, err error) {
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
	if l.client.VerifyTerminalStopped(ctx, p) != nil && closeOriginal {
		progress.Phase = "not-attempted"
		t, inspectErr := l.client.InspectBoundTerminal(ctx, l.profile.Target(), p)
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
		proof, closeErr := l.client.CloseTerminalChecked(ctx, p.Terminal, p.RuntimeID)
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
		err = l.client.VerifyTerminalStopped(ctx, p)
		if err == nil {
			break
		}
		if ctx.Err() != nil || time.Now().After(until) {
			return receipt, err
		}
		select {
		case <-ctx.Done():
			return receipt, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	receipt.State.WorkerAbsent = true
	if l.StopEvidenceScope() == "" {
		return receipt, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Legacy remote terminal ended; original helper writer-tree/authentication settlement remains unverified. Use the original build or manual cleanup."}
	}
	progress.Phase = "confirmed"
	receipt.Disposition = b.OperationConfirmed
	receipt.State.Stopped = true
	receipt.Diagnostic = NativeStopDiagnostic
	return receipt, nil
}

func (l *Lifecycle) reconcileRemoteStop(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	var saved b.LifecycleReceipt
	var progress stopProgress
	firstClose := json.Unmarshal(r.Operation.Receipt, &saved) == nil && saved.ValidateFor(r) == nil && json.Unmarshal(saved.Operation.Receipt, &progress) == nil && progress.Version == 1 && progress.Phase == "not-attempted" && !saved.State.Stopped
	receipt, err := l.remoteStopResult(ctx, r, firstClose)
	return receipt, nil, err
}
