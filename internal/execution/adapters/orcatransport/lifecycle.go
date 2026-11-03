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
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"io"
	"os"
	"reflect"
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
	agentSessions      func(string) agentSessionPort
	stopping           atomic.Bool
	probeProcess       func(*b.ProcessIdentity) b.ProcessProbeFacts
	uploadStopArtifact func(context.Context, string, string, string, string, io.Reader) error
	observationMu      sync.Mutex
	nativeObservation  *nativeObservation
	terminalMu         sync.Mutex
	terminalJob        b.ProcessTreeHandle
	setup              b.LifecycleSetup
	profile            Profile
	profileVersion     int
	client             native.Client
	stateFactory       stateFeedFactory
	queryNative        func(context.Context, b.ProcessIdentity, string) (native.StateSample, error)
	queryInput         func(context.Context, b.ProcessIdentity, string, int64, native.AgentState) (originalInputState, error)
	feed               native.StateFeed
	feedProcess        b.ProcessIdentity
	feedMu             sync.Mutex
	feedRetryAt        time.Time
	clock              func() time.Time
	nativeAccess       bool
	inspectionOnce     sync.Once
	inspectionSerial   chan struct{}
}

func NewLifecycle(setup b.LifecycleSetup) (b.LifecyclePort, error) {
	port, err := NewLifecycleWithStateFactory(setup, openOrcaStateFeed)
	if err == nil {
		port.(*Lifecycle).nativeAccess = true
		l := port.(*Lifecycle)
		l.queryNative = func(ctx context.Context, process b.ProcessIdentity, pane string) (native.StateSample, error) {
			return native.QueryOriginalSample(ctx, l.profile.Environment, orcaStateResource(process), pane)
		}
		l.queryInput = func(ctx context.Context, process b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (originalInputState, error) {
			return queryOriginalInput(ctx, l.profile.Environment, process, pane, baseline, before)
		}
	}
	return port, err
}

// The explicit port supplies isolated native-response fixtures without adding
// simulation flags to profiles or the shared business workflow.
func NewLifecycleWithStateFactory(setup b.LifecycleSetup, factory stateFeedFactory) (b.LifecyclePort, error) {
	if setup.Snapshot.Ref.Agent.Name != "codex" && setup.Snapshot.Ref.Agent.Name != "pi" {
		return nil, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Orca agent execution/history is unsupported; only Codex and Pi are supported."}
	}
	if factory == nil {
		return nil, errors.New("missing Orca native state source")
	}
	setup = setup.Clone()
	if setup.ProcessHost == nil {
		return nil, errors.New("unsupported Orca terminal lifecycle")
	}
	p, version, err := decodeLifecycleProfile(setup)
	if err != nil {
		return nil, err
	}
	l := &Lifecycle{setup: setup, profile: p, profileVersion: version, stateFactory: factory, clock: func() time.Time { return time.Now().UTC() }}
	l.probeProcess, l.uploadStopArtifact = process.NewServiceFacts().Probe, UploadWorkspaceArtifact
	call := func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		command := cliLaunchCommand(p, p.Executable, setup.RepositoryPath, timeout, args...)
		if err := process.CheckCommandLine(command.Executable, command.Args); err != nil {
			return nil, err
		}
		invoke := func(ctx context.Context) ([]byte, error) {
			return setup.ProcessHost.OutputCommand(ctx, command)
		}
		if native.PreparationOwnsCLI(ctx) {
			return invoke(ctx)
		}
		if l.nativeAccess {
			callCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return native.SerializeCLI(callCtx, invoke)
		}
		if err := native.CheckpointDispatch(ctx); err != nil {
			return nil, err
		}
		return invoke(ctx)
	}
	l.client = native.New(native.Config{CommandTimeout: "40s", Output: call})
	return l, nil
}

func (l *Lifecycle) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	if l.structured() {
		return l.structuredPolicy()
	}
	resource := b.MechanismNotApplicable
	if l.setup.AgentLaunchCommand != "" {
		resource = b.MechanismApplicable
	}
	policy := "host_managed_permissions; business_read_only_is_prompt_policy"
	if l.profile.nativeTerminalContract() {
		policy = "configured_native_launch_parameters; execution_host_existing_authentication; business_read_only_is_prompt_policy"
	}
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: resource, Session: b.MechanismApplicable}, ReportToolPolicy: policy}
}

func (l *Lifecycle) validate(s b.LifecycleSnapshot) error {
	w := l.setup.Snapshot
	if s.Ref.Backend != w.Ref.Backend || s.Ref.Protocol != w.Ref.Protocol || s.Ref.Target.ID != w.Ref.Target.ID || s.Ref.Agent != w.Ref.Agent || s.Ref.RunID != w.Ref.RunID || s.Ref.AttemptID != w.Ref.AttemptID || s.Session.ProjectID != w.Session.ProjectID || s.Session.WorkflowRunID != w.Session.WorkflowRunID || s.Ref.Target.ExternalRuntime != l.profile.Target().ExternalRuntime || s.Ref.Target.ExecutionHost != l.profile.Target().ExecutionHost || s.Session.ExecutionID != w.Session.ExecutionID || s.Session.AttemptID != w.Session.AttemptID || s.ConfigurationDigest != w.ConfigurationDigest || s.ResourceGeneration != w.ResourceGeneration {
		return errors.New("Orca terminal execution binding changed")
	}
	if s.Session.ProjectID == "" || s.ConfigurationDigest == "" || s.ResourceGeneration == "" {
		return errors.New("incomplete Orca terminal execution")
	}
	return nil
}

