package application

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"time"
)

func (e *Engine) inspectSource(r *runstate.Record, preHead string) *workflow.WorkspaceSource {
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()
	var f b.WorkspaceInspection
	var err error
	if r.Request.Execution.Adapter == "orca-terminal" {
		w := e.workers[r.ID]
		if w == nil {
			err = errors.New("original remote workspace owner unavailable")
		} else {
			var port b.LifecyclePort
			port, err = e.port(r, w)
			if err == nil {
				if owner, ok := port.(b.LifecycleAuditWorkspacePort); ok {
					f, err = owner.InspectLifecycleAuditWorkspace(ctx, r.Snapshot, preHead)
				} else {
					err = errors.New("original remote audit inspection unsupported")
				}
			}
		}
	} else {
		f, err = workspace.InspectAudit(ctx, gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, &r.Snapshot.WorkspacePlan, preHead)
	}
	result := &workflow.WorkspaceSource{Repository: f.Repository, Worktree: f.Worktree, Branch: f.Branch, Base: f.Base, Head: f.Head, Tree: f.Tree, Dirty: f.Dirty, Qualified: f.Qualified}
	if err != nil {
		result.Qualified = false
		result.Diagnostic = e.redact(r, err.Error())
	}
	return result
}

func (e *Engine) originalCheckpoint(r *runstate.Record, w *worker, input workflow.StageInput) *workflow.WorkspaceSource {
	if prior := w.checkpoints[input.CommandID]; prior != nil {
		copy := *prior
		return &copy
	}
	if !w.ownedInputs[input.CommandID] || unknown(r) {
		return &workflow.WorkspaceSource{Diagnostic: "original terminal checkpoint was not observed in this service; current HEAD cannot recreate it"}
	}
	preHead := ""
	state, _ := r.Workflow.State(input.StepID)
	if state.Contract == "audit" {
		if implementation := workflow.Acquired(r.Workflow, input.ImplementationResultID); implementation != nil {
			if pre := workflow.Result(r.Workflow, implementation.Source.CommandID); pre != nil && pre.Workspace != nil {
				preHead = pre.Workspace.Head
			}
		}
	}
	fact := e.inspectSource(r, preHead)
	if w.checkpoints == nil {
		w.checkpoints = map[string]*workflow.WorkspaceSource{}
	}
	w.checkpoints[input.CommandID] = fact
	return fact
}

func checkpoint(r *runstate.Record, id string) *workflow.WorkspaceSource {
	a := workflow.Acquired(r.Workflow, id)
	if a == nil {
		return nil
	}
	s := workflow.Result(r.Workflow, a.Source.CommandID)
	if s == nil {
		return nil
	}
	return s.Workspace
}
func matchesCheckpoint(f, want *workflow.WorkspaceSource) bool {
	return f != nil && want != nil && f.Qualified && want.Qualified && !f.Dirty && !want.Dirty && f.Repository == want.Repository && f.Worktree == want.Worktree && f.Branch == want.Branch && f.Base == want.Base && f.Head == want.Head && f.Tree == want.Tree
}

func (e *Engine) checkDispatchSource(r *runstate.Record, input workflow.StageInput) error {
	state, ok := r.Workflow.State(input.StepID)
	if !ok {
		return errors.New("unknown state")
	}
	if err := workflow.VerifyPrerequisites(r.Workflow, input, state); err != nil {
		return err
	}
	if state.Workspace == "" {
		return nil
	}
	var want *workflow.WorkspaceSource
	if state.Kind == "report" {
		if original := workflow.Result(r.Workflow, input.SourceCommandID); original != nil {
			want = original.Workspace
		}
		// Only a non-done report can be accepted without an original checkpoint.
		if want == nil || !want.Qualified {
			return nil
		}
	} else if state.Contract == "audit" {
		want = checkpoint(r, input.ImplementationResultID)
	} else if state.Contract == "publication" {
		want = checkpoint(r, input.AuditResultID)
	} else {
		return nil
	}
	if !matchesCheckpoint(e.inspectSource(r, ""), want) {
		return errors.New("workspace or source differs from frozen completion checkpoint")
	}
	return nil
}

func (e *Engine) checkPublicationSource(r *runstate.Record) error {
	if !r.Workflow.RequiresWorkspace() {
		return nil
	}
	if !workflow.AuditsPassed(r.Workflow) || !matchesCheckpoint(e.inspectSource(r, ""), checkpoint(r, r.Workflow.AuditResultID)) {
		return errors.New("publication no longer matches audited source")
	}
	return nil
}

// Keep native input dispatch qualification distinct from model reporting.
func nativeInputAction(a b.LifecycleAction) bool {
	return a == b.LifecycleStartTurn || a == b.LifecycleSubmitInput
}
