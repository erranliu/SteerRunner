package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// frozenRecord reads original facts. It has no execution, input, observer,
// state feed, process Job or mutation method and owns no durable state.
type frozenRecord struct {
	setup            b.LifecycleSetup
	profile          Profile
	profileVersion   int
	client           native.Client
	probeProcess     func(*b.ProcessIdentity) b.ProcessProbeFacts
	agentHostID      string
	agentDevelopment bool
	agentAudited     bool
}

func decodeFrozenRecord(setup b.LifecycleSetup) (*frozenRecord, error) {
	if setup.Snapshot.Ref.Agent.Name != "codex" && setup.Snapshot.Ref.Agent.Name != "pi" {
		return nil, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Orca agent execution/history is unsupported; only Codex and Pi are supported."}
	}
	setup = setup.Clone()
	if setup.ProcessHost == nil {
		return nil, errors.New("unsupported Orca terminal lifecycle")
	}
	r := &frozenRecord{setup: setup, probeProcess: process.NewServiceFacts().Probe}
	if r.structured() {
		p, err := decodeAgentSessionProfile(setup)
		if err != nil {
			return nil, err
		}
		r.agentHostID = p.PairingHostID
		r.agentDevelopment = p.Development
		r.agentAudited = p.Audited
		r.profile = p.Transport
		r.profileVersion = p.version
	} else {
		p, version, err := decodeLifecycleProfile(setup)
		if err != nil {
			return nil, err
		}
		r.profile = p
		r.profileVersion = version
	}
	return r, nil
}

func newFrozenReader(setup b.LifecycleSetup) (*frozenRecord, error) {
	r, err := decodeFrozenRecord(setup)
	if err != nil {
		return nil, err
	}
	output := orcaCLIOutput(r.setup, r.profile, r.agentHostID, func() bool { return true })
	r.client = native.New(native.Config{CommandTimeout: "40s", Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if len(args) < 2 || args[0] != "terminal" || args[1] != "list" && args[1] != "show" {
			return nil, errors.New("frozen Orca proof does not permit mutation")
		}
		return output(ctx, timeout, args...)
	}})
	return r, nil
}

// Runtime views share the exact decoder/validation with offline proofs without
// giving the frozen reader an execution owner or state-feed dependency.
func (l *Lifecycle) frozenRecord() *frozenRecord {
	return &frozenRecord{setup: l.setup, profile: l.profile, profileVersion: l.profileVersion, client: l.client, probeProcess: l.probeProcess, agentHostID: l.agentHostID, agentDevelopment: l.agentDevelopment, agentAudited: l.agentAudited}
}

func orcaCLIOutput(setup b.LifecycleSetup, p Profile, host string, nativeAccess func() bool) func(context.Context, time.Duration, ...string) ([]byte, error) {
	return func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		command := cliLaunchCommand(p, p.Executable, setup.RepositoryPath, timeout, args...)
		if err := process.CheckCommandLine(command.Executable, command.Args); err != nil {
			return nil, err
		}
		invoke := func(ctx context.Context) ([]byte, error) {
			if setup.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 && p.Environment != "" {
				actual, err := native.UsageHostID(p.Environment)
				if err != nil || actual != host {
					return nil, errors.New("original structured Orca pairing changed or is unavailable")
				}
			}
			return setup.ProcessHost.OutputCommand(ctx, command)
		}
		if native.PreparationOwnsCLI(ctx) {
			return invoke(ctx)
		}
		if nativeAccess() {
			callCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return native.SerializeCLI(callCtx, invoke)
		}
		if err := native.CheckpointDispatch(ctx); err != nil {
			return nil, err
		}
		return invoke(ctx)
	}
}

func (l *frozenRecord) structured() bool {
	return l.setup.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1
}

func (l *frozenRecord) validateStructured(s b.LifecycleSnapshot) error {
	if err := l.validate(s); err != nil {
		return err
	}
	if s.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || s.Ref.Agent.Name != "codex" && s.Ref.Agent.Name != "pi" {
		return errors.New("unsupported structured Orca identity")
	}
	return nil
}