// resourceState reads bound resource identities without accepting business evidence.
func (l *Lifecycle) resourceState(s b.LifecycleSnapshot) (State, error) {
	if err := l.validate(s); err != nil {
		return State{}, err
	}
	state := State{}
	found := false
	for _, e := range s.Implementation {
		if e.ImplementationKey == StateKey {
			if e.CodecVersion != l.stateVersion() {
				return State{}, errors.New("terminal state/profile codec mismatch")
			}
			if found || !readableStateVersion(e.CodecVersion) {
				return state, errors.New("unsupported terminal state")
			}
			found = true
			if err := e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest); err != nil {
				return state, err
			}

			var err error
			err = b.DecodeStrictJSON(e.Payload, &state, "trailing terminal state")
			if err != nil {
				return state, err
			}
		}
	}
	if state.Process.Terminal != "" && state.Process != s.Ref.External {
		return state, errors.New("saved original terminal differs from current resource")
	}
	if state.LaunchAttempted && (state.LaunchDigest != digest([]byte(l.setup.AgentLaunchCommand)) || state.LaunchRequest == "") || state.LaunchAccepted && !state.LaunchAttempted || state.LaunchConfirmed && !state.LaunchAccepted {
		return state, errors.New("original agent launch binding changed")
	}
	if state.AdmissionAttempted && state.RemoteReadiness == nil || state.AdmissionAccepted && !state.AdmissionAttempted {
		return state, errors.New("remote admission lost its original preparation binding")
	}
	if state.Control != nil {
		if !l.usesControl() {
			return state, errors.New("unexpected native control resource")
		}
		if err := l.validateControl(s, state, false); err != nil {
			return state, err
		}
	}
	return state, nil
}

func (l *Lifecycle) state(s b.LifecycleSnapshot) (State, error) {
	state, err := l.resourceState(s)
	if err != nil {
		return state, err
	}
	if err := validateNativeProofFlags(state); err != nil {
		return state, err
	}
	if l.profile.nativeTerminalContract() {
		if err := validateNativeTerminalState(state); err != nil {
			return state, err
		}
	}
	if l.usesControl() && (state.LaunchAttempted || len(state.Turns) != 0) {
		if err := l.validateControl(s, state, true); err != nil {
			return state, err
		}
	}
	seen := map[string]bool{}
	for _, turn := range state.Turns {
		if turn.WireFormat != "" && turn.WireFormat != remoteWireCommand && turn.WireFormat != remoteModelWireCommand && turn.WireFormat != extensionWireCommand {
			return state, errors.New("unknown native input wire format")
		}
		if turn.Recorded != nil {
			if err := validateRecordedTurn(state, &turn, *turn.Recorded); err != nil {
				return state, err
			}
			if turn.ReceiptRequest == "" || turn.ReceiptQueriedAt.IsZero() || state.Control == nil {
				return state, errors.New("original native control query evidence missing")
			}
		}
		if !b.StableExecutionID(turn.Command) || seen[turn.Command] || len(turn.InputDigest) != 64 || turn.Request != "" && !b.StableExecutionID(strings.ReplaceAll(turn.Request, "-", "")) || turn.Started && !turn.Accepted || turn.Accepted && !turn.Attempted || turn.Settled && (turn.Reply == "" || turn.ReplyDigest != digest([]byte(turn.Reply))) {
			return state, errors.New("invalid original terminal input binding")
		}
		seen[turn.Command] = true
		if s.PendingTurn != nil && s.PendingTurn.CommandID == turn.Command && turn.InputDigest != digest([]byte(s.PendingTurn.Prompt)) {
			return state, errors.New("saved terminal input differs from original command")
		}
		if s.PendingTurn != nil && s.PendingTurn.CommandID == turn.Command && (turn.Model != s.PendingTurn.Model || turn.ReasoningEffort != s.PendingTurn.ReasoningEffort || turn.Model != "" && turn.StepID != s.PendingTurn.StepID) {
			return state, errors.New("saved native model preferences changed")
		}
		if turn.InputDigest != digest([]byte(turn.Prompt)) || len(turn.Reply) > maxReplyBytes {
			return state, errors.New("invalid saved terminal reply binding")
		}
		if turn.Native.agentName() != s.Ref.Agent.Name {
			return state, errors.New("original native proof agent differs from frozen execution")
		}
		if turn.WireFormat == extensionWireCommand {
			if l.profile.Terminal == nil || state.TerminalReadiness == nil || state.TerminalOwner == nil || turn.Recorded != nil || turn.ReceiptRequest != "" {
				return state, errors.New("native extension readiness/turn binding missing")
			}
			if err := l.validateTerminalReceipt(*state.TerminalReadiness); err != nil {
				return state, err
			}
			if err := l.validateTerminalTurn(state, &turn); err != nil {
				return state, err
			}
			continue
		}
		if turn.Native.Pane == "" || turn.Native.WorkingAt > 0 && (turn.Native.Provider == "" || turn.Native.Authority == "" || turn.Native.Revision <= 0) || turn.Native.EndedAt > 0 && turn.Native.EndedAt <= turn.Native.WorkingAt || turn.Settled && turn.Recorded == nil && (turn.Native.EndedAt == 0 || turn.Native.ReplyDigest != digest([]byte(turn.Reply))) || turn.Settled && turn.Recorded != nil && (!turn.Recorded.Completed || turn.Recorded.Status != "completed" || turn.Recorded.FinalText != turn.Reply) {
			return state, errors.New("invalid original native turn evidence")
		}
	}
	return state, nil
}

func (l *Lifecycle) stateVersion() int { return l.profile.stateCodec() }

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
func turnFor(state *State, command string) *Turn {
	for i := range state.Turns {
		if state.Turns[i].Command == command {
			return &state.Turns[i]
		}
	}
	return nil
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
		// The explicit state-factory constructor uses isolated CLI fixtures.
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

func (l *Lifecycle) agentExecutionSupport(agent string) error {

	if agent != "codex" && !(agent == "pi" && l.profile.Terminal != nil && l.profile.Terminal.Agent == agent && l.profile.Environment == "" && len(l.profile.RuntimeSHA256) == 64) {
		// Orca 1.4.224 lacks the start, root outcome and permission preparation contract.
		return &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "terminal_agent_evidence_unsupported: selected agent lacks verified native turn and permission preparation; zero business input"}
	}
	return nil
}

