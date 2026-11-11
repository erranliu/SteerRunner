// Package orcatransport implements terminal communication, not worker scheduling.
package orcatransport

import (
	"context"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"

	"sync"
	"sync/atomic"
	"time"
)

type Turn struct {
	Budget           *m.InputBudget           `json:"input_budget,omitempty"`
	WireBudget       *m.InputBudget           `json:"wire_budget,omitempty"`
	Model            string                   `json:"model,omitempty"`
	ReasoningEffort  string                   `json:"reasoning_effort,omitempty"`
	StepID           string                   `json:"step_id,omitempty"`
	Extension        *TerminalReceipt         `json:"extension,omitempty"`
	WireFormat       string                   `json:"wire_format,omitempty"`
	Recorded         *codex.RemoteTurnReceipt `json:"recorded,omitempty"`
	ReceiptRequest   string                   `json:"receipt_request,omitempty"`
	ReceiptQueriedAt time.Time                `json:"receipt_queried_at,omitempty"`
	Native           NativeProof              `json:"native"`
	Command          string                   `json:"command"`
	Request          string                   `json:"request,omitempty"`
	InputDigest      string                   `json:"input_digest"`
	Attempted        bool                     `json:"attempted"`
	Rejected         bool                     `json:"rejected,omitempty"`
	Accepted         bool                     `json:"accepted"`
	Started          bool                     `json:"started"`
	Settled          bool                     `json:"settled"`
	Prompt           string                   `json:"prompt"`
	Reply            string                   `json:"reply,omitempty"`
	ReplyDigest      string                   `json:"reply_digest,omitempty"`
}
type State struct {
	Control            *ControlTerminal       `json:"control,omitempty"`
	TerminalReadiness  *TerminalReceipt       `json:"terminal_readiness,omitempty"`
	TerminalOwner      *TerminalOwner         `json:"terminal_owner,omitempty"`
	AdmissionAttempted bool                   `json:"admission_attempted,omitempty"`
	AdmissionAccepted  bool                   `json:"admission_accepted,omitempty"`
	RemoteReadiness    *codex.RemoteReadiness `json:"remote_readiness,omitempty"`
	Process            b.ProcessIdentity      `json:"process"`
	Turns              []Turn                 `json:"turns"`
	LaunchDigest       string                 `json:"launch_digest,omitempty"`
	LaunchRequest      string                 `json:"launch_request,omitempty"`
	LaunchAttempted    bool                   `json:"launch_attempted,omitempty"`
	LaunchAccepted     bool                   `json:"launch_accepted,omitempty"`
	LaunchConfirmed    bool                   `json:"launch_confirmed,omitempty"`
}
type Lifecycle struct {
	agentSessions    func(string) agentSessionPort
	agentHostID      string
	agentDevelopment bool
	agentAudited     bool
	stopping         atomic.Bool
	probeProcess     func(*b.ProcessIdentity) b.ProcessProbeFacts
	setup            b.LifecycleSetup
	profile          Profile
	profileVersion   int
	client           native.Client
	clock            func() time.Time
	nativeAccess     bool
	inspectionOnce   sync.Once
	inspectionSerial chan struct{}
}

func NewLifecycle(setup b.LifecycleSetup) (b.LifecyclePort, error) {
	if setup.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
		return nil, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Legacy Orca terminal execution must be drained with its original Runner build."}
	}
	r, err := decodeFrozenRecord(setup)
	if err != nil {
		return nil, err
	}
	l := &Lifecycle{setup: r.setup, profile: r.profile, profileVersion: r.profileVersion, agentHostID: r.agentHostID, agentDevelopment: r.agentDevelopment, agentAudited: r.agentAudited, probeProcess: r.probeProcess, clock: func() time.Time { return time.Now().UTC() }, nativeAccess: true}
	l.client = native.New(native.Config{CommandTimeout: "40s", Output: orcaCLIOutput(r.setup, l.profile, l.agentHostID, func() bool { return true })})
	return l, nil
}

func (l *Lifecycle) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return l.structuredPolicy()
}

func (l *Lifecycle) validate(s b.LifecycleSnapshot) error { return l.frozenRecord().validate(s) }

func SessionID(p b.ProcessIdentity) string {
	return "orca-terminal:" + p.RuntimeID + ":" + p.Terminal + ":" + p.Incarnation
}

func command(s b.LifecycleSnapshot) string {
	if s.PendingTurn != nil {
		return s.PendingTurn.CommandID
	}
	if s.Session.PendingCommandID != "" {
		return s.Session.PendingCommandID
	}
	return s.Session.CurrentCommandID
}

func (l *Lifecycle) PreparationFacts(ctx context.Context, s b.LifecycleSnapshot) (b.PreparationFacts, error) {
	return l.structuredPreparation(ctx, s)
}

func (l *Lifecycle) Execute(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	return l.executeStructured(ctx, r, false)
}

// Recovery resumes observation of the same resource and saved input/baseline only.
func (l *Lifecycle) ObserveLifecycle(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	return l.structuredObserveLifecycle(ctx, s)
}

func (l *Lifecycle) Reconcile(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	return l.executeStructured(ctx, r, true)
}

func (l *Lifecycle) StopLifecycle(ctx context.Context, r b.LifecycleRequest, owned b.StopHandle) (receipt b.LifecycleReceipt, err error) {
	return l.structuredStop(ctx, r, false)
}

func (l *Lifecycle) ShutdownLifecycle(ctx context.Context, s b.LifecycleSnapshot, owned b.StopHandle) (b.LifecycleStateFacts, error) {
	if err := l.verifyAgentSessionStopped(s); err != nil {
		return b.LifecycleStateFacts{}, err
	}
	return b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}, nil
}
func (l *Lifecycle) VerifyManualRecovery(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	receipt, err := l.readStructured(ctx, s)
	if l.agentDevelopment {
		return b.LifecycleRecoveryEvidence{SessionAvailable: receipt.State.SessionAvailable, ResourceAvailable: receipt.State.SessionAvailable, SessionIdle: receipt.State.TurnCompleted && !receipt.State.WorkerLive && !receipt.State.QuestionPending}, err
	}
	return b.LifecycleRecoveryEvidence{SessionAvailable: receipt.State.SessionAvailable, ResourceAvailable: receipt.State.SessionAvailable, Diagnostic: "Structured execute/1 does not support additional input or resume."}, err
}
