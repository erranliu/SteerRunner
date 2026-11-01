package application

import (
	"context"
	"errors"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// Called under the worker gate. Callers retain their distinct input/source
// checks, diagnostics, authorization and atomic business/command commit.
func (e *Engine) qualifyContinuation(r *runstate.Record, w *worker, p b.LifecyclePort, needsInput bool, unavailable func(b.LifecycleRecoveryEvidence, error) error) error {
	return e.qualifyContinuationContext(e.ctx, r, w, p, needsInput, unavailable)
}
func (e *Engine) qualifyContinuationContext(parent context.Context, r *runstate.Record, w *worker, p b.LifecyclePort, needsInput bool, unavailable func(b.LifecycleRecoveryEvidence, error) error) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	facts, readErr := p.VerifyManualRecovery(ctx, r.Snapshot.Clone())
	readErr = errors.Join(readErr, ctx.Err())
	cancel()
	var err error
	if !reliable.CanContinueOriginal(facts) || needsInput && !facts.SessionIdle {
		err = reliable.ErrContinuationUnavailable
	}
	if readErr != nil || errors.Is(err, reliable.ErrContinuationUnavailable) {
		failure := unavailable(facts, readErr)
		if failure != nil && (readErr != nil || !reliable.CanContinueOriginal(facts)) {
			e.requireManual(r.ID, failure.Error())
		}
		return failure
	}
	if err != nil {
		return err
	}
	return nil
}