func (l *Lifecycle) PreparationFacts(ctx context.Context, s b.LifecycleSnapshot) (b.PreparationFacts, error) {
	if l.structured() {
		return l.structuredPreparation(ctx, s)
	}
	if err := l.validate(s); err != nil {
		return b.PreparationFacts{}, err
	}
	if err := l.agentExecutionSupport(s.Ref.Agent.Name); err != nil {
		return b.PreparationFacts{}, err
	}
	if s.Workspace.Path == "" {
		return b.PreparationFacts{Workspace: b.GateRequired, Resource: b.GateUnverified, Session: b.GateUnverified}, nil
	}
	state, err := l.state(s)
	if err != nil {
		return b.PreparationFacts{}, err
	}
	if l.setup.AgentLaunchCommand != "" && !state.LaunchConfirmed {
		if l.usesControl() && (state.Control == nil || state.Control.Ready == nil) {
			intent := l.controlIntent(s, state)
			return b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateRequired, Session: b.GateUnverified, ResourceRequired: &intent}, nil
		}
		return b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateRequired, Session: b.GateUnverified, ResourceRequired: &b.ResourceIntent{Purpose: "agent-start", Name: "original-terminal", Objective: l.setup.AgentLaunchCommand}}, nil
	}
	resource := b.GateNotApplicable
	if l.setup.AgentLaunchCommand != "" {
		resource = b.GateReady
	}
	if s.PendingTurn == nil {
		return b.PreparationFacts{Workspace: b.GateReady, Resource: resource, Session: b.GateUnverified}, nil
	}
	if s.Ref.External.Terminal == "" {
		return b.PreparationFacts{}, errors.New("workspace has no original agent terminal; do not create a replacement")
	}
	gate := b.GateRequired
	if turn := turnFor(&state, s.PendingTurn.CommandID); turn != nil && (turn.Attempted || turn.WireBudget != nil) {
		gate = b.GateReady
	}
	return b.PreparationFacts{Workspace: b.GateReady, Resource: resource, Session: gate}, nil
}

func (l *Lifecycle) Execute(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if l.structured() {
		return l.executeStructured(ctx, r, false)
	}

	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	if err := r.Validate(); err != nil {
		return receipt, nil, err
	}
	if err := l.validate(r.Snapshot); err != nil {
		return receipt, nil, err
	}
	if err := l.agentExecutionSupport(r.Snapshot.Ref.Agent.Name); err != nil {
		receipt.Disposition = b.OperationRejected
		return receipt, nil, err
	}
	var err error
	switch b.LifecycleAction(r.Operation.Action) {
	case b.LifecyclePrepareWorkspace:
		receipt, err = l.prepareWorkspace(ctx, r, false)
	case b.LifecycleCreateResource:
		if r.Input.Resource != nil && r.Input.Resource.Purpose == controlPurpose {
			receipt, err = l.prepareControl(ctx, r, false)
		} else {
			receipt, err = l.launchAgent(ctx, r, false)
		}
	case b.LifecyclePrepareSession:
		receipt, err = l.prepareInput(ctx, r)
	case b.LifecycleStartTurn, b.LifecycleSubmitInput:
		receipt, err = l.submit(ctx, r)
	default:
		err = errors.New("unsupported terminal action")
	}
	if err == nil && (b.LifecycleAction(r.Operation.Action) == b.LifecycleStartTurn || b.LifecycleAction(r.Operation.Action) == b.LifecycleSubmitInput) && receipt.State.InputAccepted {
		snapshot := r.Snapshot.Clone()
		snapshot.Implementation = receipt.Implementation
		snapshot.PendingTurn = r.Input.Turn
		return receipt, l.observe(snapshot, nil), nil
	}
	return receipt, nil, err
}

