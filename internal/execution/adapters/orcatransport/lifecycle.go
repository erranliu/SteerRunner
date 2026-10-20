// Package orcatransport implements terminal communication, not worker scheduling.
package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type Turn struct {
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
	terminalMu       sync.Mutex
	terminalJob      b.ProcessTreeHandle
	setup            b.LifecycleSetup
	profile          Profile
	profileVersion   int
	client           native.Client
	stateFactory     native.StateFeedFactory
	queryNative      func(context.Context, b.ProcessIdentity, string) (native.AgentState, error)
	feed             native.StateFeed
	feedProcess      b.ProcessIdentity
	feedMu           sync.Mutex
	feedRetryAt      time.Time
	clock            func() time.Time
	nativeAccess     bool
	readStopArtifact func(context.Context, string, string, string, string) ([]byte, error)
	inspectionOnce   sync.Once
	inspectionSerial chan struct{}
}

func NewLifecycle(setup b.LifecycleSetup) (b.LifecyclePort, error) {
	port, err := NewLifecycleWithStateFactory(setup, native.OpenStateFeed)
	if err == nil {
		port.(*Lifecycle).nativeAccess = true
		l := port.(*Lifecycle)
		l.queryNative = func(ctx context.Context, process b.ProcessIdentity, pane string) (native.AgentState, error) {
			return native.QueryOriginalAgent(ctx, l.profile.Environment, process, pane)
		}
	}
	return port, err
}

// The explicit port supplies isolated native-response fixtures without adding
// simulation flags to profiles or the shared business workflow.
func NewLifecycleWithStateFactory(setup b.LifecycleSetup, factory native.StateFeedFactory) (b.LifecyclePort, error) {
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
	l := &Lifecycle{setup: setup, profile: p, profileVersion: version, stateFactory: factory, readStopArtifact: native.ReadStopArtifact, clock: func() time.Time { return time.Now().UTC() }}
	call := func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		all := append(append([]string(nil), p.Args...), args...)
		blocked := []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}
		if p.Environment != "" {
			all = append(all, "--environment", p.Environment)
		}
		invoke := func(ctx context.Context) ([]byte, error) {
			return setup.ProcessHost.OutputCommand(ctx, b.LaunchCommand{Executable: p.Executable, Args: all, WorkingDirectory: setup.RepositoryPath, Timeout: timeout, BlockedEnvironment: blocked})
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
	l.client = native.New(native.Config{CommandTimeout: "40s", Target: p.Target(), Output: call})
	return l, nil
}

func (l *Lifecycle) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	resource := b.MechanismNotApplicable
	if l.setup.AgentLaunchCommand != "" {
		resource = b.MechanismApplicable
	}
	policy := "host_managed_permissions; business_read_only_is_prompt_policy"
	if l.profile.Environment != "" {
		policy = "configured_native_launch_parameters; execution_host_existing_authentication; business_read_only_is_prompt_policy"
	}
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: resource, Session: b.MechanismApplicable}, ServiceStopBehavior: ServiceStopBehavior, StructuredFinalEvidence: true, ReportToolPolicy: policy, ResumeActiveObservation: true, LinkedWorkspaceWriteRoots: false, SeparateTurnStart: true}
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

