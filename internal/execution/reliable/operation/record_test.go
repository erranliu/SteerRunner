package operation

import (
	"bytes"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

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