// Readiness is a native main-agent fact. Never infer it from a prompt,
// screen draft, completion decoration or the terminal's title.
func (l *Lifecycle) ready(ctx context.Context, s b.LifecycleSnapshot) (native.AgentState, error) {
	wait, _ := time.ParseDuration(l.profile.WaitTimeout)
	readyCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	backoff := 250 * time.Millisecond
	for {
		// Retry read-only connection failures and transient absence of the original
		// main-agent query. Every attempt validates the same original resource;
		// neither the readiness deadline nor any business input is recreated.
		current, err := l.readyOnce(readyCtx, s)
		if err == nil {
			return current, nil
		}
		if !native.IsStateConnectionError(err) && !errors.Is(err, native.ErrOriginalMainAgentUnavailable) {
			return current, err
		}
		l.closeStateFeed()
		timer := time.NewTimer(backoff)
		select {
		case <-readyCtx.Done():
			timer.Stop()
			return current, fmt.Errorf("original native connection recovery exhausted; no input was sent: %w", errors.Join(err, readyCtx.Err()))
		case <-timer.C:
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

func (l *Lifecycle) readyOnce(readyCtx context.Context, s b.LifecycleSnapshot) (native.AgentState, error) {
	if err := readyCtx.Err(); err != nil {
		return native.AgentState{}, err
	}
	if _, err := l.original(readyCtx, s); err != nil {
		return native.AgentState{}, err
	}
	feed, err := l.stateFeed(readyCtx, s.Ref.External)
	if err != nil {
		return native.AgentState{}, err
	}
	for {
		current, err := feed.Query(readyCtx)
		if err != nil {
			return current.AgentState, fmt.Errorf("original native readiness unavailable; no input was sent: %w", err)
		}
		if _, err := l.original(readyCtx, s); err != nil {
			return current.AgentState, err
		}
		if current.PaneKey != feed.Initial().PaneKey || current.AgentType != s.Ref.Agent.Name {
			return current.AgentState, errors.New("original native main-agent identity changed; no input was sent")
		}
		if current.MainAgent != nil && current.ProviderSession.ID != "" {
			switch current.MainAgent.State {
			case "done", "idle":
				return current.AgentState, nil
			case "waiting", "blocked":
				return current.AgentState, fmt.Errorf("original native main agent is %s; no input was sent", current.MainAgent.State)
			}
		}
		select {
		case <-readyCtx.Done():
			return current.AgentState, fmt.Errorf("original native main-agent readiness timed out; no input was sent: %w", readyCtx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func nativeRequest(operation string) string {
	value := digest([]byte(operation))[:32]
	return value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
}

// Persist the original input and native state baseline before submission.
// A visible reply cannot substitute for this turn's native ending evidence.
func (l *Lifecycle) prepareInput(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, error) {
	s := r.Snapshot
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}
	state, err := l.state(s)
	if err != nil {
		return receipt, err
	}
	if !l.profile.nativeTerminalContract() {
		if err := l.checkLocalReadiness(state); err != nil {
			return receipt, err
		}
	}
	if l.setup.AgentLaunchCommand != "" && !state.LaunchConfirmed {
		return receipt, errors.New("original agent launch is not confirmed; no task input was sent")
	}
	input := r.Input.Turn
	if input == nil || !b.StableExecutionID(input.CommandID) {
		return receipt, errors.New("missing original worker input")
	}
	if input.Budget == nil && s.PendingTurn != nil && s.PendingTurn.CommandID == input.CommandID && s.PendingTurn.Prompt == input.Prompt {
		copy := *input
		copy.Budget = s.PendingTurn.Budget.Clone()
		input = &copy
	}
	inputDigest := digest([]byte(input.Prompt))
	if turn := turnFor(&state, input.CommandID); turn != nil {
		if turn.InputDigest != inputDigest {
			return receipt, errors.New("prepared worker input changed")
		}
		if !turn.Attempted && turn.WireBudget == nil {
			turn.Budget = input.Budget.Clone()
			if err := freezeWireInput(turn, input.InputVersion); err != nil {
				return receipt, err
			}
		}
	} else {
		if l.profile.Terminal != nil {
			if _, e := l.original(ctx, s); e != nil {
				return receipt, e
			}
			if e := l.readTerminalReadiness(&state, true); e != nil {
				return receipt, e
			}
			prepared := Turn{Budget: input.Budget.Clone(), Command: input.CommandID, Prompt: input.Prompt, InputDigest: inputDigest, WireFormat: extensionWireCommand, Native: NativeProof{Agent: s.Ref.Agent.Name}}
			if err := freezeWireInput(&prepared, input.InputVersion); err != nil {
				return receipt, err
			}
			state.Turns = append(state.Turns, prepared)
		} else {
			defer l.closeStateFeed()
			initial, readyErr := l.ready(ctx, s)
			if readyErr != nil {
				return receipt, readyErr
			}
			if l.profile.Environment == "" && state.RemoteReadiness != nil && initial.ProviderSession.ID != state.RemoteReadiness.SessionID {
				return receipt, errors.New("remote preparation native session changed")
			}
			baseline := nativeBaseline(initial, l.clock())
			prepared := Turn{Budget: input.Budget.Clone(), Command: input.CommandID, Prompt: input.Prompt, InputDigest: inputDigest, Native: baseline, Model: input.Model, ReasoningEffort: input.ReasoningEffort, StepID: input.StepID}
			if l.profile.Environment == "" && state.RemoteReadiness != nil && state.RemoteReadiness.NativeTransport == codex.RemoteNativeTransport {
				prepared.WireFormat = remoteWireCommand
				if input.Model != "" {
					prepared.WireFormat = remoteModelWireCommand
				}
			}
			if err := freezeWireInput(&prepared, input.InputVersion); err != nil {
				return receipt, err
			}
			state.Turns = append(state.Turns, prepared)
		}
	}
	receipt.Process = &state.Process
	receipt.State = b.LifecycleStateFacts{SessionID: SessionID(state.Process), SessionGeneration: state.Process.Incarnation, SessionAvailable: true, WorkerLive: true}
	receipt.Implementation, err = l.envelope(s, state)
	if err == nil {
		receipt.Disposition = b.OperationConfirmed
	}
	return receipt, err
}
func (l *Lifecycle) submit(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, error) {
	s := r.Snapshot
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}
	state, err := l.state(s)
	if err != nil {
		return receipt, err
	}
	if !l.profile.nativeTerminalContract() {
		if err := l.checkLocalReadiness(state); err != nil {
			return receipt, err
		}
	}
	if l.setup.AgentLaunchCommand != "" && !state.LaunchConfirmed {
		return receipt, errors.New("original agent launch is not confirmed; no task input was sent")
	}
	input := r.Input.Turn
	if input == nil || !b.StableExecutionID(input.CommandID) {
		return receipt, errors.New("missing original worker input")
	}
	turn := turnFor(&state, input.CommandID)
	if turn == nil || turn.InputDigest != digest([]byte(input.Prompt)) {
		return receipt, errors.New("original input has no committed output baseline")
	}
	if turn.Attempted {
		return receipt, errors.New("original terminal input was already attempted; reconcile only")
	}
	// Budget metadata fences new dispatch, never existing observation/stop.
	if turn.Budget != nil {
		if _, err := m.InputParts(turn.Prompt, turn.Budget); err != nil {
			return receipt, err
		}
	}
	if _, err := input.ModelInputParts(); err != nil && input.Budget != nil {
		return receipt, err
	}
	if turn.WireBudget == nil {
		if input.Budget == nil && s.PendingTurn != nil && s.PendingTurn.CommandID == input.CommandID && s.PendingTurn.Prompt == input.Prompt {
			copy := *input
			copy.Budget = s.PendingTurn.Budget.Clone()
			input = &copy
		}
		turn.Budget = input.Budget.Clone()
		if err := freezeWireInput(turn, input.InputVersion); err != nil {
			return receipt, err
		}
	}
	if err := m.CheckInput(nativeWireText(turn), turn.WireBudget); err != nil {
		return receipt, err
	}
	if l.profile.Terminal != nil {
		if _, e := l.original(ctx, s); e != nil {
			return receipt, e
		}
		if e := l.readTerminalReadiness(&state, true); e != nil {
			return receipt, e
		}
	} else {
		initial, readyErr := l.ready(ctx, s)
		if readyErr != nil {
			l.closeStateFeed()
			return receipt, readyErr
		}
		if l.profile.Environment == "" && state.RemoteReadiness != nil && initial.ProviderSession.ID != state.RemoteReadiness.SessionID {
			l.closeStateFeed()
			return receipt, errors.New("remote preparation native session changed")
		}
		if initial.PaneKey != turn.Native.Pane || turn.Native.Provider != "" && initial.ProviderSession.ID != turn.Native.Provider {
			l.closeStateFeed()
			return receipt, errors.New("native original session changed before submission")
		}
		turn.Native = nativeBaseline(initial, l.clock())
		l.feedMu.Lock()
		feed := l.feed
		l.feedMu.Unlock()
		l.observationMu.Lock()
		l.observation(s, feed)
		l.observationMu.Unlock()
	}
	// A completed READY hook can precede the TUI returning to its composer.
	// Gate the only native Codex send on the original terminal becoming idle;
	// this is input readiness, never completion/attribution evidence.
	if (l.profile.nativeTerminalContract() || l.usesControl()) && l.profile.Terminal == nil && s.Ref.Agent.Name == "codex" {
		if err := l.waitOriginalComposer(ctx, s); err != nil {
			return receipt, err
		}
	}
	turn.Request = nativeRequest(r.Operation.ID)
	turn.Attempted = true
	receipt.Disposition = b.OperationUnknown
	receipt.Process = &state.Process
	receipt.RequestID = turn.Request
	receipt.Implementation, err = l.envelope(s, state)
	if err != nil {
		return receipt, err
	}
	receipt.State = b.LifecycleStateFacts{SessionID: SessionID(state.Process), SessionGeneration: state.Process.Incarnation, TurnID: input.CommandID, SessionAvailable: true, WorkerLive: true}
	// Capture acceptance promptly; the original native history/state supplies
	// start/completion evidence. A 30s CLI wait would hold the shared file lock
	// while other Runs need independent preparation/observation calls.
	text := wirePrompt(turn)
	if turn.WireFormat == "" && s.Ref.Agent.Name == "codex" {
		// ConPTY pastes CR and LF as separate line breaks. Compile Codex text
		// to its existing original-input matching form before the only send;
		// keep the frozen logical input/digest and other agents unchanged.
		text = nativePromptText(text)
	}
	// Preserve the removed multiline ingress's control-character rejection.
	// Other agents and existing wire formats retain their own input contract.
	if l.profile.nativeTerminalContract() && l.profile.Terminal == nil && s.Ref.Agent.Name == "codex" && turn.WireFormat == "" && strings.Contains(text, "\n") && strings.ContainsAny(text, "\x00\x1b") {
		l.closeStateFeed()
		return receipt, errors.New("native multiline input contains terminal control characters; no input sent")
	}
	if l.profile.Terminal == nil && s.Ref.Agent.Name == "codex" {
		// Encode terminal paste line breaks as CR; Codex restores logical LF.
		// Frozen wire/budget digests still describe the original logical text,
		// not this CLI parameter. Local and remote inputs use the same encoding.
		text = strings.ReplaceAll(text, "\n", "\r")
	}
	send := native.TerminalSend{Handle: state.Process.Terminal, Text: text, RequestID: turn.Request}
	if l.profile.nativeTerminalContract() {
		// Orca's default receipt may precede the original turn-start observation.
		// Wait on this single accepted request; timeout never resends input and
		// is only a hint; original native input/history must prove business start.
		send.WaitSubmitSeconds = 20
	}
	result, sent, callErr := l.client.SendTerminalWithTimeout(ctx, 50*time.Second, send)
	receipt.Operation.Receipt = append([]byte(nil), result.Payload...)
	bound := sent.Matches(native.TerminalIdentity{RuntimeID: state.Process.RuntimeID, Handle: state.Process.Terminal, IncarnationID: state.Process.Incarnation}, turn.Request) && sent.Prompt.Decoded() && !result.CallUncertain && result.ErrorCode == ""
	if bound && sent.Accepted != nil && !*sent.Accepted && len(sent.Prompt.Stages) == 0 {
		turn.Rejected = true
		receipt.Disposition = b.OperationRejected
		receipt.Implementation, err = l.envelope(s, state)
		l.closeStateFeed()
		return receipt, errors.Join(err, errors.New("original native input explicitly rejected; no input was accepted"))
	}
	if !bound || sent.Accepted == nil || !*sent.Accepted {
		l.closeStateFeed()
		return receipt, errors.Join(callErr, errors.New("original terminal input outcome is unknown; do not resend"))
	}
	if l.profile.Terminal == nil && s.Ref.Agent.Name == "codex" {
		prompt := sent.Prompt
		stages := prompt.Stages
		validStages := len(stages) == 1 && stages[0] == "input_accepted" ||
			len(stages) == 2 && stages[0] == "input_accepted" && stages[1] == "turn_started"
		if prompt.Provider != "codex" || prompt.Observation != "supported" || !validStages {
			l.closeStateFeed()
			return receipt, errors.Join(callErr, errors.New("original Codex prompt receipt is unsupported or unverified; outcome unknown; do not resend"))
		}
	}
	turn.Accepted = true
	for _, stage := range sent.Prompt.Stages {
		if stage == "turn_started" && l.profile.Terminal == nil && s.Ref.Agent.Name != "codex" {
			turn.Started = true
		}
	}
	receipt.Implementation, err = l.envelope(s, state)
	receipt.State.InputAccepted, receipt.State.TurnStarted = true, turn.Started
	receipt.Disposition = b.OperationAccepted
	if turn.Started {
		receipt.Facts = []b.EventFact{{Kind: b.EventStarted, Source: "orca.terminal.send", SourceEventID: turn.Request, SessionID: SessionID(state.Process), StartedTurnID: turn.Command, InputAccepted: true, Matched: true}}
	}
	return receipt, err
}
func (l *Lifecycle) observeOnce(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, *b.PublicMessage, error) {
	receipt := b.LifecycleReceipt{Disposition: b.OperationUnknown}
	state, err := l.state(s)
	if err != nil {
		return receipt, nil, err
	}
	turn := turnFor(&state, command(s))
	if turn == nil || !turn.Attempted {
		return receipt, nil, nil
	}
	receipt.Process = &state.Process
	receipt.State = b.LifecycleStateFacts{SessionID: SessionID(state.Process), SessionGeneration: state.Process.Incarnation, TurnID: turn.Command, InputAccepted: turn.Accepted, TurnStarted: turn.Started}
	if !turn.Settled {
		if _, err = l.original(ctx, s); err != nil {
			return receipt, nil, err
		}
		if turn.WireFormat == extensionWireCommand {
			receipt.Diagnostic = l.observeTerminal(&state, turn)
		} else if turn.WireFormat == remoteWireCommand || turn.WireFormat == remoteModelWireCommand {
			receipt.Diagnostic = l.observeRecorded(ctx, &state, turn)
		} else {
			receipt.Diagnostic = l.observeNative(ctx, s, &state, turn)
		}
		// Bind every streamed observation to the original live PTY, including
		// ended-without-body facts. Never infer identity from an omitted field.
		if _, err = l.original(ctx, s); err != nil {
			receipt.Implementation, _ = l.envelope(s, state)
			l.observationMu.Lock()
			if l.nativeObservation != nil {
				l.nativeObservation.invalidate()
			}
			l.observationMu.Unlock()
			return receipt, nil, err
		}
		if turn.WireFormat == remoteWireCommand || turn.WireFormat == remoteModelWireCommand || turn.WireFormat == extensionWireCommand {
			goto observed
		}
		if !turn.Accepted || !turn.Started || turn.Native.EndedAt == 0 || turn.Native.ReplyBody == "" || receipt.Diagnostic != "" {
			goto observed
		}
		// Native start/end proof already binds this body to the accepted input.
		// Screen rendering, scroll position and a later draft are not final evidence.
		turn.Reply, turn.ReplyDigest, turn.Settled = turn.Native.ReplyBody, turn.Native.ReplyDigest, true
		turn.Native.ReplyBody = ""
	}

observed:
	if turn.Native.EndedAt > 0 && !turn.Settled && receipt.Diagnostic == "" {
		receipt.Diagnostic = "Native turn ended; original final body is missing or reaches Orca's truncation boundary; awaiting a complete native body"
	}
	receipt.Implementation, err = l.envelope(s, state)
	if err != nil {
		return receipt, nil, err
	}
	receipt.State.InputAccepted, receipt.State.TurnStarted = turn.Accepted, turn.Started
	receipt.Disposition = b.OperationAccepted
	receipt.State.TurnCompleted = turn.Native.EndedAt > 0 || turn.Recorded != nil && turn.Recorded.Completed
	receipt.State.WorkerLive = !receipt.State.TurnCompleted
	if failure := l.terminalFailure(state, *turn); failure != nil {
		receipt.TurnEvidence = failure
		receipt.State.TurnCompleted, receipt.State.WorkerLive = true, false
		receipt.Disposition = b.OperationConfirmed
		return receipt, l.publicCandidate(), nil
	}
	if !turn.Settled {
		return receipt, l.publicCandidate(), nil
	}
	receipt.Disposition = b.OperationConfirmed
	l.closeStateFeed()
	receipt.State.TurnCompleted = true
	nativeOutcome := turn.Native.Outcome
	if nativeOutcome == "success" {
		nativeOutcome = "succeeded"
	}
	receipt.TurnEvidence = &b.TurnEvidence{NativeOutcome: nativeOutcome, FinalOrigin: "terminal_observed", FinalSource: NativeReplySource, FinalItemID: turn.ReplyDigest, FinalVerified: true, TurnID: turn.Command, CommandID: turn.Command, SessionID: SessionID(state.Process), Started: true, Completed: true, Succeeded: true, LastMessage: turn.Reply}
	if turn.Extension != nil {
		receipt.TurnEvidence = l.terminalEvidence(state, *turn)
	}
	if turn.Recorded != nil {
		receipt.TurnEvidence.FinalOrigin = "native_history"
		receipt.TurnEvidence.FinalSource = codex.RemoteTurnSource
		receipt.TurnEvidence.FinalItemID = turn.Recorded.FinalItemID
		receipt.TurnEvidence.NativeOutcome = "succeeded"
		receipt.TurnEvidence.FinalSessionID = SessionID(state.Process)
		receipt.TurnEvidence.FinalTurnID = turn.Command
		receipt.TurnEvidence.FinalNativeSessionID = state.RemoteReadiness.SessionID
		receipt.TurnEvidence.FinalNativeTurnID = turn.Recorded.NativeTurnID
	}
	return receipt, l.publicCandidate(), nil
}

// Recovery resumes observation of the same resource and saved input/baseline only.
func (l *Lifecycle) ObserveLifecycle(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if l.structured() {
		return l.structuredObserveLifecycle(ctx, s)
	}
	if err := l.agentExecutionSupport(s.Ref.Agent.Name); err != nil {
		return b.LifecycleReceipt{}, nil, err
	}
	receipt, message, err := l.observeOnce(ctx, s)
	if err != nil {
		l.closeStateFeed()
		return receipt, nil, err
	}
	if receipt.TurnEvidence != nil || receipt.Process == nil {
		return receipt, nil, err
	}
	snapshot := s.Clone()
	snapshot.Implementation = receipt.Implementation
	return receipt, l.observe(snapshot, message), nil
}

func (l *Lifecycle) Reconcile(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if l.structured() {
		return l.executeStructured(ctx, r, true)
	}

	defer l.closeStateFeed()
	if r.Operation.Action != string(b.LifecycleStopExecution) {
		if err := l.agentExecutionSupport(r.Snapshot.Ref.Agent.Name); err != nil {
			return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}, nil, err
		}
	}
	if err := r.Validate(); err != nil {
		return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}, nil, err
	}
	if err := l.validate(r.Snapshot); err != nil {
		return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}, nil, err
	}
	switch b.LifecycleAction(r.Operation.Action) {
	case b.LifecyclePrepareWorkspace:
		receipt, err := l.prepareWorkspace(ctx, r, true)
		return receipt, nil, err
	case b.LifecycleCreateResource:
		if r.Input.Resource != nil && r.Input.Resource.Purpose == controlPurpose {
			receipt, err := l.prepareControl(ctx, r, true)
			return receipt, nil, err
		}
		receipt, err := l.launchAgent(ctx, r, true)
		return receipt, nil, err
	case b.LifecyclePrepareSession:
		receipt, err := l.prepareInput(ctx, r)
		return receipt, nil, err
	case b.LifecycleStartTurn, b.LifecycleSubmitInput:
		state, stateErr := l.state(r.Snapshot)
		if stateErr == nil {
			if turn := turnFor(&state, command(r.Snapshot)); turn != nil && turn.Rejected {
				return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected, Implementation: r.Snapshot.Implementation, Diagnostic: "Original native input was rejected; no input was resent"}, nil, nil
			}
		}
		receipt, _, err := l.observeOnce(ctx, r.Snapshot)
		receipt.Operation = r.Operation
		if receipt.TurnEvidence == nil {
			receipt.Disposition = b.OperationUnknown
			if receipt.Diagnostic == "" {
				receipt.Diagnostic = "Original terminal input is unresolved; no input or resource was recreated"
			}
		}
		return receipt, nil, err
	case b.LifecycleStopExecution:
		return l.reconcileStop(ctx, r)
	default:
		return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}, nil, errors.New("unsupported terminal reconciliation")
	}
}

