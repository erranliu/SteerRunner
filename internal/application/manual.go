package application

import (
	"fmt"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

const manualIntervention = "manual_intervention_required"

func reconciliationDeadline(r *runstate.Record, intent operation.Intent) time.Time {
	if intent.PreparationQuery != nil {
		return intent.PreparationQuery.Wait.DeadlineAt
	}
	if (intent.Request.Operation.Action == string(b.LifecyclePrepareWorkspace) || intent.Request.Operation.Action == string(b.LifecycleCreateResource) || intent.Request.Operation.Action == string(b.LifecyclePrepareSession)) && !r.PreparationDeadline.IsZero() {
		return r.PreparationDeadline
	}
	return intent.Request.Operation.StartedAt.Add(30 * time.Second)
}

func (e *Engine) requireManual(id, diagnostic string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if r.Closed || r.PendingStop != "" || r.StopStatus == "stopped" {
		return
	}
	if r.ManualPause == nil {
		r.ManualPause = &runstate.ManualPause{Status: r.Status, Phase: r.Phase, Diagnostic: r.Diagnostic}
	}
	r.Allowed, r.ResumeRequired = false, true
	r.Status, r.Phase, r.Diagnostic = "blocked", manualIntervention, e.redact(r, diagnostic)
	_ = e.commitLocked(r, nil, "manual_intervention_required")
}

// The accepted resume owns one finite read-only scope. It can only query saved
// operations; restarting the service may repeat these reads, never their effects.
func (e *Engine) reconcileManual(id string, w *worker, p b.LifecyclePort, command string, deadline time.Time) bool {
	for {
		e.mu.Lock()
		r := e.runs[id].Clone()
		e.mu.Unlock()
		if r.PendingResume != command || r.PendingStop != "" || r.Closed {
			return false
		}
		index := -1
		for i, intent := range r.Intents {
			if intent.NeedsReconciliation() {
				index = i
				break
			}
		}
		if index < 0 {
			return true
		}
		e.invoke(id, w, p, index, true, deadline)
		e.mu.Lock()
		next := e.runs[id].Clone()
		e.mu.Unlock()
		if next.PendingResume != command || next.PendingStop != "" || next.Closed || e.ctx.Err() != nil {
			return false
		}
		if next.Intents[index].NeedsReconciliation() || next.Intents[index].Request.Operation.Status == b.OperationRejected {
			op := next.Intents[index].Request.Operation
			e.failControl(id, command, fmt.Errorf("Original %s operation %s remains %s: %s; retry resume with a new request_id and current revision", op.Action, op.ID, op.Status, op.Diagnostic))
			return false
		}
	}
}
