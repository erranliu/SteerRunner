package orcatransport

import (
	"context"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// VerifyStopped constructs only a frozen reader, never an observer or execution.
func VerifyStopped(ctx context.Context, setup b.LifecycleSetup) error {
	l, err := newFrozenReader(setup)
	if err != nil {
		return err
	}
	if l.structured() {
		if err := l.validateStructured(setup.Snapshot); err != nil {
			return err
		}
		return l.verifyAgentSessionStopped(setup.Snapshot)
	}
	return l.verifyStopped(ctx, setup.Snapshot)
}
