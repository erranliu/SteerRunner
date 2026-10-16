package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestSRTurnModelOperation(t *testing.T) {
	snapshot := originalRequest().Snapshot
	snapshot.PendingTurn.Model, snapshot.PendingTurn.ReasoningEffort = "reviewer", "high"
	intent, err := New("association", snapshot, b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: snapshot.PendingTurn}}, "original", "original-key", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(snapshot.PendingTurn)
	if !bytes.Equal(intent.Request.Operation.Request, wire) || intent.Request.Operation.PayloadDigest != reliable.Digest(wire) {
		t.Fatal("operation did not freeze its per-turn models in the original request and digest")
	}
	p := &effectFixture{preparationFixture: preparationFixture{caps: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: b.MechanismNotApplicable, Session: b.MechanismApplicable}, facts: b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateNotApplicable, Session: b.GateReady}}}
	for _, field := range []string{"model", "effort"} {
		t.Run(field, func(t *testing.T) {
			request := intent.Request.Clone()
			if field == "model" {
				request.Input.Turn.Model = "executor"
			} else {
				request.Input.Turn.ReasoningEffort = "low"
			}
			result := Invoke(context.Background(), p, p, request, ExecuteNewIntent)
			if result.Err == nil || p.executes != 0 || p.reconciles != 0 {
				t.Fatal("changed model or effort reached native dispatch")
			}
		})
	}
	_ = Invoke(context.Background(), p, p, intent.Request, ExecuteNewIntent)
	if p.executes != 1 {
		t.Fatal("unchanged model request did not reach native dispatch")
	}
}

func TestStopSettlementPreservesExistingSnapshotRepresentation(t *testing.T) {
	snapshot := originalRequest().Snapshot
	intent, err := New("association", snapshot, b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{TurnID: "turn"}}}, "original-stop", "snapshot-key", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	operations := []b.Operation{intent.Request.Operation}
	original := intent.Request.Clone()
	// Legacy v2 records bound the stop command after saving their first intent.
	intent.Request.Operation.IdempotencyKey = "stop-command"
	intent.SettleStop(operations, b.LifecycleReceipt{Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true}}, b.OperationConfirmed)
	if intent.Request.Operation.IdempotencyKey != "stop-command" || intent.Receipt.Operation.IdempotencyKey != "stop-command" || operations[0].IdempotencyKey != "snapshot-key" {
		t.Fatal("stop settlement rewrote the existing v2 key representations")
	}
	if intent.Request.Operation.Status != b.OperationConfirmed || operations[0].Status != b.OperationConfirmed || intent.Receipt.Operation.Status != b.OperationInFlight {
		t.Fatal("stop settlement changed status or original receipt identity semantics")
	}
	if !bytes.Equal(intent.Request.Operation.Request, original.Operation.Request) || intent.Request.Operation.PayloadDigest != original.Operation.PayloadDigest || !intent.Request.Operation.UpdatedAt.Equal(original.Operation.UpdatedAt) || operations[0].Receipt != nil {
		t.Fatal("stop settlement normalized historical input, timestamp or receipt metadata")
	}
}
