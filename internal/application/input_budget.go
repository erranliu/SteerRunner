package application

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func priorFileInstruction(r *runstate.Record, source string) bool {
	for _, i := range r.Workflow.StageInputs {
		if i.Budget == nil {
			continue
		}
		result := workflow.Result(r.Workflow, i.CommandID)
		if result == nil || result.SessionID == "" || result.SessionID != r.Snapshot.Session.SessionID || !result.FinalVerified {
			continue
		}
		for _, p := range i.Budget.Fragments {
			if p.Source == source {
				return true
			}
		}
	}
	return false
}

// Validate the original frozen budget; never infer ownership from saved text.
func ensureStageBudget(r *runstate.Record, i *workflow.StageInput) error {
	if i.Budget == nil {
		return errors.New("frozen input budget unavailable; candidate not sent")
	}
	parts, err := m.InputParts(i.Prompt, i.Budget)
	if err != nil {
		return err
	}
	if r.Request.Execution.Adapter == "orca-terminal" && r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
		parts = m.NativeInputParts(parts)
	}
	_, _, err = m.FreezeInput(parts...)
	return err
}
