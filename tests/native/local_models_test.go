//go:build windows && live

package native

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"testing"
)

func auditManagedLocalOrcaModels(t *testing.T, m livetest.Manifest, view contracts.RunView) {
	t.Helper()
	record, run, err := originalStructuredRecord(m, view)
	if err != nil {
		t.Fatal(err)
	}
	if m.Execution.Target.Kind != "local" || view.Audit == nil || m.AuditModel == nil {
		t.Fatal("original structured audited model proof missing")
	}
	writer, err := contracts.ExecutionModel(m.Execution)
	if err != nil {
		t.Fatal(err)
	}
	byCommand := map[string]livetest.StructuredBinding{}
	for _, binding := range run.Inputs {
		byCommand[binding.CommandID] = binding
	}
	verify := func(command, step, model, effort string) any {
		binding, ok := byCommand[command]
		if !ok {
			t.Fatal("original model input missing", command)
		}
		matches := 0
		var fact any
		for _, result := range record.Workflow.StageResults {
			if result.CommandID != command {
				continue
			}
			matches++
			proof := result.ActualModel
			if proof == nil || !result.Succeeded || !result.FinalVerified || result.StepID != step || result.TurnID != binding.TurnID || proof.Model != model || proof.ReasoningEffort != effort || proof.Turn != binding.TurnID || proof.ProviderSession == "" || len(proof.SourceDigest) != 64 {
				t.Fatal("actual original native model differs from frozen workflow", step)
			}
			fact = map[string]any{"step": step, "command_id": command, "turn_id": binding.TurnID, "native_turn_id": binding.TurnID, "source": "orca.structured.original-model.v1", "model": proof.Model, "effort": proof.ReasoningEffort, "source_digest": proof.SourceDigest, "provider_session": proof.ProviderSession}
		}
		if matches != 1 {
			t.Fatal("original model result missing or ambiguous", step)
		}
		return fact
	}
	var facts []any
	for _, audit := range view.Audit.Results {
		if audit.Failure != "" {
			t.Fatal("audit failed")
		}
		facts = append(facts, verify(audit.CommandID, audit.StepID, m.AuditModel.Model, m.AuditModel.ReasoningEffort))
	}
	final := run.Binding
	var step string
	for _, result := range record.Workflow.StageResults {
		if result.CommandID == final.CommandID {
			step = result.StepID
		}
	}
	effort := writer.ReasoningEffort
	if effort == "" {
		effort = "medium"
	}
	facts = append(facts, verify(final.CommandID, step, writer.Model, effort))
	save(t, "audit-models.json", map[string]any{"run_id": view.RunID, "session": run.Binding.SessionID, "turns": facts, "configuration_digest": record.Snapshot.ConfigurationDigest, "passed": true, "protocol": "orca_agent_session_v1"})
}
