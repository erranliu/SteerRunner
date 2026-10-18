package application

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"slices"
)

// Legacy development records deliberately retain their original frozen inputs.
func validateWorkflow(r *runstate.Record) error {
	p, ok := workflow.Lookup(r.Request.Workflow.ID, r.Request.Workflow.Version)
	if !ok {
		return errors.New("unsupported frozen workflow")
	}
	w := r.Workflow
	if !p.AuditModelRequired {
		if w.Config != nil || w.AuditModel != nil || workflow.AuditStep(w.StepID) {
			return errors.New("ordinary workflow contains audit state")
		}
		return nil
	}
	if w.Config == nil || w.Config.ID != p.ID || w.Config.Version != p.Version || !slices.Equal(w.Config.AuditSteps, p.AuditSteps) || w.Config.MaximumRepairCycles != p.MaximumRepairCycles || w.ExecutionModel == nil || w.AuditModel == nil || w.ExecutionModel.Model == "" || w.AuditModel.Model == "" || w.ExecutionModel.ReasoningEffort == "" || w.AuditModel.ReasoningEffort == "" || !slices.Contains(p.SupportedAdapters, r.Request.Execution.Adapter) {
		return errors.New("audited workflow lacks its supported frozen configuration or models")
	}
	return nil
}
