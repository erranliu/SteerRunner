package application

import (
	"context"
	"errors"
	"reflect"
	"time"
	"unicode/utf8"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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
	// Only this scope's own commits advance the expected Run revision.
	ctx = b.WithPreparationProgress(ctx, func(name string, update func(*b.PreparationQuery) error) (*b.PreparationQuery, error) {
		w.gate.Lock()
		defer w.gate.Unlock()
		e.mu.Lock()
		defer e.mu.Unlock()
		current := e.runs[id]
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		if !current.Allowed || current.ResumeRequired || current.PendingStop != "" {
			return nil, context.Canceled
		}
		if current.Revision != revision {
			return nil, reliable.ErrStale
		}
		if len(current.Intents) == 0 {
			return nil, errors.New("preparation query has no original operation")
		}
		next := current.Clone()
		intent := &next.Intents[len(next.Intents)-1]
		if intent.Request.Operation.Action != string(b.LifecyclePrepareWorkspace) {
			return nil, errors.New("query is outside workspace preparation")
		}
		prior := intent.PreparationQuery
		q := &b.PreparationQuery{}
		if prior != nil {
			if time.Now().UTC().Before(prior.Wait.UpdatedAt.Add(-time.Second)) {
				return nil, errors.New("preparation query clock moved backwards; automatic dispatch stopped")
			}
			*q = *prior
		}
		if q.Wait.Query != "" && q.Wait.Query != name {
			return nil, reliable.ErrStale
		}
		if err := update(q); err != nil {
			return nil, err
		}
		if ctx.Err() != nil && q.Wait.Stage != "exhausted" {
			return nil, ctx.Err()
		}
		q.Wait.OperationID = intent.Request.Operation.ID
		if !next.PreparationDeadline.IsZero() && next.PreparationDeadline.Before(q.Wait.DeadlineAt) {
			q.Wait.DeadlineAt = next.PreparationDeadline
		}
		q.Wait.Diagnostic = e.redact(next, q.Wait.Diagnostic)
		if len(q.Wait.Diagnostic) > 1024 {
			q.Wait.Diagnostic = q.Wait.Diagnostic[:1024]
		}
		for !utf8.ValidString(q.Wait.Diagnostic) {
			q.Wait.Diagnostic = q.Wait.Diagnostic[:len(q.Wait.Diagnostic)-1]
		}
		if err := q.Validate(intent.Request.Operation); err != nil {
			return nil, err
		}
		if reflect.DeepEqual(prior, q) {
			return q, nil
		}
		q.Wait.UpdatedAt = time.Now().UTC()
		intent.PreparationQuery = q
		if q.Visible {
			next.Phase = "execution_wait"
		}
		if err := e.commitLocked(next, nil, "execution_wait_changed"); err != nil {
			return nil, err
		}
		revision = next.Revision
		copy := *q
		return &copy, nil
	})
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
	// Merge into the current aggregate; clearing the public projection must not
	// discard the original budget, accepted stop or other concurrent facts.
	if len(current.Intents) > 0 {
		next := current.Clone()
		q := next.Intents[len(next.Intents)-1].PreparationQuery
		if q != nil && q.Visible && (q.Wait.Stage != "exhausted" || revoked) {
			q.Visible = false
			if next.Phase == "execution_wait" {
				next.Phase = next.Intents[len(next.Intents)-1].Request.Operation.Action
			}
			if commitErr := e.commitLocked(next, nil, "execution_wait_changed"); commitErr != nil {
				err = errors.Join(err, commitErr)
			}
		}
	}
	e.mu.Unlock()
	if revoked {
		return errors.Join(err, context.Canceled)
	}
	if changed {
		return errors.Join(err, reliable.ErrStale)
	}
	return errors.Join(err, ctx.Err())
}