func (l *Lifecycle) state(s b.LifecycleSnapshot) (State, error) {
	if err := l.validate(s); err != nil {
		return State{}, err
	}
	state := State{}
	legacyRemote := l.profile.Environment != "" && l.profileVersion == ProfileVersion
	found := false
	for _, e := range s.Implementation {
		if e.ImplementationKey == StateKey {
			// The retired remote v5 payload is read only to fence an explicit
			// stop to its original Process; no helper state is restored.
			if found || e.CodecVersion != StateVersion && (!legacyRemote || e.CodecVersion != 5) {
				return state, errors.New("unsupported terminal state")
			}
			found = true
			if err := e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest); err != nil {
				return state, err
			}
			if legacyRemote {
				// Legacy stop extracts only the original identity, never an old
				// helper reader/control runtime or its historical turn codec.
				var original struct {
					Process b.ProcessIdentity `json:"process"`
				}
				if err := json.Unmarshal(e.Payload, &original); err != nil {
					return state, err
				}
				state.Process = original.Process
				continue
			}
			if err := b.DecodeStrictJSON(e.Payload, &state, "trailing terminal state"); err != nil {
				return state, err
			}
		}
	}
	if state.Process.Terminal != "" && state.Process != s.Ref.External {
		return state, errors.New("saved original terminal differs from current resource")
	}
	if l.profile.Environment != "" {
		if l.profileVersion != NativeRemoteProfileVersion {
			return state, nil // Only legacy stop may extract the original identity.
		}
		if state.Control != nil || state.RemoteReadiness != nil || state.TerminalOwner != nil || state.TerminalReadiness != nil || state.AdmissionAttempted || state.AdmissionAccepted {
			return state, errors.New("native remote state contains helper preparation")
		}
		for _, turn := range state.Turns {
			if turn.Recorded != nil || turn.Extension != nil || turn.WireFormat != "" || turn.ReceiptRequest != "" || !turn.ReceiptQueriedAt.IsZero() {
				return state, errors.New("native remote state contains helper turn evidence")
			}
		}
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

func (l *Lifecycle) envelope(s b.LifecycleSnapshot, state State) ([]b.ImplementationDataEnvelope, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: StateVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: StateKey, ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}, raw)
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

func (l *Lifecycle) original(ctx context.Context, s b.LifecycleSnapshot) (native.Terminal, error) {
	state, stateErr := l.state(s)
	if stateErr != nil {
		return native.Terminal{}, stateErr
	}
	initializing := l.setup.AgentLaunchCommand != "" && !state.LaunchConfirmed
	t, err := l.client.InspectBoundTerminal(ctx, l.profile.Target(), s.Ref.External)
	if err != nil {
		return t, err
	}
	if t.WorktreePath != s.Workspace.Path || strings.TrimPrefix(t.Branch, "refs/heads/") != s.Workspace.Branch && !(initializing && t.Branch == "") {
		return t, errors.New("original Orca workspace or branch changed")
	}
	// A connected native background terminal can be unadopted by the UI
	// (orphaned=true). Its original runtime/incarnation/worktree and agent must
	// still match; adoption into a visible tab is not execution identity.
	// This label is a display heuristic: the native DSH TUI can be labeled Gemini.
	// Managed agents instead require their original OS owner and native receipt
	// before every business input/observation. PTY/runtime/incarnation still bind.
	unknownNativeAgent := l.profile.Terminal != nil
	if t.WorktreeID != s.Ref.External.WorktreeID || t.AgentIdentity != s.Ref.Agent.Name && !(initializing && t.AgentIdentity == "") && !unknownNativeAgent || !t.Connected || t.Exited() {
		return t, fmt.Errorf("original Orca agent/worktree changed (worktree=%q expected=%q agent=%q orphaned=%t)", t.WorktreeID, s.Ref.External.WorktreeID, t.AgentIdentity, t.Orphaned)
	}
	return t, nil
}

