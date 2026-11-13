package application

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func (e *Engine) admitSubmit(input contracts.Request, digest string) contracts.Response {
	unknown := func() contracts.Response {
		return contracts.Rejected(input.RequestID, contracts.Fail(6, "request_outcome_unknown", "Submission commit could not be verified; inspect the original request_id."))
	}
	receipt, err := e.store.BeginSubmit(input, digest)
	if err != nil {
		return unknown()
	}
	reject := func(err error) contracts.Response {
		response := contracts.Rejected(input.RequestID, fault(err))
		if e.store.RejectSubmit(receipt, response) != nil {
			return unknown()
		}
		return response
	}

	var request contracts.RunRequest
	if err := contracts.DecodeStrict(input.Input, &request); err != nil {
		return reject(err)
	}
	var source *runstate.Record
	if request.ReuseWorktreeFromRunID != "" {
		var unlock func()
		source, unlock, err = e.reusableSource(request.ReuseWorktreeFromRunID)
		if err != nil {
			return reject(err)
		}
		defer unlock()
	}
	var r *runstate.Record
	if request.CheckoutOperation() {
		r, err = freezeEnvironmentCheck(request)
	} else {
		r, err = freezeWithWorkspace(e.ctx, e.dir, input, source)
	}
	if err != nil {
		return reject(err)
	}
	receipt.AcceptSubmit(r.ID)
	r.SubmitCommandID = receipt.ID
	if workflow.Content(r.Request.Workflow.ID) && len(r.History) > 0 {
		r.History[0].RequestCommandID = receipt.ID
	}
	r.CommandStates[receipt.ID] = "pending"

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctx.Err() != nil {
		return reject(contracts.Fail(4, "service_unavailable", "Service is stopping; submission was not accepted."))
	}
	if source != nil && (e.runs[source.ID].Revision != source.Revision || e.workspaceSuperseded(source)) {
		return reject(contracts.Fail(5, "workspace_already_reused", "Source Run changed or its workspace already has a successor."))
	}
	events := []string{"run_accepted"}
	if source != nil {
		events = append(events, "worktree_reused")
	}
	if err := e.commitLocked(r, &receipt, events...); err != nil {
		// A failed Run transaction can be rejected only if the original
		// admission row still has no Run and remains in preparation.
		return reject(err)
	}
	w := &worker{wake: make(chan struct{}, 1)}
	e.workers[r.ID] = w
	e.launch(r.ID, w)
	return receipt.Response
}
