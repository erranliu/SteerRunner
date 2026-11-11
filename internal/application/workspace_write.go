package application

import (
	"context"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// The caller holds the dispatch gate. The actor lock is released for these
// read-only checks and the caller rechecks the revision before committing.
func (e *Engine) qualifyWorkspaceWrite(r *runstate.Record, w *worker) *contracts.Fault {
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	defer cancel()
	if err := verifyRetainedWorkspace(ctx, r); err != nil {
		return contracts.Fail(5, "workspace_identity_conflict", "The original managed worktree identity could not be verified.")
	}
	p, err := e.port(r, w)
	if err != nil {
		return contracts.Fail(4, "environment_unavailable", "The original native session configuration is unavailable.")
	}
	facts, err := p.VerifyManualRecovery(ctx, r.Snapshot.Clone())
	if err != nil || ctx.Err() != nil || !reliable.CanContinueOriginal(facts) || !facts.SessionIdle {
		return contracts.Fail(5, "operation_unverified", "The original native session could not be verified idle and available.")
	}
	return nil
}
