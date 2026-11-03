package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

const AgentSessionProfileKey = "orca.agent-session-profile"
const AgentSessionStateKey = "orca.agent-session-state"
const AgentSessionVersion = 1
const AgentSessionStopScope = "orca_agent_session"
const AgentSessionStopDiagnostic = "Orca 原会话 close 不提供完整退出证据；资源结算未核实 (resources_settled=false)."
const agentResultSource = "orca.agent-session.turn-result.v1"

// Only this frozen protocol uses Orca's native Codex owner. Transport remains
// the existing local IPC/CLI routing configuration, not another backend.
type AgentSessionProfile struct {
	Transport       Profile `json:"transport"`
	Model           string  `json:"model,omitempty"`
	ReasoningEffort string  `json:"reasoning_effort,omitempty"`
	AccessMode      string  `json:"access_mode"`
}

type agentSessionPort interface {
	SupportsCreate(context.Context, string) error
	Create(context.Context, native.AgentCreate) (json.RawMessage, error)
	History(context.Context, string, *native.AgentCursor, int) (native.AgentPage, error)
	Item(context.Context, string, string) (native.AgentItem, error)
	Close(context.Context, string) (json.RawMessage, error)
}

// No input or result bodies are copied into opaque resource state.
type agentSessionState struct {
	RuntimeID       string              `json:"runtime_id"`
	Worktree        native.Worktree     `json:"worktree"`
	Shells          []b.ProcessIdentity `json:"shells,omitempty"`
	SessionID       string              `json:"session_id"`
	CommandID       string              `json:"command_id,omitempty"`
	InputDigest     string              `json:"input_digest,omitempty"`
	BodyFingerprint string              `json:"body_fingerprint,omitempty"`
	CreateID        string              `json:"create_id,omitempty"`
	MessageID       string              `json:"message_id,omitempty"`
	TurnID          string              `json:"turn_id,omitempty"`
	TurnItemID      string              `json:"turn_item_id,omitempty"`
	ProviderItemID  string              `json:"provider_item_id,omitempty"`
	Epoch           string              `json:"epoch,omitempty"`
	Attempted       bool                `json:"attempted,omitempty"`
	CloseAttempted  bool                `json:"close_attempted,omitempty"`
}