func (l *frozenRecord) agentState(s b.LifecycleSnapshot) (agentSessionState, error) {
	var state agentSessionState
	if err := l.validateStructured(s); err != nil {
		return state, err
	}
	found := false
	for _, e := range s.Implementation {
		if e.ImplementationKey != AgentSessionStateKey {
			continue
		}
		if found || e.CodecVersion != AgentSessionVersion {
			return state, errors.New("invalid structured Orca state codec")
		}
		found = true
		if err := e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest); err != nil {
			return state, err
		}
		if err := b.DecodeStrictJSON(e.Payload, &state, "trailing structured Orca state"); err != nil {
			return state, err
		}
	}
	if found && (state.RuntimeID == "" || state.Worktree.ID == "" || state.Worktree.InstanceID == "" || state.SessionID != agentSessionID(s) || state.Worktree.ID != s.Ref.External.WorktreeID || state.RuntimeID != s.Ref.External.RuntimeID || s.Ref.External.Kind != "orca_agent_session" || s.Ref.External.Executable != l.profile.Executable || s.Ref.External.ExecutionHost != l.profile.Target().ExecutionHost || s.Ref.External.Terminal != "" || s.Ref.External.PID != 0 || s.Ref.External.Incarnation != "" || state.Worktree.Path != s.Workspace.Path) {
		return state, errors.New("original structured Orca resource binding changed")
	}
	if len(state.CloseReceipt) > 0 && (!state.CloseAttempted || !agentCloseSucceeded(state.CloseReceipt)) {
		return state, errors.New("original structured close receipt is invalid")
	}
	if state.SessionNotCreated && agentSessionMayExist(s, state) {
		return state, errors.New("structured session absence contradicts its original dispatch")
	}
	if state.CommandID != "" && (!b.StableExecutionID(state.CommandID) || len(state.InputDigest) != 64 || len(state.BodyFingerprint) != 64 || state.CreateID == "" || state.MessageID == "") {
		return state, errors.New("incomplete original structured input binding")
	}
	if s.PendingTurn != nil && state.CommandID != "" && (state.CommandID == s.PendingTurn.CommandID && state.InputDigest != digest([]byte(s.PendingTurn.Prompt)) || !l.agentDevelopment && state.CommandID != s.PendingTurn.CommandID) {
		return state, errors.New("original structured input changed")
	}
	return state, nil
}

func (l *frozenRecord) closeSettlesAgentSession() bool {
	return l.profileVersion == AgentSessionCloseProfileVersion || l.profile.Environment == "" && !l.agentDevelopment
}

func (l *frozenRecord) verifyAgentSessionStopped(s b.LifecycleSnapshot) error {
	if !l.closeSettlesAgentSession() {
		if l.agentDevelopment {
			return errors.New("Orca 原会话 close 不提供完整退出证据；资源结算未核实 (resources_settled=false).")
		}
		return errors.New(AgentSessionStopDiagnostic)
	}
	state, err := l.agentState(s)
	if err != nil {
		return err
	}
	if !state.CloseAttempted || !agentCloseSucceeded(state.CloseReceipt) && !(state.SessionNotCreated && !agentSessionMayExist(s, state)) {
		return errors.New(AgentSessionStopDiagnostic)
	}
	return nil
}

func (l *frozenRecord) usesControl() bool {
	return l.profile.Environment == "" && l.profile.Terminal == nil && l.profile.RuntimeExecutable != ""
}

func (l *frozenRecord) controlSetup(s b.LifecycleSnapshot, state State) codex.RemoteControlSetup {
	workspace := s.Workspace.Path
	if state.RemoteReadiness != nil {
		workspace = state.RemoteReadiness.Workspace
	}
	if workspace == "" && state.Control != nil && state.Control.Ready != nil {
		workspace = state.Control.Ready.Workspace
	}
	raw, _ := json.Marshal(struct {
		Project, Execution, Attempt, Generation, Configuration, Helper string
		Process                                                        b.ProcessIdentity
		Workspace                                                      string
	}{s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest, l.profile.RuntimeSHA256, state.Process, strings.ToLower(strings.ReplaceAll(workspace, "\\", "/"))})
	return codex.RemoteControlSetup{Binding: digest(raw), Workspace: workspace, RuntimeSHA256: l.profile.RuntimeSHA256}
}

