// Package operation owns durable intent and receipt rules. The application
// retains the dispatch gate, persistence, and native resource ownership.
package operation

import (
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"time"
)

type Intent struct {
	Request b.LifecycleRequest  `json:"request"`
	Receipt *b.LifecycleReceipt `json:"receipt,omitempty"`
	Mode    string              `json:"mode"`
}

func (i Intent) NeedsReconciliation() bool {
	status := i.Request.Operation.Status
	return status == b.OperationInFlight || status == b.OperationUnknown || status == b.OperationAccepted && i.Receipt != nil && i.Receipt.State.ResourcePending
}

// New fixes the original request before the application commits and dispatches
// it. IDs and time are supplied by the caller; this function has no effects.
func New(runID string, snapshot b.LifecycleSnapshot, candidate b.LifecycleCandidate, id, key string, at time.Time) (Intent, error) {
	value, err := InputValue(candidate.Action, candidate.Input)
	if err != nil {
		return Intent{}, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Intent{}, err
	}
	ref := snapshot.Ref
	op := b.Operation{Version: 1, ID: id, RunID: runID, AttemptID: snapshot.Session.AttemptID, Backend: ref.Backend, Protocol: ref.Protocol, Target: ref.Target, Action: string(candidate.Action), IdempotencyKey: key, Request: raw, PayloadDigest: reliable.Digest(raw), Status: b.OperationInFlight, StartedAt: at, UpdatedAt: at, Ref: &ref, LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: snapshot.Session.ProjectID, ExecutionID: snapshot.Session.ExecutionID, ConfigurationDigest: snapshot.ConfigurationDigest, ResourceGeneration: snapshot.ResourceGeneration, InputBindingDigest: b.LifecycleBindingDigest(snapshot), Ref: ref}}
	request := b.LifecycleRequest{Snapshot: snapshot.Clone(), Operation: op, Input: candidate.Input.Clone()}
	if err := request.Validate(); err != nil {
		return Intent{}, err
	}
	return Intent{Request: request, Mode: string(ExecuteNewIntent)}, nil
}

// Settle records the execution result in both existing representations. The
// caller supplies the already-redacted diagnostic and receipt and commits the
// aggregate afterwards. This does not change the saved input or its binding.
func (i *Intent) Settle(operations []b.Operation, receipt b.LifecycleReceipt, mode InvocationMode, status b.OperationStatus, diagnostic string, at time.Time) {
	op := i.Request.Operation
	op.Status, op.UpdatedAt, op.Diagnostic = status, at, diagnostic
	saved := receipt.Clone()
	if op.Action != string(b.LifecyclePrepareWorkspace) {
		saved.Operation.Receipt = nil
	}
	wire, err := json.Marshal(saved)
	if err != nil {
		panic(err) // Preserve the existing canonical receipt encoding contract.
	}
	op.Receipt = wire
	i.Request.Operation, i.Receipt, i.Mode = op, &saved, string(mode)
	for n := range operations {
		if operations[n].ID == op.ID {
			operations[n] = op
		}
	}
}

// SettleStop preserves stop's status-only snapshot update and original receipt
// identity. It deliberately does not normalize the older snapshot's key,
// timestamps or receipt summary to the journal's representation.
func (i *Intent) SettleStop(operations []b.Operation, receipt b.LifecycleReceipt, status b.OperationStatus) {
	original := i.Request.Operation
	// Preserve the implementation's committed progress under the same original
	// operation binding. Missing or foreign receipts never authorize continuation.
	if receipt.ValidateFor(i.Request) == nil {
		wire, err := json.Marshal(receipt)
		if err != nil {
			panic(err)
		}
		i.Request.Operation.Receipt = wire
	} else {
		i.Request.Operation.Receipt = nil
	}
	i.Request.Operation.Status = status
	receipt.Operation = original
	i.Receipt = &receipt
	for n := range operations {
		if operations[n].ID == original.ID {
			operations[n].Status = status
		}
	}
}
