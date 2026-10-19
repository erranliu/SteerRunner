package application

import (
	"context"
	"errors"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// advance retains the dispatch gate and checks authorization, policy and
// higher-priority controls before preparing or dispatching the next input.
func (e *Engine) prepareRun(id string, w *worker, p b.LifecyclePort, r *runstate.Record) bool {
	if r.Snapshot.Workspace.Path != "" && r.Snapshot.PendingTurn == nil && len(r.Workflow.StageInputs) == 0 {
		var materialErr error
		if !workflow.Content(r.Request.Workflow.ID) {
			materialErr = materialize(r)
		}
		if err := materialErr; err != nil {
			e.block(id, "materialization_failed", err, false)
			return false
		}
		if err := queueInput(r, workflow.StageInput{StepID: r.Workflow.Config.Initial}, ""); err != nil {
			e.block(id, "input_failed", err, false)
			return false
		}
		if err := e.attachWriteRoots(r); err != nil {
			e.block(id, "workspace_unverified", err, false)
			return false
		}
		e.mu.Lock()
		err := e.commitLocked(r, nil, "input_frozen")
		e.mu.Unlock()
		return err == nil
	}
	if r.Snapshot.PendingTurn == nil && r.Snapshot.Workspace.Path != "" {
		return false
	}
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	candidate, err := reliable.Next(ctx, p, r.Snapshot.Clone())
	cancel()
	if err != nil {
		e.block(id, "preparation_blocked", err, false)
		return false
	}
	if candidate.Done {
		return false
	}
	if r.PreparationDeadline.IsZero() {
		r.PreparationDeadline = time.Now().UTC().Add(3 * time.Minute)
	}
	if time.Now().After(r.PreparationDeadline) && candidate.Action != b.LifecycleStartTurn && candidate.Action != b.LifecycleSubmitInput {
		e.block(id, "preparation_deadline", errors.New("The original preparation budget is exhausted"), false)
		return false
	}
	if nativeInputAction(candidate.Action) && r.Snapshot.PendingTurn != nil {
		input := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
		if input == nil {
			return false
		}
		w.gate.Unlock()
		checkErr := e.checkDispatchSource(r, *input)
		w.gate.Lock()
		e.mu.Lock()
		stale := e.runs[id].Revision != r.Revision || !e.runs[id].Allowed || e.runs[id].PendingStop != ""
		e.mu.Unlock()
		if stale {
			return false
		}
		if checkErr != nil {
			e.blockRevision(id, r.Revision, "source_unverified", checkErr, false)
			return false
		}
	}
	i, err := e.saveIntent(id, r, candidate)
	if err != nil {
		return false
	}
	return e.invoke(id, w, p, i, false)
}
