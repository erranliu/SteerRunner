package orcatransport

import (
	"encoding/json"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const NativeStopDiagnostic = "Stopped the original Orca terminal; detached background processes and execution-host credentials were not verified or revoked."

// Scope describes the frozen contract, including when its proof is unavailable.
// The Run saves it as a contract fact, never as authority to resume or resend.
func (l *Lifecycle) StopEvidenceScope() string { return l.frozenRecord().StopEvidenceScope() }

func StopEvidenceScope(setup b.LifecycleSetup) (string, error) {
	l, err := newFrozenReader(setup)
	if err != nil {
		return "", err
	}
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
