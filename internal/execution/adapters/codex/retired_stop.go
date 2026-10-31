package codexworker

import (
	"context"
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// A retired execution can address only its previously bound owner. There is
// no profile normalization, native launch or authentication preparation here.
func NewOriginalOwnerStopLifecycle(setup b.LifecycleSetup) b.LifecyclePort {
	return &originalOwnerStop{ExecutionLifecycle: ExecutionLifecycle{
		setup:   setup.Clone(),
		Backend: Backend{Process: ProcessPort{Inspect: process.InspectProcess}},
	}}
}

type originalOwnerStop struct{ ExecutionLifecycle }

func (*originalOwnerStop) Execute(_ context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	err := errors.New("retired execution only supports an original owner stop")
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected, Diagnostic: err.Error()}, nil, err
}

func (l *originalOwnerStop) StopLifecycle(ctx context.Context, r b.LifecycleRequest, _ b.StopHandle) (b.LifecycleReceipt, error) {
	if r.Operation.Action != string(b.LifecycleStopExecution) || ExecutionOwner(r.Snapshot) == nil {
		return b.LifecycleReceipt{}, errors.New("original independent owner stop is unsupported; use the original build to settle resources")
	}
	if err := r.Validate(); err != nil {
		return b.LifecycleReceipt{}, err
	}
	return l.ownerCall(ctx, r, false)
}

func (l *originalOwnerStop) Reconcile(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if r.Operation.Action != string(b.LifecycleStopExecution) || ExecutionOwner(r.Snapshot) == nil {
		return b.LifecycleReceipt{}, nil, errors.New("retired execution only reconciles an original owner stop")
	}
	if err := r.Validate(); err != nil {
		return b.LifecycleReceipt{}, nil, err
	}
	receipt, err := l.ownerCall(ctx, r, true)
	return receipt, nil, err
}

func (*originalOwnerStop) ShutdownLifecycle(context.Context, b.LifecycleSnapshot, b.StopHandle) (b.LifecycleStateFacts, error) {
	return b.LifecycleStateFacts{}, nil
}
