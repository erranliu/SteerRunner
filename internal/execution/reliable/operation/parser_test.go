package operation

import (
	"context"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestParserCommittedIntentSharesUnknownReconciliation(t *testing.T) {
	s := originalRequest().Snapshot
	i, err := New("association", s, b.LifecycleCandidate{Action: b.LifecycleParseResult, Input: b.LifecycleInput{Parse: &b.ParseInput{ResultID: "result", Payload: "frozen", Generation: "parser", Deadline: time.Now().Add(time.Minute)}}}, "parser", "key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	i.Request.Operation.Status = b.OperationPending
	ops := []b.Operation{i.Request.Operation}
	p := &effectFixture{}
	if result := InvokeParser(context.Background(), p, p, i.Request, ExecuteNewIntent); result.Err == nil || p.executes != 0 {
		t.Fatal("pending parser escaped commit", result)
	}
	if err = i.MarkParserDispatch(ops); err != nil {
		t.Fatal(err)
	}
	result := InvokeParser(context.Background(), p, p, i.Request, ExecuteNewIntent)
	if !result.Uncertain || p.executes != 1 || p.read != 0 {
		t.Fatal("parser lost unknown outcome or invoked native-turn preparation", result)
	}
	i.Settle(ops, result.Receipt, ExecuteNewIntent, b.OperationUnknown, "lost", time.Now())
	for n := 0; n < 2; n++ {
		_ = InvokeParser(context.Background(), p, p, i.Request, ExecuteNewIntent)
	}
	if p.executes != 1 || p.reconciles != 2 || ops[0].Status != b.OperationUnknown {
		t.Fatal("parser bypassed original-operation reconciliation")
	}
	if err = i.MarkParserDispatch(ops); err == nil {
		t.Fatal("unknown parser was reset to unsent")
	}
	changed := i.Request.Clone()
	changed.Input.Parse.Payload = "replacement"
	_ = InvokeParser(context.Background(), p, p, changed, ReconcileOriginalIntent)
	if p.reconciles != 2 {
		t.Fatal("changed frozen input reached resource")
	}
}

func TestParserCanceledInvocationReachesOriginalSettlementPath(t *testing.T) {
	s := originalRequest().Snapshot
	i, err := NewParser("association", s, b.ParseInput{ResultID: "result", Payload: "saved", Generation: "original", Deadline: time.Now().Add(time.Minute)}, "parser", "key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = i.MarkParserDispatch(nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &effectFixture{}
	_ = InvokeParser(ctx, p, p, i.Request, ExecuteNewIntent)
	if p.executes != 1 || p.reconciles != 0 {
		t.Fatal("cancellation skipped the resource's original terminal journal path")
	}
}
