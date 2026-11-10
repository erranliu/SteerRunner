package application

import (
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func initialRejectedRun(t *testing.T, action b.LifecycleAction, status string) *runstate.Record {
	t.Helper()
	r := &runstate.Record{ID: "run", SubmitCommandID: "submit", Closed: true, CommandStates: map[string]string{"submit": status, "other": "completed"}, Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: workflow.SingleExecution, Version: 1}}}
	r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1, Backend: b.BackendLocal, Protocol: b.ProtocolLocalCodexAppServer, RunID: r.ID, AttemptID: "attempt"}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: r.ID, ExecutionID: r.ID, AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
	intent, err := operation.New(r.ID, r.Snapshot, b.LifecycleCandidate{Action: action, Input: b.LifecycleInput{Turn: &b.TurnInput{CommandID: "input", Prompt: "original input"}}}, "input-operation", "original-key", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	intent.Request.Operation.Status = b.OperationRejected
	intent.Receipt = &b.LifecycleReceipt{Operation: intent.Request.Operation, Disposition: b.OperationRejected}
	if err := intent.Receipt.ValidateFor(intent.Request); err != nil {
		t.Fatal("fixture rejection is not bound to the original request", err)
	}
	r.Intents = []operation.Intent{intent}
	return r
}

func TestStopSettlementFailsOnlyProvenUnacceptedSubmit(t *testing.T) {
	for _, action := range []b.LifecycleAction{b.LifecycleStartTurn, b.LifecycleSubmitInput} {
		for _, status := range []string{"pending", "unknown"} {
			t.Run(string(action)+"/"+status, func(t *testing.T) {
				r := initialRejectedRun(t, action, status)
				before := string(canonical(r.Intents))
				settleRunStop(r)
				if r.CommandStates["submit"] != "failed" || r.CommandStates["other"] != "completed" || string(canonical(r.Intents)) != before || r.StopStatus != "stopped" {
					t.Fatal("confirmed rejection was not settled independently of its immutable intent", r)
				}
			})
		}
	}
	for _, status := range []string{"pending", "unknown"} {
		r := initialRejectedRun(t, b.LifecycleStartTurn, status)
		r.Intents = nil // Revoked before queueInput/intent commit.
		settleRunStop(r)
		if r.CommandStates["submit"] != "failed" {
			t.Fatal("cold stop left the never-dispatched submit unresolved", r)
		}
	}
}

func TestStopSettlementPreservesUncertainOrContradictorySubmit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*runstate.Record)
	}{
		{"in-flight", func(r *runstate.Record) { r.Intents[0].Request.Operation.Status = b.OperationInFlight }},
		{"unknown", func(r *runstate.Record) { r.Intents[0].Request.Operation.Status = b.OperationUnknown }},
		{"accepted", func(r *runstate.Record) { r.Intents[0].Request.Operation.Status = b.OperationAccepted }},
		{"confirmed", func(r *runstate.Record) { r.Intents[0].Request.Operation.Status = b.OperationConfirmed }},
		{"missing-receipt", func(r *runstate.Record) { r.Intents[0].Receipt = nil }},
		{"foreign-receipt", func(r *runstate.Record) { r.Intents[0].Receipt.Operation.ID = "other-operation" }},
		{"unknown-receipt", func(r *runstate.Record) { r.Intents[0].Receipt.Disposition = b.OperationUnknown }},
		{"accepted-state", func(r *runstate.Record) { r.Intents[0].Receipt.State.InputAccepted = true }},
		{"started-state", func(r *runstate.Record) { r.Intents[0].Receipt.State.TurnStarted = true }},
		{"completed-state", func(r *runstate.Record) { r.Intents[0].Receipt.State.TurnCompleted = true }},
		{"receipt-turn", func(r *runstate.Record) { r.Intents[0].Receipt.State.TurnID = "possibly-started-turn" }},
		{"turn-evidence", func(r *runstate.Record) { r.Intents[0].Receipt.TurnEvidence = &b.TurnEvidence{} }},
		{"native-started", func(r *runstate.Record) { r.NativeStarted = true }},
		{"current-turn", func(r *runstate.Record) { r.Snapshot.Session.CurrentTurnID = "current-turn" }},
		{"pending-turn", func(r *runstate.Record) { r.Snapshot.Session.PendingTurnID = "pending-turn" }},
		{"other-unknown-operation", func(r *runstate.Record) {
			r.Intents = append(r.Intents, operation.Intent{Request: b.LifecycleRequest{Operation: b.Operation{Action: string(b.LifecyclePrepareSession), Status: b.OperationUnknown}}})
		}},
		{"another-accepted-input", func(r *runstate.Record) {
			other := r.Intents[0]
			other.Request.Operation.Status = b.OperationAccepted
			r.Intents = append(r.Intents, other)
		}},
	} {
		for _, status := range []string{"pending", "unknown", "completed", "failed"} {
			t.Run(tc.name+"/"+status, func(t *testing.T) {
				r := initialRejectedRun(t, b.LifecycleStartTurn, status)
				tc.mutate(r)
				settleRunStop(r)
				if r.CommandStates["submit"] != status {
					t.Fatal("stop fabricated settlement for an unverified original input", r)
				}
			})
		}
	}
}