func (l *frozenRecord) validateControl(s b.LifecycleSnapshot, state State, ready bool) error {
	c := state.Control
	if c == nil {
		return errors.New("original native control identity unavailable; no replacement created")
	}
	p := c.Process
	if p.Kind != "orca_native_control" || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == "" || p.Terminal == state.Process.Terminal || p.Incarnation == "" {
		return errors.New("original native control resource changed")
	}
	expected := l.controlSetup(s, state)
	if c.Ready != nil && (c.Ready.Binding != expected.Binding || c.Ready.RuntimeSHA256 != expected.RuntimeSHA256 || !strings.EqualFold(strings.ReplaceAll(c.Ready.Workspace, "\\", "/"), strings.ReplaceAll(expected.Workspace, "\\", "/"))) || ready && c.Ready == nil {
		return errors.New("original native control readiness unavailable or changed")
	}
	return nil
}

func (l *frozenRecord) validate(s b.LifecycleSnapshot) error {
	w := l.setup.Snapshot
	if s.Ref.Backend != w.Ref.Backend || s.Ref.Protocol != w.Ref.Protocol || s.Ref.Target.ID != w.Ref.Target.ID || s.Ref.Agent != w.Ref.Agent || s.Ref.RunID != w.Ref.RunID || s.Ref.AttemptID != w.Ref.AttemptID || s.Session.ProjectID != w.Session.ProjectID || s.Session.WorkflowRunID != w.Session.WorkflowRunID || s.Ref.Target.ExternalRuntime != l.profile.Target().ExternalRuntime || s.Ref.Target.ExecutionHost != l.profile.Target().ExecutionHost || s.Session.ExecutionID != w.Session.ExecutionID || s.Session.AttemptID != w.Session.AttemptID || s.ConfigurationDigest != w.ConfigurationDigest || s.ResourceGeneration != w.ResourceGeneration {
		return errors.New("Orca terminal execution binding changed")
	}
	if s.Session.ProjectID == "" || s.ConfigurationDigest == "" || s.ResourceGeneration == "" {
		return errors.New("incomplete Orca terminal execution")
	}
	return nil
}