func (l *Lifecycle) StopLifecycle(ctx context.Context, r b.LifecycleRequest, owned b.StopHandle) (receipt b.LifecycleReceipt, err error) {
	if l.structured() {
		return l.structuredStop(ctx, r, false)
	}
	receipt = b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	receipt.Operation.Receipt = nil
	if err := r.Validate(); err != nil {
		return receipt, err
	}
	if err := l.validate(r.Snapshot); err != nil {
		return receipt, err
	}
	if b.LifecycleAction(r.Operation.Action) == b.LifecycleStopObservation {
		receipt.Disposition = b.OperationConfirmed
		return receipt, nil
	}
	if b.LifecycleAction(r.Operation.Action) != b.LifecycleStopExecution {
		return receipt, errors.New("unsupported terminal stop")
	}
	if h, ok := owned.(*observationHandle); ok {
		if h.Identity() != r.Snapshot.Ref.External {
			return receipt, errors.New("stop changed original observation identity")
		}
		_ = h.owner.Close(ctx)
		// Do not await an event acknowledgement from the actor performing stop.
		// The gate cancels in-flight reads; the scoped stop's in-memory fence
		// rejects later control queries without another persistent checkpoint.
	}
	if l.profile.nativeTerminalContract() {
		return l.nativeStopResult(ctx, r, true)
	}
	if l.localCodexScope() {
		return l.stopLocalCodex(ctx, r, true)
	}
	progress := stopProgress{Version: 1, Phase: "not-attempted"}
	defer func() { receipt.Operation.Receipt, _ = json.Marshal(progress) }()
	if err = l.originalForStop(ctx, r.Snapshot); err != nil {
		return receipt, err
	}
	var terminalOwner *TerminalOwner
	if l.profile.Terminal != nil {
		state, e := l.state(r.Snapshot)
		if e != nil {
			return receipt, e
		}
		owner, e := l.readTerminalOwner()
		if e != nil {
			// Only a positive pre-worker exit can qualify this fallback. A lost
			// owner after readiness/input is never evidence that no worker ran.
			if !errors.Is(e, os.ErrNotExist) || state.TerminalOwner != nil || state.TerminalReadiness != nil || state.LaunchConfirmed || len(state.Turns) != 0 {
				return receipt, e
			}
			owner, e = l.startupFailureOwner()
			if e != nil {
				return receipt, e
			}
		}
		terminalOwner = &owner
		if state.TerminalOwner != nil && !reflect.DeepEqual(state.TerminalOwner, terminalOwner) {
			return receipt, errors.New("original terminal owner changed before stop")
		}
		if owner.Job.Handle != 0 && !l.terminalExitVerified(owner) {
			if e = l.pinTerminalJob(owner); e != nil {
				return receipt, e
			}
			l.terminalMu.Lock()
			l.terminalJob.Kill()
			until := time.Now().Add(3 * time.Second)
			for !l.terminalJob.WritersAbsent() && time.Now().Before(until) {
				time.Sleep(10 * time.Millisecond)
			}
			absent := l.terminalJob.WritersAbsent()
			l.terminalMu.Unlock()
			if !absent {
				return receipt, errors.New("original native terminal owned writers remain unverified")
			}
		}
		// Let the helper stop its native tree and commit its exit before closing
		// the PTY. Closing the PTY first can terminate that cleanup mid-write.
		if owner.Job.Handle != 0 && l.profile.Terminal.ReceiptVersion >= 2 {
			until := time.Now().Add(3 * time.Second)
			for !l.terminalExitVerified(owner) && time.Now().Before(until) {
				time.Sleep(20 * time.Millisecond)
			}
			if !l.terminalExitVerified(owner) {
				return receipt, errors.New("original native terminal exit cleanup unverified")
			}
		}
	}

	// Stop authorization is already durable. Credential uncertainty cannot keep
	// the original worker running; finish its exact PTY closure within a bounded
	// cleanup context even if the preceding credential observation timed out.
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancelStop()
	if err = l.originalForStop(stopCtx, r.Snapshot); err != nil {
		return receipt, err
	}
	progress.Phase = "attempted"
	proof, err := CloseTerminalChecked(stopCtx, l.client, r.Snapshot.Ref.External.Terminal, r.Snapshot.Ref.External.RuntimeID)
	if err != nil {
		return receipt, err
	}
	if proof.Disposition == "confirmed" {
		if l.profile.Terminal != nil {
			until := time.Now().Add(3 * time.Second)
			for process.Probe(&terminalOwner.Job.Owner).State == "matches" && time.Now().Before(until) {
				time.Sleep(20 * time.Millisecond)
			}
			if process.Probe(&terminalOwner.Job.Owner).State != "absent" {
				return receipt, errors.New("original native terminal runtime root stop unverified")
			}
			if terminalOwner.Job.Handle != 0 && l.profile.Terminal.ReceiptVersion >= 2 && !l.terminalExitVerified(*terminalOwner) {
				return receipt, errors.New("original native terminal exit cleanup unverified")
			}
			l.closeTerminalJob()
		}
		progress.Phase = "business-confirmed"
		receipt.State.WorkerAbsent = true

		progress.Phase = "confirmed"
		receipt.Disposition = b.OperationConfirmed
		receipt.State.Stopped = true
		receipt.State.WorkerAbsent = true
	} else {
		receipt.Disposition = b.OperationAccepted
	}
	return receipt, nil
}

