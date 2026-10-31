package codexworker

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type preparationError struct{ error }

func preparationBlocked(err error) error { return &preparationError{err} }
func isPreparationBlocked(err error) bool {
	var target *preparationError
	return errors.As(err, &target)
}
func (backend Backend) verifySandboxReadiness(ctx context.Context, h *SessionHandle, input b.TurnInput) error {
	if backend.Config.Sandbox == "read-only" {
		return backend.verifyContentReadiness(ctx, h, input)
	}
	return nil
}