func (l *Lifecycle) structured() bool {
	return l.setup.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1
}
func decodeAgentSessionProfile(setup b.LifecycleSetup) (AgentSessionProfile, error) {
	var p AgentSessionProfile
	found := false
	for _, e := range setup.ProfileData {
		if e.ImplementationKey != AgentSessionProfileKey {
			continue
		}
		if found || e.CodecVersion != AgentSessionVersion {
			return p, errors.New("invalid structured Orca profile codec")
		}
		found = true
		if err := e.ValidateProfileIdentity(setup.Snapshot.Session.ProjectID, setup.ProfileID, setup.AcceptedConfigVersion); err != nil {
			return p, err
		}
		if err := b.DecodeStrictJSON(e.Payload, &p, "trailing structured Orca profile"); err != nil {
			return p, err
		}
	}
	wire, _ := json.Marshal(p.Transport)
	_, err := DecodeProfile(wire)
	if !found || err != nil || setup.Snapshot.Ref.Backend != b.BackendOrca || setup.Snapshot.Ref.Agent.Name != "codex" || p.AccessMode != "full_access" || p.Transport.Environment != "" || p.Transport.NativeTerminal || p.Transport.Terminal != nil || p.Transport.RuntimeExecutable != "" || p.Transport.RuntimeSHA256 != "" || p.Transport.RuntimeConfigurationDigest != "" || setup.AgentLaunchCommand != "" || setup.Snapshot.Ref.Target.ID != p.Transport.Target().ID {
		return p, errors.New("structured Orca requires its frozen local native Codex profile")
	}
	return p, nil
}
func (l *Lifecycle) validateStructured(s b.LifecycleSnapshot) error {
	if err := l.validate(s); err != nil {
		return err
	}
	if s.Ref.Agent.Name != "codex" || s.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
		return errors.New("unsupported structured Orca identity")
	}
	return nil
}
func (l *Lifecycle) agentClient(runtime string) agentSessionPort {
	if l.agentSessions != nil {
		return l.agentSessions(runtime)
	}
	return native.NewLocalAgentSessionClient(runtime)
}
func agentSessionID(s b.LifecycleSnapshot) string {
	return "oas_" + digest([]byte(s.Ref.RunID + "\x00" + s.Ref.AttemptID))[:32]
}
func (l *Lifecycle) agentState(s b.LifecycleSnapshot) (agentSessionState, error) {
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
	if found && (state.RuntimeID == "" || state.Worktree.ID == "" || state.Worktree.InstanceID == "" || state.SessionID != agentSessionID(s) || state.Worktree.ID != s.Ref.External.WorktreeID || state.RuntimeID != s.Ref.External.RuntimeID || s.Ref.External.Kind != "orca_agent_session" || s.Ref.External.Executable != l.profile.Executable || s.Ref.External.ExecutionHost != "local" || s.Ref.External.Terminal != "" || s.Ref.External.PID != 0 || s.Ref.External.Incarnation != "" || state.Worktree.Path != s.Workspace.Path) {
		return state, errors.New("original structured Orca resource binding changed")
	}
	if state.CommandID != "" && (!b.StableExecutionID(state.CommandID) || len(state.InputDigest) != 64 || len(state.BodyFingerprint) != 64 || state.CreateID == "" || state.MessageID == "") {
		return state, errors.New("incomplete original structured input binding")
	}
	if s.PendingTurn != nil && state.CommandID != "" && (state.CommandID != s.PendingTurn.CommandID || state.InputDigest != digest([]byte(s.PendingTurn.Prompt))) {
		return state, errors.New("original structured input changed")
	}
	return state, nil
}
func (l *Lifecycle) agentEnvelope(s b.LifecycleSnapshot, state agentSessionState) ([]b.ImplementationDataEnvelope, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: AgentSessionVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: AgentSessionStateKey, ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}, raw)
	if err != nil {
		return nil, err
	}
	return []b.ImplementationDataEnvelope{e}, nil
}
func (l *Lifecycle) structuredPolicy() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: b.MechanismNotApplicable, Session: b.MechanismApplicable}, ReportToolPolicy: "configured_native_launch_parameters; execution_host_existing_authentication; business_read_only_is_prompt_policy"}
}
func (l *Lifecycle) structuredPreparation(ctx context.Context, s b.LifecycleSnapshot) (b.PreparationFacts, error) {
	if err := l.validateStructured(s); err != nil {
		return b.PreparationFacts{}, err
	}
	if s.Workspace.Path == "" {
		return b.PreparationFacts{Workspace: b.GateRequired, Resource: b.GateNotApplicable, Session: b.GateUnverified}, nil
	}
	state, err := l.agentState(s)
	if err != nil {
		return b.PreparationFacts{}, err
	}
	if err = l.originalAgentWorkspace(ctx, state); err != nil {
		return b.PreparationFacts{}, err
	}
	gate := b.GateUnverified
	if s.PendingTurn != nil {
		gate = b.GateRequired
		if state.CommandID == s.PendingTurn.CommandID {
			gate = b.GateReady
		}
	}
	return b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateNotApplicable, Session: gate}, nil
}
func (l *Lifecycle) originalAgentWorkspace(ctx context.Context, state agentSessionState) error {
	result, shown, err := l.client.ShowWorktree(ctx, "id:"+state.Worktree.ID)
	if err != nil {
		return err
	}
	if !shown.Decoded() || result.RuntimeID != state.RuntimeID || shown.Worktree.ID != state.Worktree.ID || shown.Worktree.InstanceID != state.Worktree.InstanceID || shown.Worktree.Path != state.Worktree.Path || shown.Worktree.BaseRef != state.Worktree.BaseRef || shown.Worktree.Branch != state.Worktree.Branch {
		return errors.New("original structured Orca workspace changed")
	}
	return nil
}
func (l *Lifecycle) finishStructuredWorkspace(ctx context.Context, r b.LifecycleRequest, created workspaceCreation) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	s := r.Snapshot
	state := agentSessionState{RuntimeID: created.RuntimeID, Worktree: created.Worktree, SessionID: agentSessionID(s)}
	if err := l.originalAgentWorkspace(ctx, state); err != nil {
		return receipt, err
	}
	inventory, err := l.client.CompleteWorktreeInventory(ctx, native.WorktreeInventoryScope{RuntimeID: state.RuntimeID, WorktreeID: state.Worktree.ID})
	if err != nil {
		return receipt, err
	}
	for _, t := range inventory {
		if t.AgentIdentity != "" || t.Orphaned || t.WorktreePath != state.Worktree.Path {
			return receipt, errors.New("structured workspace contains an unexpected agent or shell")
		}
		state.Shells = append(state.Shells, b.ProcessIdentity{Kind: "orca_terminal", Executable: l.profile.Executable, RuntimeID: state.RuntimeID, WorktreeID: state.Worktree.ID, Terminal: t.Handle, Incarnation: t.IncarnationID, ExecutionHost: "local"})
	}
	sort.Slice(state.Shells, func(i, j int) bool { return state.Shells[i].Terminal < state.Shells[j].Terminal })
	p := b.ProcessIdentity{Kind: "orca_agent_session", Executable: l.profile.Executable, RuntimeID: state.RuntimeID, WorktreeID: state.Worktree.ID, ExecutionHost: "local"}
	workspace := b.WorkspaceInspection{InspectionScope: "revision", Repository: s.Session.ProjectID, Worktree: state.Worktree.Path, Branch: strings.TrimPrefix(state.Worktree.Branch, "refs/heads/"), Base: r.Input.Workspace.BaseCommit, Head: state.Worktree.Head, CheckedAt: time.Now().UTC()}
	target := s.Ref.Target
	target.WorktreeID = state.Worktree.ID
	target.Branch = workspace.Branch
	target.BaseRevision = workspace.Base
	receipt.Workspace, receipt.Target, receipt.Process = &workspace, &target, &p
	receipt.State = b.LifecycleStateFacts{WorktreeID: state.Worktree.ID, RuntimeID: state.RuntimeID}
	receipt.Implementation, err = l.agentEnvelope(s, state)
	if err == nil {
		receipt.Disposition = b.OperationConfirmed
	}
	return receipt, err
}
func agentBody(text string) native.AgentMessage {
	return native.AgentMessage{Kind: "message", Role: "user", Blocks: []native.AgentTextBlock{{Type: "text", Text: text}}}
}
func (l *Lifecycle) prepareStructured(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}
	state, err := l.agentState(r.Snapshot)
	if err != nil {
		return receipt, err
	}
	input := r.Input.Turn
	if input == nil || input.CommandID != command(r.Snapshot) || state.CommandID != "" && state.CommandID != input.CommandID {
		return receipt, errors.New("structured execute permits one original input")
	}
	parts, err := input.ModelInputParts()
	if err != nil {
		return receipt, err
	}
	if _, _, err = m.FreezeInput(parts...); err != nil {
		return receipt, err
	}
	if err = agentBody(input.Prompt).ValidateInput(); err != nil {
		return receipt, err
	}
	if state.CommandID == "" {
		if err = l.originalAgentWorkspace(ctx, state); err != nil {
			return receipt, err
		}
		if err = l.agentClient(state.RuntimeID).SupportsCreate(ctx, "id:"+state.Worktree.ID); err != nil {
			return receipt, err
		}
		state.CommandID, state.InputDigest = input.CommandID, digest([]byte(input.Prompt))
		id := digest([]byte(r.Snapshot.Ref.RunID + "\x00" + input.CommandID))[:32]
		state.CreateID = fmt.Sprintf("%d-%s", r.Operation.StartedAt.UnixMilli(), id)
		state.MessageID = fmt.Sprintf("%d-%s", r.Operation.StartedAt.UnixMilli(), digest([]byte("message\x00" + id))[:32])
		state.BodyFingerprint, err = native.AgentFingerprint("agentSession.send", state.SessionID, map[string]any{"body": agentBody(input.Prompt)})
		if err != nil {
			return receipt, err
		}
	}
	receipt.Implementation, err = l.agentEnvelope(r.Snapshot, state)
	if err == nil {
		receipt.Disposition = b.OperationConfirmed
	}
	return receipt, err
}
func (l *Lifecycle) executeStructured(ctx context.Context, r b.LifecycleRequest, reconcile bool) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	if err := r.Validate(); err != nil {
		return receipt, nil, err
	}
	if err := l.validateStructured(r.Snapshot); err != nil {
		return receipt, nil, err
	}
	var err error
	switch b.LifecycleAction(r.Operation.Action) {
	case b.LifecyclePrepareWorkspace:
		receipt, err = l.prepareWorkspace(ctx, r, reconcile)
	case b.LifecyclePrepareSession:
		receipt, err = l.prepareStructured(ctx, r)
	case b.LifecycleStopExecution:
		receipt, err = l.structuredStop(ctx, r, reconcile)
	case b.LifecycleStartTurn, b.LifecycleSubmitInput:
		state, e := l.agentState(r.Snapshot)
		if e != nil {
			return receipt, nil, e
		}
		if r.Input.Turn == nil || state.CommandID != r.Input.Turn.CommandID || state.InputDigest != digest([]byte(r.Input.Turn.Prompt)) {
			return receipt, nil, errors.New("structured input is not the prepared original input")
		}
		if !reconcile {
			if state.Attempted {
				return receipt, nil, errors.New("original structured creation already attempted; reconcile only")
			}
			profile, e := decodeAgentSessionProfile(l.setup)
			if e != nil {
				return receipt, nil, e
			}
			selection := &native.AgentSelection{Model: profile.Model, Effort: profile.ReasoningEffort}
			if selection.Model == "" && selection.Effort == "" {
				selection = nil
			}
			create := native.AgentCreate{Worktree: "id:" + state.Worktree.ID, Agent: "codex", Options: map[string]string{"permissionMode": "full-access"}, FirstMessage: native.AgentFirstMessage{ClientMessageID: state.MessageID, Body: agentBody(r.Input.Turn.Prompt), Selection: selection}, Envelope: native.AgentMutationEnvelope{SessionID: state.SessionID, ClientOperationID: state.CreateID}}
			create.Envelope.PayloadFingerprint, e = native.AgentFingerprint("agentSession.create", state.SessionID, map[string]any{"worktree": create.Worktree, "agent": create.Agent, "options": create.Options, "firstMessage": create.FirstMessage})
			if e != nil {
				return receipt, nil, e
			}
			state.Attempted = true
			receipt.Implementation, _ = l.agentEnvelope(r.Snapshot, state)
			receipt.Operation.Receipt, err = l.agentClient(state.RuntimeID).Create(ctx, create)
			if err != nil {
				return receipt, nil, err
			}
		}
		state.Attempted = true // the original durable in-flight operation may have written before a crash
		snapshot := r.Snapshot.Clone()
		snapshot.Implementation, _ = l.agentEnvelope(snapshot, state)
		observed, e := l.readStructured(ctx, snapshot)
		if e != nil {
			return receipt, nil, e
		}
		observed.Operation = receipt.Operation
		if observed.State.InputAccepted {
			observed.Disposition = b.OperationConfirmed
			snapshot.Implementation = observed.Implementation
			if observed.State.TurnStarted {
				snapshot.Session.CurrentTurnID = observed.State.TurnID
			}
			return observed, l.observeStructured(snapshot), nil
		}
		observed.Disposition = b.OperationUnknown
		return observed, nil, nil
	default:
		err = errors.New("unsupported structured Orca action")
	}
	return receipt, nil, err
}

