// Package orcatransport implements terminal communication, not worker scheduling.
package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"

	"strings"
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
	p, err := decodeAgentSessionProfile(r.setup)
	if err != nil {
		return nil, err
	}
	l := &Lifecycle{setup: r.setup, profile: r.profile, profileVersion: r.profileVersion, agentHostID: r.agentHostID, agentDevelopment: r.agentDevelopment, agentAudited: p.Audited, probeProcess: r.probeProcess, clock: func() time.Time { return time.Now().UTC() }, nativeAccess: true}
	l.client = native.New(native.Config{CommandTimeout: "40s", Output: orcaCLIOutput(r.setup, l.profile, l.agentHostID, func() bool { return true })})
	return l, nil
}

func (l *Lifecycle) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return l.structuredPolicy()
}

func (l *Lifecycle) validate(s b.LifecycleSnapshot) error { return l.frozenRecord().validate(s) }

// resourceState reads bound resource identities without accepting business evidence.
func (l *Lifecycle) resourceState(s b.LifecycleSnapshot) (State, error) {
	return l.frozenRecord().resourceState(s)
}

func (l *Lifecycle) state(s b.LifecycleSnapshot) (State, error) { return l.frozenRecord().state(s) }

func (l *Lifecycle) stateVersion() int { return l.frozenRecord().stateVersion() }

func (l *Lifecycle) envelope(s b.LifecycleSnapshot, state State) ([]b.ImplementationDataEnvelope, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: l.stateVersion(), Purpose: b.ImplementationResourcePurpose, ImplementationKey: StateKey, ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}, raw)
	if err != nil {
		return nil, err
	}
	return []b.ImplementationDataEnvelope{e}, nil
}

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

func (l *Lifecycle) boundTerminal(ctx context.Context, p b.ProcessIdentity) (native.Terminal, error) {
	if !l.nativeAccess {
		// Package fixtures supply an isolated original-resource CLI client.
		return InspectBoundTerminal(ctx, l.client, p)
	}
	t, err := native.QueryBoundTerminal(ctx, l.profile.Environment, native.TerminalIdentity{RuntimeID: p.RuntimeID, Handle: p.Terminal, IncarnationID: p.Incarnation})
	var identity *native.TerminalIdentityError
	if errors.As(err, &identity) {
		err = &b.BackendError{Code: b.BackendIdentityMismatch, Diagnostic: identity.Error(), Cause: err}
	}
	return t, err
}

func (l *Lifecycle) original(ctx context.Context, s b.LifecycleSnapshot) (native.Terminal, error) {
	state, stateErr := l.state(s)
	if stateErr != nil {
		return native.Terminal{}, stateErr
	}
	initializing := l.setup.AgentLaunchCommand != "" && !state.LaunchConfirmed
	t, err := l.boundTerminal(ctx, s.Ref.External)
	if err != nil {
		return t, err
	}
	if t.WorktreePath != s.Workspace.Path || strings.TrimPrefix(t.Branch, "refs/heads/") != s.Workspace.Branch && !(initializing && t.Branch == "") {
		return t, errors.New("original Orca workspace or branch changed")
	}
	// A connected native background terminal can be unadopted by the UI
	// (orphaned=true). Its original runtime/incarnation/worktree and agent must
	// still match; adoption into a visible tab is not execution identity.
	// Agent labels are display heuristics, not execution ownership evidence.
	// Managed agents instead require their original OS owner and native receipt
	// before every business input/observation. PTY/runtime/incarnation still bind.
	unknownNativeAgent := l.profile.Terminal != nil
	if t.WorktreeID != s.Ref.External.WorktreeID || t.AgentIdentity != s.Ref.Agent.Name && !(initializing && t.AgentIdentity == "") && !unknownNativeAgent || !t.Connected || t.Exited() {
		return t, fmt.Errorf("original Orca agent/worktree changed (worktree=%q expected=%q agent=%q orphaned=%t)", t.WorktreeID, s.Ref.External.WorktreeID, t.AgentIdentity, t.Orphaned)
	}
	return t, nil
}

// Probe the already registered source before creating a business workspace.
// Local observation uses native queries; it does not require a streaming pipe.
func (l *Lifecycle) verifyLocalObservation(ctx context.Context, runtime string) error {
	bootstrap, err := native.LocalRuntimeID()
	if err != nil {
		return err
	}
	if bootstrap != runtime {
		return fmt.Errorf("%w: local CLI/bootstrap runtime changed", native.ErrStateIdentityChanged)
	}
	if _, err := native.QueryWorktreeAgents(ctx, "", runtime); err != nil {
		return err
	}
	_, err = native.QueryState(ctx, "", runtime, "path:"+l.setup.RepositoryPath)
	return err
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
