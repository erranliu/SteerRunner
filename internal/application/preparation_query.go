package application

import (
	"context"
	"errors"

	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

// Called with the worker's dispatch gate held. Only read-only preparation may
// release it; mutations still serialize with the committed stop authorization.
func (e *Engine) preparationQuery(id string, w *worker, ctx context.Context, query func(context.Context) error) error {
	e.mu.Lock()
	original := e.runs[id]
	revision := original.Revision
	revoked := !original.Allowed || original.ResumeRequired || original.PendingStop != ""
	e.mu.Unlock()
	if revoked {
		return context.Canceled
	}
	ctx, cancel := context.WithCancel(ctx)
	w.queryCancel = cancel
	defer func() {
		cancel()
		w.queryCancel = nil
	}()
	err := func() error {
		w.gate.Unlock()
		defer w.gate.Lock()
		return query(ctx)
	}()
	// Reacquire the gate before checking state and before returning to any
	// mutation. A successful query that raced with stop grants no authority.
	e.mu.Lock()
	current := e.runs[id]
	revoked = !current.Allowed || current.ResumeRequired || current.PendingStop != ""
	changed := current.Revision != revision
	e.mu.Unlock()
	if revoked {
		return errors.Join(err, context.Canceled)
	}
	if changed {
		return errors.Join(err, reliable.ErrStale)
	}
	return errors.Join(err, ctx.Err())
}