// Read all public pages from the same epoch. Missing pages or shortened items
// cannot silently become a successful result. Bounds never trigger resubmission.
func (l *Lifecycle) agentHistory(ctx context.Context, state agentSessionState) ([]native.AgentItem, native.AgentPage, error) {
	client := l.agentClient(state.RuntimeID)
	var before *native.AgentCursor
	var tail native.AgentPage
	items := map[string]native.AgentItem{}
	bytesRead := 0
	for pageNo := 0; pageNo < 64; pageNo++ {
		page, err := client.History(ctx, state.SessionID, before, 200)
		if err != nil {
			return nil, tail, err
		}
		if pageNo == 0 {
			tail = page
			if state.Epoch != "" && state.Epoch != page.Epoch {
				return nil, tail, errors.New("original agent history epoch changed")
			}
		}
		if page.Epoch != tail.Epoch || page.SessionID != state.SessionID || page.HasNewer || len(page.RemovedItemIDs) > 0 {
			return nil, tail, errors.New("original agent history is incomplete or changed")
		}
		for _, item := range page.Items {
			if item.ItemID == "" {
				return nil, tail, errors.New("agent item has no identity")
			}
			if item.Shortened {
				full, e := client.Item(ctx, state.SessionID, item.ItemID)
				if e != nil {
					return nil, tail, e
				}
				if full.Revision != item.Revision || full.Sequence != item.Sequence || full.SequenceIndex != item.SequenceIndex || full.TurnScope != item.TurnScope {
					return nil, tail, errors.New("agent item changed during detail lookup")
				}
				item = full
			}
			if old, ok := items[item.ItemID]; ok && (old.Revision != item.Revision || old.Sequence != item.Sequence || old.SequenceIndex != item.SequenceIndex || old.TurnScope != item.TurnScope || string(old.Body) != string(item.Body)) {
				return nil, tail, errors.New("agent item changed during pagination")
			}
			bytesRead += len(item.Body)
			if bytesRead > 8<<20 {
				return nil, tail, errors.New("complete agent result exceeds observation limit")
			}
			items[item.ItemID] = item
		}
		if !page.HasOlder {
			ordered := make([]native.AgentItem, 0, len(items))
			for _, item := range items {
				ordered = append(ordered, item)
			}
			sort.Slice(ordered, func(i, j int) bool {
				if ordered[i].Sequence == ordered[j].Sequence {
					return ordered[i].SequenceIndex < ordered[j].SequenceIndex
				}
				return ordered[i].Sequence < ordered[j].Sequence
			})
			for i := 1; i < len(ordered); i++ {
				if ordered[i-1].Sequence == ordered[i].Sequence && ordered[i-1].SequenceIndex == ordered[i].SequenceIndex {
					return nil, tail, errors.New("agent history item order is ambiguous")
				}
			}
			return ordered, tail, nil
		}
		next := page.Window.NextCursor
		if next == nil || next.Epoch != tail.Epoch || before != nil && next.Sequence >= before.Sequence {
			return nil, tail, errors.New("agent history cursor did not advance")
		}
		before = next
	}
	return nil, tail, errors.New("complete agent history exceeds page limit")
}
func (l *Lifecycle) readStructured(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Disposition: b.OperationUnknown}
	state, err := l.agentState(s)
	if err != nil {
		return receipt, err
	}
	if state.CommandID == "" {
		return receipt, errors.New("original structured input is unavailable")
	}
	// An incomplete later read cannot erase the already verified native binding
	// needed to persist its diagnostic; it does not reconfirm input or liveness.
	if state.TurnID != "" {
		receipt.State.SessionID, receipt.State.TurnID = state.SessionID, state.TurnID
	}
	items, page, err := l.agentHistory(ctx, state)
	if err != nil {
		return receipt, err
	}
	var submission *native.AgentSubmission
	for i := range page.Submissions {
		sub := &page.Submissions[i]
		if sub.ClientMessageID == state.MessageID {
			if submission != nil || sub.PayloadFingerprint != state.BodyFingerprint {
				return receipt, errors.New("original agent submission binding changed")
			}
			submission = sub
		}
	}
	userID := "orca:" + state.MessageID
	users := 0
	userMatched := false
	var turn native.AgentTurn
	turnItem := ""
	for _, item := range items {
		var body struct {
			Kind, Role string
			Blocks     []native.AgentTextBlock
		}
		if json.Unmarshal(item.Body, &body) != nil {
			return receipt, errors.New("invalid public agent item")
		}
		if body.Kind == "message" && body.Role == "user" {
			users++
			if item.ItemID == userID && len(body.Blocks) == 1 && body.Blocks[0].Type == "text" && digest([]byte(body.Blocks[0].Text)) == state.InputDigest {
				userMatched = true
			}
		}
		if body.Kind == "turn" {
			var value native.AgentTurn
			if json.Unmarshal(item.Body, &value) != nil {
				return receipt, errors.New("invalid native turn")
			}
			if value.UserItemID == userID {
				if turnItem != "" {
					return receipt, errors.New("multiple native turns for original input")
				}
				turn = value
				turnItem = item.ItemID
			}
		}
	}
	if users > 1 || users == 1 && !userMatched {
		return receipt, errors.New("unexpected user input in original execute session")
	}
	if submission == nil || submission.DispatchState != "accepted" || !userMatched || turn.TurnID == "" {
		receipt.Diagnostic = "Original structured input/turn not yet verified; read-only reconciliation required."
		return receipt, nil
	}
	if state.TurnID != "" && state.TurnID != turn.TurnID || state.TurnItemID != "" && state.TurnItemID != turnItem || state.ProviderItemID != "" && state.ProviderItemID != submission.ProviderItemID {
		return receipt, errors.New("original native turn identity changed")
	}
	state.TurnID, state.TurnItemID, state.ProviderItemID, state.Epoch = turn.TurnID, turnItem, submission.ProviderItemID, page.Epoch
	receipt.Implementation, err = l.agentEnvelope(s, state)
	if err != nil {
		return receipt, err
	}
	ended := turn.State == "completed" || turn.State == "interrupted"
	receipt.State = b.LifecycleStateFacts{SessionID: state.SessionID, SessionGeneration: s.ResourceGeneration, SessionAvailable: true, WorkerLive: page.Working, TurnID: turn.TurnID, InputAccepted: true, TurnStarted: turn.StartedAt > 0, TurnCompleted: ended}
	if len(page.ActionablePromptIDs) > 0 {
		receipt.State.QuestionPending = true
		receipt.Diagnostic = "Original Orca session requires native input/approval; structured execute/1 cannot answer it. Stop remains available."
	}
	if !ended {
		return receipt, nil
	}
	outcome := "unknown"
	switch turn.Outcome {
	case "success":
		outcome = "succeeded"
	case "failure":
		outcome = "failed"
	case "cancellation":
		outcome = "cancelled"
	}
	if outcome == "unknown" {
		return receipt, errors.New("original native turn outcome unverified")
	}
	if turn.CompletedAt <= 0 || turn.StartedAt <= 0 || turn.CompletedAt < turn.StartedAt {
		return receipt, errors.New("native turn completion timestamps unverified")
	}
	var messages, ids []string
	for _, item := range items {
		var body struct {
			Kind, Role string
			Blocks     []native.AgentTextBlock
		}
		_ = json.Unmarshal(item.Body, &body)
		if body.Kind != "message" || body.Role != "assistant" {
			continue
		}
		if item.TurnScope.Kind != "turn" || item.TurnScope.TurnItemID != turnItem {
			return receipt, errors.New("assistant result attribution unverified")
		}
		var text strings.Builder
		for _, block := range body.Blocks {
			if block.Type != "text" {
				return receipt, errors.New("non-text assistant result cannot be returned completely")
			}
			text.WriteString(block.Text)
		}
		messages = append(messages, text.String())
		ids = append(ids, item.ItemID)
	}
	result := strings.Join(messages, "\n\n")
	receipt.TurnEvidence = &b.TurnEvidence{CommandID: state.CommandID, SessionID: state.SessionID, TurnID: turn.TurnID, Started: true, Completed: true, Succeeded: outcome == "succeeded", NativeOutcome: outcome, LastMessage: result, FinalVerified: true, FinalSource: agentResultSource, FinalItemID: turnItem + ":" + digest([]byte(strings.Join(ids, "\n")+"\x00"+result)), FinalSessionID: state.SessionID, FinalTurnID: turn.TurnID, FinalNativeSessionID: state.SessionID, FinalNativeTurnID: turn.TurnID, FinalOrigin: "native_turn_assistant_messages"}
	return receipt, nil
}
func (l *Lifecycle) structuredObserveLifecycle(ctx context.Context, s b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	receipt, err := l.readStructured(ctx, s)
	if err != nil || receipt.TurnEvidence != nil {
		return receipt, nil, err
	}
	return receipt, l.observeStructured(s), nil
}
func (l *Lifecycle) observeStructured(s b.LifecycleSnapshot) b.LifecycleObserver {
	ctx, cancel := context.WithCancel(context.Background())
	o := &observer{events: make(chan b.LifecycleEvent, 4), done: make(chan error, 1), finished: make(chan struct{}), cancel: cancel, process: s.Ref.External}
	go func() {
		var failure error
		started := s.Session.CurrentTurnID != ""
		lastDiagnostic := ""
		send := func(event b.LifecycleEvent) bool {
			event.Persisted = make(chan error, 1)
			select {
			case o.events <- event:
			case <-ctx.Done():
				return false
			}
			select {
			case failure = <-event.Persisted:
				return failure == nil
			case <-ctx.Done():
				return false
			}
		}
		defer func() {
			close(o.events)
			if ctx.Err() == nil {
				o.done <- failure
			}
			close(o.done)
			close(o.finished)
		}()
		for {
			callCtx, done := context.WithTimeout(ctx, 30*time.Second)
			receipt, err := l.readStructured(callCtx, s)
			done()
			if err != nil {
				failure = err
				return
			}
			if len(receipt.Implementation) > 0 {
				s.Implementation = receipt.Implementation
			}
			if receipt.State.TurnStarted && !started {
				if !send(b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventStarted, Source: agentResultSource, SourceIdentity: receipt.State.SessionID, SourceSegment: receipt.State.TurnID, SourceEventID: receipt.State.TurnID + ":started", SessionID: receipt.State.SessionID, StartedTurnID: receipt.State.TurnID, Matched: true, InputAccepted: true}, Implementation: receipt.Implementation}) {
					return
				}
				started = true
			}
			if receipt.Diagnostic != lastDiagnostic {
				if !send(b.LifecycleEvent{Observation: &b.ObservationStatus{Diagnostic: receipt.Diagnostic}, Fact: b.EventFact{Kind: b.EventStatus, Source: agentResultSource, SourceIdentity: receipt.State.SessionID, SourceSegment: receipt.State.TurnID, SourceEventID: receipt.State.TurnID + ":observation:" + contracts.NewID(), SessionID: receipt.State.SessionID, StartedTurnID: receipt.State.TurnID}, Implementation: receipt.Implementation}) {
					return
				}
				lastDiagnostic = receipt.Diagnostic
			}
			if evidence := receipt.TurnEvidence; evidence != nil {
				event := b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventCompleted, Source: agentResultSource, SourceIdentity: evidence.SessionID, SourceSegment: evidence.TurnID, SourceEventID: evidence.FinalItemID, SessionID: evidence.SessionID, StartedTurnID: evidence.TurnID, Completed: true, Matched: true, Outcome: evidence.NativeOutcome, LastMessage: evidence.LastMessage}, Implementation: receipt.Implementation, Persisted: make(chan error, 1)}
				select {
				case o.events <- event:
				case <-ctx.Done():
					return
				}
				select {
				case failure = <-event.Persisted:
				case <-ctx.Done():
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	return o
}
func (l *Lifecycle) structuredStop(ctx context.Context, r b.LifecycleRequest, reconcile bool) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown, Diagnostic: AgentSessionStopDiagnostic}
	receipt.Operation.Receipt = nil
	if err := r.Validate(); err != nil {
		return receipt, err
	}
	state, err := l.agentState(r.Snapshot)
	if err != nil {
		return receipt, err
	}
	if l.stopping.Swap(true) {
		reconcile = true
	}
	for _, op := range r.Snapshot.Operations {
		if op.Action == string(b.LifecycleStopExecution) && op.ID != r.Operation.ID {
			reconcile = true
		}
	}
	if reconcile && len(r.Operation.Receipt) > 0 {
		var saved b.LifecycleReceipt
		if json.Unmarshal(r.Operation.Receipt, &saved) != nil || saved.ValidateFor(r) != nil {
			return receipt, errors.New("original structured stop receipt is invalid")
		}
		receipt.Operation.Receipt = append([]byte(nil), saved.Operation.Receipt...)
	}
	if !reconcile && !state.CloseAttempted {
		state.CloseAttempted = true
		receipt.Implementation, _ = l.agentEnvelope(r.Snapshot, state)
		mayExist := state.Attempted
		for _, op := range r.Snapshot.Operations {
			if (op.Action == string(b.LifecycleStartTurn) || op.Action == string(b.LifecycleSubmitInput)) && op.Status != b.OperationPending && op.Status != b.OperationRejected {
				mayExist = true
			}
		}
		if mayExist {
			receipt.Operation.Receipt, err = l.agentClient(state.RuntimeID).Close(ctx, state.SessionID)
		}
		for _, shell := range state.Shells {
			if _, e := InspectBoundTerminal(ctx, l.client, shell); e != nil {
				err = errors.Join(err, e)
				continue
			}
			_, e := CloseTerminalChecked(ctx, l.client, shell.Terminal, shell.RuntimeID)
			err = errors.Join(err, e)
		}
	}
	if len(receipt.Implementation) == 0 {
		receipt.Implementation, _ = l.agentEnvelope(r.Snapshot, state)
	}
	return receipt, errors.Join(err, errors.New(AgentSessionStopDiagnostic))
}
