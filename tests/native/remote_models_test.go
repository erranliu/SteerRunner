//go:build windows && live

package native

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"path/filepath"
	"testing"
)

func auditRemoteModels(t *testing.T, m livetest.Manifest, view contracts.RunView) {
	t.Helper()
	store, err := storage.OpenReader(filepath.Join(m.Root, "s"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runs, err := store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	var state *orca.State
	for _, run := range runs {
		if run.ID != view.RunID {
			continue
		}
		for _, e := range run.Snapshot.Implementation {
			if e.ImplementationKey == orca.StateKey {
				var s orca.State
				if json.Unmarshal(e.Payload, &s) != nil {
					t.Fatal("invalid native state")
				}
				state = &s
			}
		}
	}
	if state == nil || state.RemoteReadiness == nil || orca.SessionID(state.Process) != view.Execution.SessionID || state.RemoteReadiness.ConfigurationDigest != livetest.ReceiptAuditPolicy(m).ConfigurationDigest {
		t.Fatal("remote model audit lost original frozen execution")
	}
	writer, err := contracts.ExecutionModel(m.Execution)
	if err != nil {
		t.Fatal(err)
	}
	byCommand := map[string]*orca.Turn{}
	for i := range state.Turns {
		turn := &state.Turns[i]
		if byCommand[turn.Command] != nil {
			t.Fatal("duplicate native command")
		}
		byCommand[turn.Command] = turn
	}
	var facts []any
	verify := func(command, step string, want workflow.ModelSettings) {
		turn := byCommand[command]
		if turn == nil || !turn.Accepted || !turn.Started || !turn.Settled || turn.StepID != step || turn.Inspection == nil || !turn.Inspection.Closed || turn.Inspection.Unknown || orca.ValidateRecordedTurn(*state, turn) != nil {
			t.Fatal("remote model proof lacks a settled original native turn", step)
		}
		r := turn.Recorded
		if r.Model != want.Model || r.ReasoningEffort != want.ReasoningEffort || !r.Completed || r.Status != "completed" {
			t.Fatal("actual native model differs from workflow", step)
		}
		facts = append(facts, map[string]any{"step": step, "command_id": turn.Command, "turn_id": turn.Command, "native_turn_id": r.NativeTurnID, "model": r.Model, "effort": r.ReasoningEffort, "source": r.Source, "session": r.Readiness.SessionID, "final_digest": r.FinalDigest, "inspector_closed": turn.Inspection.Closed})
	}
	for _, audit := range view.Audit.Results {
		if audit.Verdict != "pass" || audit.CommandID != audit.TurnID {
			t.Fatal("audit outer turn binding changed")
		}
		verify(audit.CommandID, audit.StepID, *m.AuditModel)
	}
	final := byCommand[view.Execution.TurnID]
	if final == nil {
		t.Fatal("publication lost original native model")
	}
	verify(final.Command, final.StepID, writer)
	save(t, "audit-models.json", map[string]any{"run_id": view.RunID, "session": view.Execution.SessionID, "provider_session": state.RemoteReadiness.SessionID, "configuration_digest": state.RemoteReadiness.ConfigurationDigest, "turns": facts, "passed": true})
}