func (l *Lifecycle) PreflightLifecycle(ctx context.Context, setup b.LifecycleSetup) (b.CapabilityReport, error) {
	if err := l.validate(setup.Snapshot); err != nil {
		return b.CapabilityReport{}, err
	}
	if err := l.agentExecutionSupport(setup.Snapshot.Ref.Agent.Name); err != nil {
		return b.CapabilityReport{}, err
	}
	if (setup.Snapshot.Ref.Agent.Name == "pi" || setup.Snapshot.Ref.Agent.Name == "dsh") && (l.profile.Environment != "" || l.profile.Terminal == nil || len(l.profile.RuntimeSHA256) != 64) {
		return b.CapabilityReport{}, errors.New("terminal agent requires a frozen local managed runtime and native receipt profile; zero business input")
	}
	r, err := l.client.MetaCall(ctx, "status", "--json")
	if err != nil {
		return b.CapabilityReport{}, fmt.Errorf("Orca terminal preflight: %w", err)
	}
	var status struct {
		Runtime struct {
			State        string   `json:"state"`
			AppVersion   string   `json:"appVersion"`
			Capabilities []string `json:"capabilities"`
		} `json:"runtime"`
	}
	if json.Unmarshal(r.Payload, &status) != nil || r.RuntimeID == "" || status.Runtime.State != "ready" {
		return b.CapabilityReport{}, errors.New("Orca runtime is not ready")
	}
	supported := false
	for _, cap := range status.Runtime.Capabilities {
		if cap == "terminal.prompt-delivery.v1" {
			supported = true
		}
	}
	if !supported {
		return b.CapabilityReport{}, errors.New("Orca prompt identity/replay capability is unavailable")
	}
	if l.nativeAccess {
		if l.profile.Environment == "" {
			if err := l.verifyLocalObservation(ctx, r.RuntimeID); err != nil {
				return b.CapabilityReport{}, err
			}
		} else if _, err := native.QueryWorktreeAgents(ctx, l.profile.Environment, r.RuntimeID); err != nil {
			return b.CapabilityReport{}, fmt.Errorf("Orca native observation authentication/access: %w", err)
		}
	}
	return b.CapabilityReport{Version: 1, Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: setup.Snapshot.Ref.Target, Agent: setup.Snapshot.Ref.Agent, PlatformVersion: status.Runtime.AppVersion, CheckedAt: time.Now().UTC(), Capabilities: []b.Capability{{Name: "start", Status: b.CapabilityVerified}, {Name: "observe", Status: b.CapabilityVerified}, {Name: "output_final_answer", Status: b.CapabilityVerified, Evidence: "original accepted request, bound native main-agent start/end and complete native final body"}}}, nil
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
	if l.profile.Environment != "" && l.profileVersion != NativeRemoteProfileVersion {
		return &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "legacy_remote_helper_unsupported: original snapshots and exact terminal stop only; start a new native Orca Run"}
	}
	if agent != "codex" && !((agent == "pi" || agent == "dsh") && l.profile.Terminal != nil && l.profile.Terminal.Agent == agent && l.profile.Environment == "" && len(l.profile.RuntimeSHA256) == 64) {
		// Orca 1.4.224 lacks the start, root outcome and permission preparation contract.
		return &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "terminal_agent_evidence_unsupported: selected agent lacks verified native turn and permission preparation; zero business input"}
	}
	return nil
}

func (l *Lifecycle) PreparationFacts(_ context.Context, s b.LifecycleSnapshot) (b.PreparationFacts, error) {
	if err := l.validate(s); err != nil {
		return b.PreparationFacts{}, err
	}
	if err := l.agentExecutionSupport(s.Ref.Agent.Name); err != nil {
		return b.PreparationFacts{}, err
	}
	if (s.Ref.Agent.Name == "pi" || s.Ref.Agent.Name == "dsh") && (l.profile.Terminal == nil || l.profile.Environment != "" || len(l.profile.RuntimeSHA256) != 64) {
		return b.PreparationFacts{}, errors.New("native terminal preparation profile missing; zero business input")
	}
	if l.profile.Environment == "" && strings.Contains(strings.ToLower(l.setup.RepositoryURL), "github.com") && l.profile.RuntimeExecutable == "" {
		return b.PreparationFacts{}, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "remote_sandbox_authentication_unsupported: Orca terminal transport has no trusted execution-host credential preparation and deterministic same-sandbox command capability; zero business input"}
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
	if turnFor(&state, s.PendingTurn.CommandID) != nil {
		gate = b.GateReady
	}
	return b.PreparationFacts{Workspace: b.GateReady, Resource: resource, Session: gate}, nil
}

