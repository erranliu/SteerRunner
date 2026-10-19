package codexworker

import (
	"context"
	"errors"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func (l ExecutionLifecycle) writersAbsent(binding Binding) bool {
	if binding.Process == nil || !l.Backend.ManagedProcessAbsent(binding) {
		return false
	}
	if owned := l.setup.OwnedProcess; owned != nil {
		if owned.Identity() != *binding.Process {
			return false
		}
		if tree, ok := owned.(executionbackend.ProcessTreeHandle); ok {
			return tree.WritersAbsent()
		}
		return false
	}
	return l.setup.WorkerTreeAbsent != nil && l.setup.WorkerTreeAbsent(*binding.Process)
}

func (l ExecutionLifecycle) closeAuthentication() error {
	if auth := l.Backend.Config.Authentication; auth != nil && !auth.Close() {
		if auth.broker != nil {
			return errors.New("authentication_cleanup_unverified: credential revoked; original socket resources could not be confirmed removed")
		}
		return errors.New("authentication_revocation_unverified: process exited; installation token remains bounded by its recorded expiry")
	}
	return nil
}

// ShutdownLifecycle releases only this worker's owned process tree. It does not
// send turn/interrupt or claim the interrupted native turn completed.
func (l ExecutionLifecycle) ShutdownLifecycle(ctx context.Context, snapshot executionbackend.LifecycleSnapshot, owned executionbackend.StopHandle) (executionbackend.LifecycleStateFacts, error) {
	if err := l.validateIdentity(snapshot); err != nil {
		return executionbackend.LifecycleStateFacts{}, err
	}
	if l.setup.LaunchExecutionOwner != nil {
		// The independent owner retains the native connection, Job and broker.
		// Closing this service's observation is not an execution stop.
		return executionbackend.LifecycleStateFacts{}, nil
	}
	if owned == nil {
		return executionbackend.LifecycleStateFacts{}, nil
	}
	if snapshot.Ref.External.Kind != "" && owned.Identity() != snapshot.Ref.External {
		return executionbackend.LifecycleStateFacts{}, errors.New("service stop changed the original owned worker identity")
	}
	defer owned.Close()
	identity := owned.Identity()
	owned.Kill()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if tree, ok := owned.(executionbackend.ProcessTreeHandle); ok && tree.WritersAbsent() && l.Backend.Process.Inspect(&identity) == "process absent" {
			return executionbackend.LifecycleStateFacts{WorkerAbsent: true, WritersAbsent: true, Stopped: true}, l.closeAuthentication()
		}
		select {
		case <-ctx.Done():
			return executionbackend.LifecycleStateFacts{}, errors.New("original owned worker tree exit could not be verified: " + ctx.Err().Error())
		case <-ticker.C:
		}
	}
}

func (l ExecutionLifecycle) reconcileStoppedObservation(ctx context.Context, request executionbackend.LifecycleRequest) (executionbackend.LifecycleReceipt, executionbackend.LifecycleObserver, error) {
	receipt := executionbackend.LifecycleReceipt{Operation: request.Operation, RequestID: request.Operation.ID, Disposition: executionbackend.OperationUnknown}
	binding, found, err := l.readBinding(request.Snapshot)
	if err != nil || !found || request.Input.Stop == nil || request.Input.Stop.TurnID != request.Snapshot.Session.CurrentTurnID {
		return receipt, nil, errors.Join(err, errors.New("original stop observation binding or turn cannot be verified"))
	}
	if err := ctx.Err(); err != nil {
		return receipt, nil, err
	}
	if l.writersAbsent(binding) {
		receipt.Disposition = executionbackend.OperationConfirmed
		receipt.State = executionbackend.LifecycleStateFacts{SessionAvailable: true, WorkerAbsent: true, WritersAbsent: true, Stopped: true}
		receipt.Diagnostic = "original stop target verified by managed writer-tree exit; interrupt RPC outcome remains historical"
	}
	return receipt, nil, nil
}
