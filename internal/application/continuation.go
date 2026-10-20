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
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	facts, readErr := p.VerifyManualRecovery(ctx, r.Snapshot.Clone())
	cancel()
	restore, err := reliable.PlanContinuation(facts, r.Snapshot.Session.SessionID, unknown(r), needsInput)
	if readErr != nil || errors.Is(err, reliable.ErrContinuationUnavailable) {
		return unavailable(facts, readErr)
	}
	if err != nil {
		return err
	}
	if restore {
		return e.restoreOriginalAttempt(r, w, p)
	}
	return nil
}
