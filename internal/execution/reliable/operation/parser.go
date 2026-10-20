package operation

import (
	"context"
	"errors"
	"reflect"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// NewParser fixes the existing unsent parser intent without changing the
// default in-flight semantics of ordinary operations.
func NewParser(runID string, snapshot b.LifecycleSnapshot, input b.ParseInput, id, key string, at time.Time) (Intent, error) {
	i, err := New(runID, snapshot, b.LifecycleCandidate{Action: b.LifecycleParseResult, Input: b.LifecycleInput{Parse: &input}}, id, key, at)
	if err == nil {
		i.Request.Operation.Status = b.OperationPending
	}
	return i, err
}

func (i Intent) ParserUnsettled() bool {
	return i.Request.Input.Parse != nil && i.Request.Operation.Status != b.OperationPending && (i.Receipt == nil || !i.Receipt.State.Stopped || !i.Receipt.State.WritersAbsent)
}

// MarkParserDispatch changes only the existing pending parser status. The host
// still owns capacity, authorization, gate/CAS and the commit before dispatch.
func (i *Intent) MarkParserDispatch(operations []b.Operation) error {
	if i.Request.Input.Parse == nil || i.Request.Operation.Action != string(b.LifecycleParseResult) || i.Request.Operation.Status != b.OperationPending {
		return errors.New("parser has no original pending intent")
	}
	if err := i.Request.Validate(); err != nil {
		return err
	}
	i.Request.Operation.Status = b.OperationInFlight
	for n := range operations {
		if operations[n].ID == i.Request.Operation.ID {
			operations[n].Status = b.OperationInFlight
		}
	}
	return nil
}

// ParserReceipt retains the existing terminal-conflict comparison, independent
// of the journal's mutable operation wrapper or business acquisition cursor.
func (i Intent) ParserReceipt(receipt b.LifecycleReceipt) (final, same bool) {
	final = i.Request.Operation.Status == b.OperationConfirmed && i.Receipt != nil && i.Receipt.State.Stopped
	if final {
		same = reflect.DeepEqual(i.Receipt.ParseEvidence, receipt.ParseEvidence) && reflect.DeepEqual(i.Receipt.Process, receipt.Process) && reflect.DeepEqual(i.Receipt.OriginalHost, receipt.OriginalHost) && reflect.DeepEqual(i.Receipt.State, receipt.State)
	}
	return
}

// InvokeParser shares immutable-request, invocation-mode and receipt validation
// with ordinary operations, without adding native-turn preparation to a parser.
// The resource may return while the original asynchronous task is still live.
// Cancellation is delivered to that resource too: its original startup path
// writes the terminal journal even when canceled before native dispatch.
func InvokeParser(ctx context.Context, port b.LifecycleExecutePort, reconciler b.LifecycleReconcilePort, request b.LifecycleRequest, mode InvocationMode) EffectResult {
	if request.Input.Parse == nil || request.Operation.Action != string(b.LifecycleParseResult) {
		return EffectResult{Request: request.Clone(), Err: errors.New("not an original parser operation")}
	}
	return invoke(ctx, port, reconciler, request, mode, nil)
}
