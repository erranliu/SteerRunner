package application

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"slices"
)

func validateOperationVersions(r *runstate.Record) error {
	if r.Snapshot.Ref.Version != 1 {
		return errors.New("original execution record version is unsupported; preserve it without dispatch")
	}
	for _, intent := range r.Intents {
		if intent.Request.Operation.Version != 1 {
			return errors.New("original lifecycle operation version is unsupported; preserve it without dispatch")
		}
	}
	for _, op := range r.Snapshot.Operations {
		if op.Version != 1 {
			return errors.New("original execution operation version is unsupported; preserve it without dispatch")
		}
	}
	return nil
}

func validateWorkflow(r *runstate.Record) error {
	if err := validateOperationVersions(r); err != nil {
		return err
	}
	for _, intent := range r.Intents {
		if receipt := intent.Receipt; receipt != nil && receipt.ParseEvidence != nil && receipt.ParseEvidence.Status == "side_effect_unverified" {
			return errors.New("original parser tool side effects require reconciliation; resume cannot authorize a successor")
		}
	}
	p, ok := workflow.Lookup(r.Request.Workflow.ID, r.Request.Workflow.Version)
	if !ok || r.Workflow.Config == nil || !compatibleWorkflow(p, *r.Workflow.Config, r.Request.Execution.Adapter) {
		return errors.New("unsupported or inconsistent frozen workflow definition")
	}
	if _, ok := r.Workflow.CurrentState(); !ok {
		return errors.New("business cursor absent from frozen definition")
	}
	if state, _ := r.Workflow.CurrentState(); state.Kind == "parse" {
		if err := validateParserReference(r); err != nil {
			return err
		}
	}
	if r.NextStage != nil && (r.Snapshot.PendingTurn != nil || r.NextStage.StepID != r.Workflow.StepID) {
		return errors.New("pending prompt conflicts with business cursor")
	}
	if r.Snapshot.PendingTurn != nil && r.Snapshot.PendingTurn.StepID != "" && r.Snapshot.PendingTurn.StepID != r.Workflow.StepID {
		return errors.New("native prompt conflicts with business cursor")
	}
	if native := r.Snapshot.PendingTurn; native != nil && native.StepID != "" {
		frozen := workflow.Input(r.Workflow, native.CommandID)
		if frozen == nil || frozen.StepID != native.StepID || frozen.Version != native.InputVersion || frozen.Prompt != native.Prompt {
			return errors.New("native prompt differs from frozen business input")
		}
	}
	if p.AuditModelRequired && (r.Workflow.ExecutionModel == nil || r.Workflow.AuditModel == nil || r.Workflow.ExecutionModel.Model == "" || r.Workflow.AuditModel.Model == "" || !slices.Contains(p.SupportedAdapters, r.Request.Execution.Adapter)) {
		return errors.New("audited workflow lacks frozen model configuration")
	}

	return nil
}

// Descriptions and admission defaults govern new Runs. Existing Runs keep
// their original execution definition, prompts, models and adapter.
func compatibleWorkflow(current, frozen workflow.Profile, adapter string) bool {
	if current.ID != frozen.ID || current.Version != frozen.Version || current.Initial != frozen.Initial || current.AuditModelRequired != frozen.AuditModelRequired || !slices.Contains(current.SupportedAdapters, adapter) || !slices.Contains(frozen.SupportedAdapters, adapter) || len(current.States) != len(frozen.States) {
		return false
	}
	for id, original := range frozen.States {
		candidate, found := current.States[id]
		if !found {
			return false
		}
		// Default text is already frozen in the original definition/PromptBodies.
		// Keep routing, contracts, permissions, model roles and budgets exact.
		candidate.Prompt, original.Prompt = "", ""
		if !reflect.DeepEqual(candidate, original) {
			return false
		}
	}
	return true
}
