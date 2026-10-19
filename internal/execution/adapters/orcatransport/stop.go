package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type stopProgress struct {
	Version int    `json:"version"`
	Phase   string `json:"phase"`
}

// Only an identity-bound committed receipt proving no close was attempted can
// authorize the first close during reconciliation. A crash with no receipt or
// a lost close response remains unknown; neither permits repeating an effect.
func (l *Lifecycle) reconcileStop(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	receipt.Operation.Receipt = nil
	var saved b.LifecycleReceipt
	var progress stopProgress
	if json.Unmarshal(r.Operation.Receipt, &saved) != nil || saved.ValidateFor(r) != nil || json.Unmarshal(saved.Operation.Receipt, &progress) != nil || progress.Version != 1 {
		return receipt, nil, errors.New("Original terminal stop has no verified progress; no close was repeated")
	}
	receipt.Operation.Receipt = append([]byte(nil), saved.Operation.Receipt...)
	switch progress.Phase {
	case "not-attempted":
		if saved.State.Stopped {
			break
		}
		result, err := l.StopLifecycle(ctx, r, nil)
		return result, nil, err
	case "confirmed":
		if saved.Disposition == b.OperationConfirmed && saved.State.Stopped && saved.State.WorkerAbsent {
			receipt.Disposition = b.OperationConfirmed
			receipt.State = saved.State
			return receipt, nil, nil
		}
	case "business-confirmed":
		if !saved.State.WorkerAbsent {
			return receipt, nil, errors.New("original business stop proof missing")
		}
		receipt.State.WorkerAbsent = true
		state, err := l.state(r.Snapshot)
		if err != nil {
			return receipt, nil, err
		}
		if err = l.reconcileAuthenticationStop(ctx, state); err != nil {
			return receipt, nil, err
		}
		if err = l.closeControl(ctx, state); err != nil {
			return receipt, nil, err
		}
		if l.profile.Terminal == nil && l.profile.RuntimeExecutable != "" {
			if err = l.verifyStopped(ctx, r.Snapshot); err != nil {
				return receipt, nil, err
			}
		}
		progress.Phase = "confirmed"
		receipt.Operation.Receipt, _ = json.Marshal(progress)
		receipt.Disposition = b.OperationConfirmed
		receipt.State.Stopped = true
		receipt.State.WorkerAbsent = true
		return receipt, nil, nil
	}
	return receipt, nil, errors.New("Original terminal close outcome is unknown; no close was repeated")
}