func (l *Lifecycle) ShutdownLifecycle(ctx context.Context, s b.LifecycleSnapshot, owned b.StopHandle) (b.LifecycleStateFacts, error) {
	if l.structured() {
		return b.LifecycleStateFacts{}, errors.New(AgentSessionStopDiagnostic)
	}
	if err := l.validate(s); err != nil {
		return b.LifecycleStateFacts{}, err
	}
	if l.profile.nativeTerminalContract() {
		l.closeStateFeed()
		if h, ok := owned.(*observationHandle); ok {
			if h.Identity() != s.Ref.External {
				return b.LifecycleStateFacts{}, errors.New("original observation identity changed")
			}
			_ = h.owner.Close(ctx)
			return b.LifecycleStateFacts{}, h.owner.awaitCleanup(ctx)
		}
		return b.LifecycleStateFacts{}, nil
	}
	if h, ok := owned.(*observationHandle); ok {
		if h.Identity() != s.Ref.External {
			return b.LifecycleStateFacts{}, errors.New("shutdown changed the original observation identity")
		}
		_ = h.owner.Close(ctx)
		if err := h.owner.awaitCleanup(ctx); err != nil {
			return b.LifecycleStateFacts{}, fmt.Errorf("original readonly observation cleanup remains unverified: %w", err)
		}
	}
	defer l.closeTerminalJob()
	if err := l.verifyStopped(ctx, s); err != nil {
		return b.LifecycleStateFacts{}, err
	}
	return b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}, nil
}
func (l *Lifecycle) VerifyManualRecovery(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	if l.structured() {
		receipt, err := l.readStructured(ctx, s)
		return b.LifecycleRecoveryEvidence{SessionAvailable: receipt.State.SessionAvailable, ResourceAvailable: receipt.State.SessionAvailable, Diagnostic: "Structured execute/1 does not support additional input or resume."}, err
	}
	if err := l.agentExecutionSupport(s.Ref.Agent.Name); err != nil {
		return b.LifecycleRecoveryEvidence{}, err
	}
	if err := l.validate(s); err != nil {
		return b.LifecycleRecoveryEvidence{}, err
	}
	_, err := l.original(ctx, s)
	evidence := b.LifecycleRecoveryEvidence{SessionAvailable: err == nil, ResourceAvailable: err == nil}
	if err != nil {
		evidence.Diagnostic = "Original terminal unavailable; replacement recovery is unsupported: " + err.Error()
		return evidence, err
	}
	if l.profile.Terminal != nil {
		state, e := l.state(s)
		if e != nil {
			return evidence, e
		}
		if e = l.readTerminalReadiness(&state, false); e != nil {
			return evidence, e
		}
		active := state.TerminalReadiness.ActiveCommandID
		if active == "" && len(state.Turns) == 0 {
			evidence.SessionIdle = true
			return evidence, nil
		}
		turn := turnFor(&state, active)
		if turn == nil {
			return evidence, errors.New("original native idle boundary unavailable")
		}
		diagnostic := l.observeTerminal(&state, turn)
		evidence.SessionIdle = turn.Extension != nil && turn.Extension.Stage == "settled" && (diagnostic == "" || strings.HasPrefix(diagnostic, "Original native turn failed: "))
		if !evidence.SessionIdle {
			evidence.Diagnostic = "Original native agent remains active/unverified; no recovery input is allowed"
		}
		return evidence, nil
	}
	// A rendered idle screen can belong to a previous turn. Qualification uses
	// the same original native main-agent source as normal observation.
	feed, err := l.stateFactory(ctx, l.profile.Environment, s.Ref.External)
	if err != nil {
		return evidence, err
	}
	defer feed.Close()
	initial := feed.Initial()
	state, err := l.state(s)
	if err != nil {
		return evidence, err
	}
	provider, pane := "", ""
	for _, turn := range state.Turns {
		if turn.Command == s.Session.CurrentCommandID || turn.Command == s.Session.PendingCommandID {
			if turn.Native.Provider != "" {
				provider, pane = turn.Native.Provider, turn.Native.Pane
			}
		}
	}
	current, err := feed.Query(ctx)
	if err != nil {
		return evidence, err
	}
	idle := current.PaneKey == initial.PaneKey && initial.PaneKey != "" && current.AgentType == s.Ref.Agent.Name &&
		current.ProviderSession.ID != "" && current.ProviderSession.ID == initial.ProviderSession.ID &&
		(provider == "" || current.ProviderSession.ID == provider) &&
		(pane == "" || current.PaneKey == pane) &&
		current.MainAgent != nil && current.MainAgent.State == "done" && current.MainAgent.StateStartedAt > 0 &&
		current.UpdatedAt >= current.MainAgent.StateStartedAt && !current.Interrupted && (current.MainAgent.Outcome == "success" || s.Ref.Agent.Name == "codex" && current.MainAgent.Outcome == "") &&
		(initial.MainAgent == nil || current.MainAgent.StateStartedAt >= initial.MainAgent.StateStartedAt)
	if _, err := l.original(ctx, s); err != nil {
		return b.LifecycleRecoveryEvidence{}, err
	}
	evidence.SessionIdle = idle
	if !evidence.SessionIdle {
		evidence.Diagnostic = "Original native main agent is active/unverified; no recovery input is allowed"
	}
	return evidence, nil
}
