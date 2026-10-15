package operation

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

type EffectResult struct {
	Request   b.LifecycleRequest
	Receipt   b.LifecycleReceipt
	Observer  b.LifecycleObserver
	Err       error
	Uncertain bool
}

type InvocationMode string

const (
	ExecuteNewIntent        InvocationMode = "execute_new_intent"
	ReconcileOriginalIntent InvocationMode = "reconcile_original_intent"
)

// Invoke receives an already committed immutable intent. An existing intent
// has only the reconciliation route, even when acceptance was never observed.
func Invoke(ctx context.Context, port interface {
	b.LifecyclePreparationPort
	b.LifecycleExecutePort
}, reconciler b.LifecycleReconcilePort, request b.LifecycleRequest, mode InvocationMode) EffectResult {
	if err := request.Validate(); err != nil {
		return EffectResult{Request: request.Clone(), Receipt: b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationRejected}, Err: err}
	}
	// A caller cannot turn an accepted/unknown/settled operation into a fresh
	// mutation by requesting Execute. The persisted original status wins.
	switch request.Operation.Status {
	case b.OperationAccepted, b.OperationUnknown, b.OperationConfirmed, b.OperationRejected:
		mode = ReconcileOriginalIntent
	case b.OperationInFlight:
	default:
		return EffectResult{Request: request.Clone(), Receipt: b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationRejected}, Err: errors.New("external action has no committed in-flight intent")}
	}
	if mode != ExecuteNewIntent && mode != ReconcileOriginalIntent {
		return EffectResult{Request: request.Clone(), Err: errors.New("external action invocation mode is unverified")}
	}
	reconcile := mode == ReconcileOriginalIntent
	result := EffectResult{Request: request.Clone()}
	if !reconcile {
		err := ctx.Err()
		if err == nil {
			err = reliable.QualifyAction(ctx, port, request)
		}
		if err == nil {
			err = ctx.Err() // Read-only qualification may have consumed the deadline.
		}
		if err != nil {
			result.Receipt = b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationRejected, Diagnostic: err.Error()}
			result.Err = err
			return result
		}
	}
	if reconcile {
		result.Receipt, result.Observer, result.Err = reconciler.Reconcile(ctx, request.Clone())
	} else {
		result.Receipt, result.Observer, result.Err = port.Execute(ctx, request.Clone())
	}
	result.Receipt = result.Receipt.Clone()
	validationErr := result.Receipt.ValidateFor(result.Request)
	result.Err = errors.Join(result.Err, validationErr)
	result.Uncertain = !reconcile && (validationErr != nil || result.Receipt.Disposition == b.OperationUnknown || result.Err != nil && result.Receipt.Disposition != b.OperationRejected)
	return result
}

func Outcome(receipt b.LifecycleReceipt, err error) (string, string) {
	status, diagnostic := string(receipt.Disposition), receipt.Diagnostic
	switch receipt.Disposition {
	case b.OperationAccepted, b.OperationConfirmed, b.OperationRejected, b.OperationUnknown:
	default:
		status = string(b.OperationUnknown)
		if diagnostic == "" {
			diagnostic = "lifecycle receipt disposition is unverified"
		}
	}
	if err != nil {
		diagnostic = err.Error()
		if receipt.Disposition != b.OperationRejected {
			status = "outcome_unknown"
		}
	}
	return status, diagnostic
}