func (l *frozenRecord) resourceState(s b.LifecycleSnapshot) (State, error) {
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

func (l *frozenRecord) state(s b.LifecycleSnapshot) (State, error) {
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

func (l *frozenRecord) stateVersion() int { return l.profile.stateCodec() }

func (l *frozenRecord) localCodexScope() bool { return l.StopEvidenceScope() == LocalCodexStopScope }

func (l *frozenRecord) localStopObjects(s b.LifecycleSnapshot, state State) ([]b.ProcessIdentity, []b.ProcessIdentity, error) {
	p := state.Process
	if p != s.Ref.External || p.Kind != "orca_terminal" || p.ExecutionHost != l.profile.Target().ExecutionHost || p.Executable != l.profile.Executable {
		return nil, nil, errors.New("original local Orca resource binding is incomplete")
	}
	terminals := []b.ProcessIdentity{p}
	if state.Control != nil {
		terminals = append(terminals, state.Control.Process)
	}
	for _, op := range s.Operations {
		if (op.Action == string(b.LifecycleCreateResource) || op.Action == string(b.LifecyclePrepareWorkspace)) && (op.Status == b.OperationInFlight || op.Status == b.OperationUnknown) {
			return terminals, nil, errors.New("original resource creation remains unresolved")
		}
	}
	if state.RemoteReadiness == nil && !state.LaunchAttempted && len(state.Turns) == 0 {
		return terminals, nil, nil // Positive pre-launch state, not missing evidence after launch.
	}
	ready, err := l.codexReadiness(state, s.Workspace.Path)
	if err != nil {
		return terminals, nil, err
	}
	if state.Control == nil {
		return terminals, nil, errors.New("original local Codex control identity missing")
	}
	return terminals, []b.ProcessIdentity{ready.Owner, ready.Process}, nil
}

func (l *frozenRecord) verifyLocalCodexStopped(ctx context.Context, s b.LifecycleSnapshot) error {
	state, err := l.resourceState(s)
	if err != nil {
		return err
	}
	terminals, roots, err := l.localStopObjects(s, state)
	if err != nil {
		return err
	}
	// This one inventory proves both terminals on the original runtime's local host.
	if err := VerifyTerminalsStopped(ctx, l.client, terminals); err != nil {
		return err
	}
	for _, root := range roots {
		if fact := l.probeProcess(&root); fact.State != "absent" {
			return fmt.Errorf("original local process %d is %s: %s", root.PID, fact.State, fact.Reason)
		}
	}
	return ctx.Err()
}

func (l *frozenRecord) StopEvidenceScope() string {
	if l.structured() {
		return AgentSessionStopScope
	}
	if l.profile.Environment != "" && l.profileVersion != NativeRemoteProfileVersion && l.profileVersion != NativeProfileVersion {
		return ""
	}
	return StopScope(l.profile, l.setup.Snapshot.Ref.Agent.Name)
}

func (l *frozenRecord) validateTerminalReceipt(r TerminalReceipt) error {
	c := l.profile.Terminal
	if c == nil || l.profile.Environment != "" {
		return errors.New("local native terminal profile missing")
	}
	sourceKind, tools := "pi-extension-v1", []string{"read", "write", "edit", "powershell"}
	expectedSHA := c.EvidenceSHA256
	if expectedSHA == "" || c.BundleSHA256 == "" {
		return errors.New("original frozen terminal source hashes missing")
	}
	version := c.ReceiptVersion
	preparationValid := validTerminalPreparation(r.Preparation, version)

	if r.Version != version || r.Source != sourceKind || r.SourceSHA256 != expectedSHA || r.RunID != c.ExecutionID || r.SessionID != c.SessionID || r.InstanceID == "" || r.PID <= 0 || r.Mode != "tui" || r.ProcessStartedAt.IsZero() || !reflect.DeepEqual(r.Tools, tools) || !r.Preparation.Verified || !preparationValid || r.Preparation.AccessMode != c.AccessMode || !r.Preparation.ProviderEnvironmentAbsent || c.AccessMode != "full_access" {
		return errors.New("native terminal receipt binding changed")
	}
	model := c.Model
	if model != "" && r.Model != model {
		return errors.New("native terminal model differs from frozen launch")
	}
	if r.ReasoningEffort == "" || c.ReasoningEffort != "" && r.ReasoningEffort != c.ReasoningEffort {
		return errors.New("native terminal thinking level differs from frozen launch")
	}
	return nil
}

func (l *frozenRecord) readTerminalOwner() (TerminalOwner, error) {
	c := l.profile.Terminal
	if c == nil {
		return TerminalOwner{}, errors.New("native terminal profile missing")
	}
	var owner TerminalOwner
	if err := readTerminalFile(c.StateDirectory, "owner", &owner); err != nil {
		return owner, err
	}
	if !l.terminalOwnerBinding(owner) || owner.Job.Handle == 0 || owner.Job.Worker.Kind != "os" || owner.Job.Worker.PID <= 0 {
		return owner, errors.New("original native terminal owner differs from frozen launch")
	}
	return owner, nil
}

func (l *frozenRecord) terminalOwnerBinding(owner TerminalOwner) bool {
	c := l.profile.Terminal
	if c == nil || len(c.BundleSHA256) != 64 {
		return false
	}
	bundle := c.BundleSHA256
	_, creationErr := strconv.ParseUint(owner.Job.Owner.Created, 16, 64)
	return len(owner.Job.Owner.Created) == 16 && creationErr == nil && owner.RuntimeSHA256 != "" && owner.RuntimeSHA256 == l.profile.RuntimeSHA256 && owner.ExtensionSHA256 == bundle && owner.RunID == c.ExecutionID && owner.SessionID == c.SessionID && owner.ConfigurationDigest == c.ConfigurationDigest && owner.AccessMode == c.AccessMode && owner.Job.Owner.Kind == "os" && owner.Job.Owner.PID > 0 && owner.Job.Owner.Created != "" && owner.Job.Owner.Executable == strings.ToLower(filepath.Clean(l.profile.RuntimeExecutable))
}

func (l *frozenRecord) terminalExitVerified(owner TerminalOwner) bool {
	var fact TerminalExit
	return readTerminalFile(l.profile.Terminal.StateDirectory, "exit", &fact) == nil && fact.Version == 2 && fact.WritersAbsent && !fact.ClosedAt.IsZero() && reflect.DeepEqual(fact.Owner, owner) && process.Probe(&owner.Job.Owner).State == "absent"
}

func (l *frozenRecord) validateTerminalTurn(state State, turn *Turn) error {
	f := turn.Extension
	if f == nil {
		if turn.Started || turn.Settled {
			return errors.New("native extension start/settlement evidence missing")
		}
		return nil
	}
	if err := l.validateTerminalReceipt(*f); err != nil {
		return err
	}
	r := state.TerminalReadiness
	if r == nil || !sameTerminalInstance(*f, *r) || f.CommandID != turn.Command || f.WireSHA256 != digest([]byte(wirePrompt(turn))) || !turn.Accepted {
		return errors.New("saved native extension turn changed")
	}
	if turn.Started && f.Stage != "started" && f.Stage != "settled" || turn.Settled && (f.Stage != "settled" || f.Outcome != "completed" || f.StopReason != "stop" || f.NativeInputID == "" || f.NativeReplyID == "" || f.Reply != "" && f.Reply != turn.Reply || f.ReplySHA256 != turn.ReplyDigest) {
		return errors.New("saved native extension outcome changed")
	}
	return nil
}

func (l *frozenRecord) verifyStopped(ctx context.Context, s b.LifecycleSnapshot) error {
	if l.localCodexScope() {
		return l.verifyLocalCodexStopped(ctx, s)
	}
	state, err := l.state(s)
	if err != nil {
		return err
	}
	if state.Process.Kind != "orca_terminal" || state.Process != s.Ref.External || state.Process.ExecutionHost != l.profile.Target().ExecutionHost || state.Process.Executable != l.profile.Executable {
		return errors.New("original Orca resource binding is incomplete")
	}
	if err = VerifyTerminalStopped(ctx, l.client, state.Process); err != nil {
		return err
	}
	if l.profile.nativeTerminalContract() {

		return ctx.Err()
	}
	if l.profile.Terminal == nil {
		return errors.New("local Codex helper scope is unavailable; PTY exit is insufficient")
	}
	if state.RemoteReadiness != nil {
		return errors.New("native terminal helper has incompatible Codex resource evidence")
	}
	if state.Control != nil {
		return errors.New("native terminal helper has unexpected control resource")
	}
	owner, e := l.readTerminalOwner()
	if e != nil || state.TerminalOwner == nil || !reflect.DeepEqual(state.TerminalOwner, &owner) || !l.terminalExitVerified(owner) {
		return errors.Join(e, errors.New("original native terminal exit cleanup is unverified"))
	}
	return ctx.Err()
}

func (l *frozenRecord) codexReadiness(state State, workspace string) (codex.RemoteReadiness, error) {
	if l.profile.Environment != "" {
		return codex.RemoteReadiness{}, errors.New("helper writer-tree verification is local only")
	}
	r := state.RemoteReadiness
	if r == nil || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.RuntimeSHA256 == "" || r.RuntimeSHA256 != l.profile.RuntimeSHA256 || r.ConfigurationDigest == "" || r.ConfigurationDigest != l.profile.RuntimeConfigurationDigest || !codex.ValidNativeID(r.SessionID) || r.Workspace == "" || !strings.EqualFold(strings.ReplaceAll(r.Workspace, "\\", "/"), strings.ReplaceAll(workspace, "\\", "/")) || r.Repository == "" || r.Repository != l.setup.RepositoryURL || r.Process.Kind != "os" || r.Process.PID <= 0 || r.Process.Created == "" || r.Process.Executable == "" || r.Owner.Kind != "os" || r.Owner.PID <= 0 || r.Owner.Created == "" || r.Owner.Executable == "" || r.CheckedAt.IsZero() || state.TerminalOwner != nil || state.TerminalReadiness != nil {
		return codex.RemoteReadiness{}, errors.New("original Codex native exit binding is incomplete; PTY exit is insufficient")
	}
	return *r, nil
}