func (l *Lifecycle) Execute(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
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
		return receipt, l.observe(snapshot), nil
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
		// Only failed read-only connections qualify. The protocol bounds each
		// dial so a stalled connection leaves room in this readiness budget.
		current, err := l.readyOnce(readyCtx, s)
		if err == nil {
			return current, nil
		}
		if !native.IsStateConnectionError(err) {
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
			return current, fmt.Errorf("original native readiness unavailable; no input was sent: %w", err)
		}
		if _, err := l.original(readyCtx, s); err != nil {
			return current, err
		}
		if current.PaneKey != feed.Initial().PaneKey || current.AgentType != s.Ref.Agent.Name {
			return current, errors.New("original native main-agent identity changed; no input was sent")
		}
		if current.MainAgent != nil && current.ProviderSession.ID != "" {
			switch current.MainAgent.State {
			case "done", "idle":
				return current, nil
			case "waiting", "blocked":
				return current, fmt.Errorf("original native main agent is %s; no input was sent", current.MainAgent.State)
			}
		}
		select {
		case <-readyCtx.Done():
			return current, fmt.Errorf("original native main-agent readiness timed out; no input was sent: %w", readyCtx.Err())
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
	if l.profile.Environment == "" {
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
	inputDigest := digest([]byte(input.Prompt))
	if turn := turnFor(&state, input.CommandID); turn != nil {
		if turn.InputDigest != inputDigest {
			return receipt, errors.New("prepared worker input changed")
		}
	} else {
		if l.profile.Terminal != nil {
			if _, e := l.original(ctx, s); e != nil {
				return receipt, e
			}
			if e := l.readTerminalReadiness(&state, true); e != nil {
				return receipt, e
			}
			state.Turns = append(state.Turns, Turn{Command: input.CommandID, Prompt: input.Prompt, InputDigest: inputDigest, WireFormat: extensionWireCommand, Native: NativeProof{Agent: s.Ref.Agent.Name}})
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
			prepared := Turn{Command: input.CommandID, Prompt: input.Prompt, InputDigest: inputDigest, Native: baseline, Model: input.Model, ReasoningEffort: input.ReasoningEffort, StepID: input.StepID}
			if l.profile.Environment == "" && state.RemoteReadiness != nil && state.RemoteReadiness.NativeTransport == codex.RemoteNativeTransport {
				prepared.WireFormat = remoteWireCommand
				if input.Model != "" {
					prepared.WireFormat = remoteModelWireCommand
				}
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
	if l.profile.Environment == "" {
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
	if l.profile.Terminal != nil && l.profile.Terminal.Agent == "dsh" {
		// ConPTY's paste ingress uses CR for line breaks. DSH normalizes these
		// back to LF in the original composer; raw LF is lost by its Windows
		// input path. Keep logical framing/digests unchanged and verify the
		// exact committed native text, as for every other input.
		text = "\x1b[200~" + strings.ReplaceAll(text, "\n", "\r") + "\x1b[201~"
	}
	args := []string{"terminal", "send", "--terminal", state.Process.Terminal, "--text", text, "--enter", "--retry-request", turn.Request, "--json"}
	if l.profileVersion == NativeRemoteProfileVersion {
		// Orca's default receipt may precede the original turn-start observation.
		// Wait on this single accepted request; timeout never resends input and
		// still leaves Started false unless the bound receipt proves it.
		args = append(args, "--wait-submit", "20")
	}
	result, callErr := l.client.MetaCallWithTimeout(ctx, 50*time.Second, args...)
	receipt.Operation.Receipt = append([]byte(nil), result.Payload...)
	var sent struct {
		Send struct {
			Handle   string `json:"handle"`
			Accepted *bool  `json:"accepted"`
			Prompt   struct {
				RequestID          string   `json:"requestId"`
				ProcessIncarnation string   `json:"processIncarnation"`
				Stages             []string `json:"stages"`
			} `json:"prompt"`
		} `json:"send"`
	}
	bound := json.Unmarshal(result.Payload, &sent) == nil && result.RuntimeID == state.Process.RuntimeID && sent.Send.Handle == state.Process.Terminal && sent.Send.Prompt.RequestID == turn.Request && sent.Send.Prompt.ProcessIncarnation == state.Process.Incarnation && !result.CallUncertain && result.ErrorCode == ""
	if bound && sent.Send.Accepted != nil && !*sent.Send.Accepted && len(sent.Send.Prompt.Stages) == 0 {
		turn.Rejected = true
		receipt.Disposition = b.OperationRejected
		receipt.Implementation, err = l.envelope(s, state)
		l.closeStateFeed()
		return receipt, errors.Join(err, errors.New("original native input explicitly rejected; no input was accepted"))
	}
	if !bound || sent.Send.Accepted == nil || !*sent.Send.Accepted {
		l.closeStateFeed()
		return receipt, errors.Join(callErr, errors.New("original terminal input outcome is unknown; do not resend"))
	}
	turn.Accepted = true
	for _, stage := range sent.Send.Prompt.Stages {
		if stage == "turn_started" && l.profile.Terminal == nil {
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
func (l *Lifecycle) observeOnce(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
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
		return receipt, nil, nil
	}
	if !turn.Settled {
		return receipt, nil, nil
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
		receipt.TurnEvidence.FinalNativeSessionID = state.RemoteReadiness.SessionID
		receipt.TurnEvidence.FinalNativeTurnID = turn.Recorded.NativeTurnID
	}
	return receipt, nil, nil
}

// Recovery resumes observation of the same resource and saved input/baseline only.
func (l *Lifecycle) ObserveLifecycle(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if err := l.agentExecutionSupport(s.Ref.Agent.Name); err != nil {
		return b.LifecycleReceipt{}, nil, err
	}
	receipt, _, err := l.observeOnce(ctx, s)
	if err != nil {
		l.closeStateFeed()
		return receipt, nil, err
	}
	if receipt.TurnEvidence != nil || receipt.Process == nil {
		return receipt, nil, err
	}
	snapshot := s.Clone()
	snapshot.Implementation = receipt.Implementation
	return receipt, l.observe(snapshot), nil
}

func (l *Lifecycle) Reconcile(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
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
	if l.profile.Environment != "" {
		return l.remoteStopResult(ctx, r, true)
	}
	progress := stopProgress{Version: 1, Phase: "not-attempted"}
	defer func() { receipt.Operation.Receipt, _ = json.Marshal(progress) }()
	if h, ok := owned.(*observationHandle); ok {
		if h.Identity() != r.Snapshot.Ref.External {
			return receipt, errors.New("stop changed original observation identity")
		}
		_ = h.owner.Close(ctx)
		// Do not await an event acknowledgement from the actor performing stop.
		// The control gate drains in-flight reads; its revocation checkpoint
		// rejects later queries even before this goroutine has fully returned.
	}
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
		// Let the helper close its broker and commit its exit before closing
		// the PTY. Closing the PTY first can terminate that cleanup mid-write.
		if owner.Job.Handle != 0 && l.profile.Terminal.ReceiptVersion >= 2 {
			until := time.Now().Add(3 * time.Second)
			for !l.terminalExitVerified(owner) && time.Now().Before(until) {
				time.Sleep(20 * time.Millisecond)
			}
			if !l.terminalExitVerified(owner) {
				return receipt, errors.New("original native terminal exit/authentication cleanup unverified")
			}
		}
	}

	state, stateErr := l.state(r.Snapshot)
	if stateErr != nil {
		return receipt, stateErr
	}
	var authErr error
	if l.profile.Terminal == nil {
		// An original pending read may have consumed the caller's stop budget.
		// Finish that same actor/request within a separate bounded cleanup budget.
		authCtx, cancelAuth := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
		authErr = l.revokeAuthentication(authCtx, state)
		cancelAuth()
	}
	// Stop authorization is already durable. Credential uncertainty cannot keep
	// the original worker running; finish its exact PTY closure within a bounded
	// cleanup context even if the preceding credential observation timed out.
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancelStop()
	if err = l.originalForStop(stopCtx, r.Snapshot); err != nil {
		return receipt, errors.Join(authErr, err)
	}
	if l.usesControl() {
		if err = l.stopCodexNative(stopCtx, state, r.Snapshot.Workspace.Path); err != nil {
			return receipt, errors.Join(authErr, err)
		}
	}
	progress.Phase = "attempted"
	proof, err := l.client.CloseTerminalChecked(stopCtx, r.Snapshot.Ref.External.Terminal, r.Snapshot.Ref.External.RuntimeID)
	if err != nil {
		return receipt, errors.Join(authErr, err)
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
				return receipt, errors.New("original native terminal exit/authentication cleanup unverified")
			}
			l.closeTerminalJob()
		}
		progress.Phase = "business-confirmed"
		receipt.State.WorkerAbsent = true
		state, stateErr := l.state(r.Snapshot)
		if stateErr != nil {
			return receipt, stateErr
		}
		// Do not destroy the sole route to an uncommitted revocation proof.
		// The known original control actor remains owned and blocks settlement.
		if authErr != nil {
			return receipt, authErr
		}
		if cleanupErr := l.closeControl(stopCtx, state); cleanupErr != nil {
			return receipt, cleanupErr
		}
		if l.profile.Terminal == nil && l.profile.RuntimeExecutable != "" {
			// Orca's acknowledged PTY close can precede its published inventory.
			// Poll only the same original identity; never repeat the close.
			until := time.Now().Add(3 * time.Second)
			for {
				if err := l.verifyStopped(stopCtx, r.Snapshot); err == nil {
					break
				} else if time.Now().After(until) || stopCtx.Err() != nil {
					return receipt, err
				}
				time.Sleep(50 * time.Millisecond)
			}
		}

		progress.Phase = "confirmed"
		receipt.Disposition = b.OperationConfirmed
		receipt.State.Stopped = true
		receipt.State.WorkerAbsent = true
	} else {
		receipt.Disposition = b.OperationAccepted
	}
	return receipt, nil
}

func (l *Lifecycle) CleanupLifecycle(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, error) {
	if r.Input.Cleanup == nil || len(r.Input.Cleanup.Resources) != 1 || r.Input.Cleanup.Resources[0] != r.Snapshot.Ref.External {
		return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}, errors.New("cleanup differs from original terminal")
	}
	copy := r
	copy.Operation.Action = string(b.LifecycleStopExecution)
	copy.Input = b.LifecycleInput{Stop: &b.StopInput{}}
	// Keep the original operation binding; cleanup and stop have the same exact resource effect.
	receipt, err := l.StopLifecycle(ctx, copy, nil)
	receipt.Operation = r.Operation
	return receipt, err
}

func (l *Lifecycle) ShutdownLifecycle(ctx context.Context, s b.LifecycleSnapshot, owned b.StopHandle) (b.LifecycleStateFacts, error) {
	if err := l.validate(s); err != nil {
		return b.LifecycleStateFacts{}, err
	}
	if l.profile.Environment != "" {
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
	_, err := l.state(s)
	if err != nil {
		return b.LifecycleStateFacts{}, err
	}
	if err = l.verifyStopped(ctx, s); err != nil {
		return b.LifecycleStateFacts{}, err
	}
	return b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}, nil
}
func (l *Lifecycle) InspectLifecycleWorkspace(ctx context.Context, s b.LifecycleSnapshot) (b.WorkspaceInspection, error) {
	return InspectWorkspace(ctx, l.client, s.Ref, s.Session.ProjectID)
}
func (l *Lifecycle) VerifyManualRecovery(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
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
func (l *Lifecycle) BindRecovery(_ context.Context, original, replacement b.LifecycleSnapshot) (b.ImplementationDataEnvelope, error) {
	return b.ImplementationDataEnvelope{}, errors.New("replacing an Orca session requires a new execution")
}
