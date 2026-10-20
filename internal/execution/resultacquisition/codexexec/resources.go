package codexexec

import (
	"context"
	"errors"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
)

// RetainedOwner interprets private native ownership, never input authority.
func RetainedOwner(s b.LifecycleSnapshot) *b.ProcessIdentity {
	if owner := codex.ExecutionOwner(s); owner != nil && owner.RetainsParser {
		id := owner.Process
		return &id
	}
	return nil
}

func Retained(request b.LifecycleRequest) bool {
	owner := codex.ExecutionOwner(request.Snapshot)
	return owner != nil && request.Input.Parse != nil && request.Input.Parse.Owner != nil && owner.Process == *request.Input.Parse.Owner
}

// Resources owns only the specific parser's native/file interactions. The host
// supplies the original pre-release root commit and retains the dispatch gate.
type Resources struct {
	RootObserved bool
	Root         string
	Owner        b.ParserOwnerPort
	Prior        *b.LifecycleReceipt
	Started      func(b.LifecycleReceipt) error
	Task         b.ParserTask
}

func (p *Resources) Execute(ctx context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if Retained(request) {
		if p.Owner == nil {
			return b.LifecycleReceipt{}, nil, errors.New("original parser owner unavailable")
		}
		return p.Owner.Execute(ctx, request)
	}
	p.Task = Start(ctx, p.Root, request, p.Started)
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationAccepted, State: b.LifecycleStateFacts{ResourcePending: true, ResourceID: request.Input.Parse.Generation}}, nil, nil
}

// Reconcile reads only the original journals. Root death by itself is never a
// proof of descendant absence while the original local owner is still live.
func (p *Resources) Reconcile(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	receipt, err := ReadReceipt(p.Root, request)
	if err == nil && receipt.State.Stopped && receipt.State.WritersAbsent && (OriginalAbsent(receipt) || Retained(request) && (receipt.Process == nil || process.InspectProcess(receipt.Process) == "process absent")) {
		return receipt, nil, nil
	}
	if p.Prior == nil {
		if started, err := ReadStartedReceipt(p.Root, request); err == nil {
			p.RootObserved = true
			return started, nil, nil
		}
	}
	if p.Prior != nil && OriginalAbsent(*p.Prior) {
		receipt = p.Prior.Clone()
		receipt.Disposition = b.OperationConfirmed
		receipt.State = b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true, WritersAbsent: true}
		receipt.ParseEvidence = &b.ParseEvidence{Status: "failed", Diagnostic: "original parser ended without a complete final; no replay"}
		return receipt, nil, nil
	}
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationUnknown}, nil, errors.New("original parser receipt unavailable")
}

func OriginalAbsent(receipt b.LifecycleReceipt) bool {
	absent := func(p *b.ProcessIdentity) bool {
		return p != nil && p.PID > 0 && p.Created != "" && p.Executable != "" && process.NewServiceFacts().Probe(p).State == "absent"
	}
	return absent(receipt.OriginalHost) && absent(receipt.Process)
}

// ValidateRecovery qualifies the existing frozen parser contract only. It does
// not start a parser, read journals, or grant source/business eligibility.
func ValidateRecovery(request b.LifecycleRequest) error {
	var pack Package
	if request.Input.Parse == nil || request.Validate() != nil || b.DecodeStrictJSON([]byte(request.Input.Parse.Payload), &pack, "invalid frozen parser package") != nil || pack.Profile != Profile || pack.Schema != string(ra.ParserSchema()) || pack.Config.Config.Validate() != nil {
		return errors.New("frozen parser/result acquisition contract unsupported")
	}
	return nil
}
