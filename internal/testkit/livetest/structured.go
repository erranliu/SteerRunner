package livetest

// This file owns test-side interpretation of the current frozen Orca protocol.
// Production never imports livetest. No execution lifecycle is constructed here.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"slices"
	"sort"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	adapter "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// StructuredBinding is an ephemeral test view, not a new persisted format.
type StructuredBinding struct {
	RuntimeID         string          `json:"runtime_id"`
	Worktree          native.Worktree `json:"worktree"`
	SessionID         string          `json:"session_id"`
	CommandID         string          `json:"command_id"`
	InputDigest       string          `json:"input_digest"`
	BodyFingerprint   string          `json:"body_fingerprint"`
	CreateID          string          `json:"create_id"`
	MessageID         string          `json:"message_id"`
	TurnID            string          `json:"turn_id"`
	TurnItemID        string          `json:"turn_item_id"`
	ProviderItemID    string          `json:"provider_item_id"`
	Epoch             string          `json:"epoch"`
	Attempted         bool            `json:"attempted"`
	CloseAttempted    bool            `json:"close_attempted"`
	CloseReceipt      json.RawMessage `json:"close_receipt"`
	SessionNotCreated bool            `json:"session_not_created"`
}

type StructuredRun struct {
	Agent   string                      `json:"agent"`
	Binding StructuredBinding           `json:"binding"`
	Profile adapter.AgentSessionProfile `json:"profile"`
	Inputs  []StructuredBinding         `json:"inputs"`
}

func StructuredSessionID(run, attempt string) string {
	sum := sha256.Sum256([]byte(run + "\x00" + attempt))
	return "oas_" + hex.EncodeToString(sum[:])[:32]
}

func ReadStructuredRun(m Manifest, view contracts.RunView, record *runstate.Record) (StructuredRun, error) {
	var result StructuredRun
	if record == nil {
		return result, errors.New("original Run unavailable")
	}
	s := record.Snapshot
	result.Agent = s.Ref.Agent.Name
	runtime, err := m.OrcaRuntimeID()
	var settings contracts.OrcaSettings
	if err != nil || contracts.DecodeStrict(m.Execution.Settings, &settings) != nil ||
		record.ID != view.RunID || s.Ref.RunID != view.RunID || s.Session.ExecutionID != record.ID || s.Session.WorkflowRunID != record.ID || s.Session.AttemptID != view.Execution.AttemptID || s.Ref.AttemptID != view.Execution.AttemptID ||
		s.ResourceGeneration != view.Execution.Generation || s.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 ||
		view.Execution.Protocol != string(b.ProtocolOrcaAgentSessionV1) ||
		s.Ref.Agent.Name != view.Execution.Agent || settings.AgentName() != view.Execution.Agent ||
		s.Ref.External.RuntimeID != runtime || s.Ref.External.Kind != "orca_agent_session" ||
		s.Ref.External.Terminal != "" || s.Ref.External.Incarnation != "" || s.Ref.External.PID != 0 ||
		s.Workspace.Path != view.Execution.Workspace || s.Workspace.Branch != view.Execution.Branch {
		return result, errors.New("original structured Run/public identity changed")
	}
	setup := b.LifecycleSetup{Snapshot: s, ProfileData: record.Profile, ProfileID: record.ID, AcceptedConfigVersion: record.InputDigest, ProcessHost: process.New(30 * time.Second)}
	scope, err := adapter.StopEvidenceScope(setup)
	if err != nil || scope != adapter.AgentSessionStopScope || view.StopEvidenceScope != scope || record.StopEvidenceScope != scope {
		return result, errors.New("original structured profile/scope unavailable")
	}
	found := false
	for _, e := range record.Profile {
		if e.ImplementationKey != adapter.AgentSessionProfileKey {
			continue
		}
		if found || json.Unmarshal(e.Payload, &result.Profile) != nil {
			return result, errors.New("ambiguous structured profile")
		}
		found = true
	}
	if !found || result.Profile.Transport.Executable != settings.Transport.Executable || result.Profile.Transport.Environment != m.Execution.Target.Environment || !slices.Equal(result.Profile.Transport.Args, settings.Transport.Args) || result.Profile.AccessMode != settings.AccessMode {
		return result, errors.New("original structured transport changed")
	}
	var bindings []StructuredBinding
	for _, e := range s.Implementation {
		if e.ImplementationKey != adapter.AgentSessionStateKey {
			continue
		}
		var value StructuredBinding
		if e.CodecVersion != adapter.AgentSessionVersion || e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest) != nil || json.Unmarshal(e.Payload, &value) != nil {
			return result, errors.New("invalid original structured envelope")
		}
		bindings = append(bindings, value)
	}
	if len(bindings) != 1 {
		return result, errors.New("original structured state missing or ambiguous")
	}
	result.Binding = bindings[0]
	current := result.Binding
	if current.SessionID != StructuredSessionID(view.RunID, view.Execution.AttemptID) || current.RuntimeID != runtime || current.Worktree.ID == "" || current.Worktree.ID != s.Ref.External.WorktreeID || current.Worktree.InstanceID == "" || current.Worktree.Path != view.Execution.Workspace || current.Worktree.Branch != view.Execution.Branch || current.TurnID != view.Execution.TurnID {
		return result, errors.New("original structured resource changed")
	}
	// A never-dispatched session has an original deterministic ID in its
	// envelope but no public native session. Only the original stop proof can
	// qualify that absence; an unknown create/close must still be rejected.
	if view.Execution.SessionID != current.SessionID && (view.Execution.SessionID != "" || view.StopStatus != "stopped" || !current.SessionNotCreated) {
		return result, errors.New("original structured public session changed")
	}
	if view.StopStatus == "stopped" {
		if view.NewInputsAllowed {
			return result, errors.New("stopped Run still permits new inputs")
		}
		if err := adapter.VerifyStopped(context.Background(), setup); err != nil {
			return result, err
		}
	}
	seen := map[string]bool{}
	seenCommands, seenTurns := map[string]bool{}, map[string]bool{}
	for _, op := range s.Operations {
		if op.Status != b.OperationConfirmed || (op.Action != string(b.LifecycleStartTurn) && op.Action != string(b.LifecycleSubmitInput)) {
			continue
		}
		requestHash := sha256.Sum256(op.Request)
		candidate, err := b.CandidateForOperation(op)
		var receipt b.LifecycleReceipt
		if err != nil || op.PayloadDigest != hex.EncodeToString(requestHash[:]) || candidate.Input.Turn == nil || json.Unmarshal(op.Receipt, &receipt) != nil || receipt.ValidateFor(b.LifecycleRequest{Snapshot: s, Operation: op, Input: candidate.Input}) != nil || receipt.Disposition != b.OperationConfirmed || !receipt.State.InputAccepted || receipt.State.SessionID != current.SessionID {
			return result, errors.New("original input receipt invalid")
		}
		found := false
		for _, e := range receipt.Implementation {
			if e.ImplementationKey != adapter.AgentSessionStateKey {
				continue
			}
			var input StructuredBinding
			if found || e.CodecVersion != adapter.AgentSessionVersion || e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest) != nil || json.Unmarshal(e.Payload, &input) != nil {
				return result, errors.New("original input envelope invalid")
			}
			found = true
			hash := sha256.Sum256([]byte(candidate.Input.Turn.Prompt))
			if input.SessionID != current.SessionID || input.RuntimeID != current.RuntimeID || input.Worktree != current.Worktree || input.Epoch != current.Epoch || input.CommandID != candidate.Input.Turn.CommandID || input.InputDigest != hex.EncodeToString(hash[:]) || input.MessageID == "" || input.TurnID == "" || input.TurnItemID == "" || input.TurnID != receipt.State.TurnID || seen[input.MessageID] || seenCommands[input.CommandID] || seenTurns[input.TurnID] {
				return result, errors.New("original input binding changed")
			}
			seen[input.MessageID] = true
			seenCommands[input.CommandID], seenTurns[input.TurnID] = true, true
			result.Inputs = append(result.Inputs, input)
		}
		if !found {
			return result, errors.New("original input proof missing")
		}
	}
	return result, nil
}

type StructuredHistoryClient interface {
	History(context.Context, string, *native.AgentCursor, int) (native.AgentPage, error)
	Item(context.Context, string, string) (native.AgentItem, error)
}

type StructuredHistory struct {
	Items       []native.AgentItem `json:"items"`
	Tail        native.AgentPage   `json:"tail"`
	Inputs      int                `json:"inputs"`
	Fingerprint string             `json:"input_fingerprint"`
	CurrentTurn native.AgentTurn   `json:"current_turn"`
}

// ReadStructuredHistory requires the full original epoch, not a terminal preview.
// A semantic input count is not a count of remote mutation RPC dispatches.
func ReadStructuredHistory(ctx context.Context, client StructuredHistoryClient, run StructuredRun) (StructuredHistory, error) {
	var result StructuredHistory
	if run.Binding.MessageID == "" || run.Binding.TurnID == "" || run.Binding.TurnItemID == "" || run.Binding.ProviderItemID == "" || len(run.Binding.BodyFingerprint) != 64 || run.Binding.Epoch == "" {
		return result, errors.New("original structured input proof incomplete")
	}
	var cursor *native.AgentCursor
	items := map[string]native.AgentItem{}
	bytesRead := 0
	for n := 0; n < 64; n++ {
		page, err := client.History(ctx, run.Binding.SessionID, cursor, 200)
		if err != nil {
			return result, err
		}
		if n == 0 {
			result.Tail = page
		}
		if page.SessionID != run.Binding.SessionID || page.Epoch == "" || page.Epoch != run.Binding.Epoch || page.Epoch != result.Tail.Epoch || page.HasNewer || len(page.RemovedItemIDs) > 0 {
			return result, errors.New("original history incomplete or changed")
		}
		for _, item := range page.Items {
			if item.ItemID == "" {
				return result, errors.New("history item identity missing")
			}
			if item.Shortened {
				full, err := client.Item(ctx, run.Binding.SessionID, item.ItemID)
				if err != nil {
					return result, err
				}
				if full.ItemID != item.ItemID || full.Shortened || full.Revision != item.Revision || full.Sequence != item.Sequence || full.SequenceIndex != item.SequenceIndex || full.TurnScope != item.TurnScope {
					return result, errors.New("history detail changed")
				}
				item = full
			}
			bytesRead += len(item.Body)
			if bytesRead > 8<<20 {
				return result, errors.New("history exceeds limit")
			}
			if old, ok := items[item.ItemID]; ok {
				a, _ := json.Marshal(old)
				bb, _ := json.Marshal(item)
				if string(a) != string(bb) {
					return result, errors.New("history item changed")
				}
			}
			items[item.ItemID] = item
		}
		if !page.HasOlder {
			break
		}
		next := page.Window.NextCursor
		if n == 63 || next == nil || next.Epoch != page.Epoch || cursor != nil && next.Sequence >= cursor.Sequence {
			return result, errors.New("history cursor incomplete")
		}
		cursor = next
	}
	for _, item := range items {
		result.Items = append(result.Items, item)
	}
	sort.Slice(result.Items, func(i, j int) bool {
		a, bb := result.Items[i], result.Items[j]
		if a.Sequence == bb.Sequence {
			return a.SequenceIndex < bb.SequenceIndex
		}
		return a.Sequence < bb.Sequence
	})
	expected := map[string]StructuredBinding{}
	for _, binding := range append(append([]StructuredBinding{}, run.Inputs...), run.Binding) {
		id := "orca:" + binding.MessageID
		if old, ok := expected[id]; ok && (old.CommandID != binding.CommandID || old.InputDigest != binding.InputDigest || old.TurnID != binding.TurnID || old.TurnItemID != binding.TurnItemID || old.BodyFingerprint != binding.BodyFingerprint) {
			return result, errors.New("current input differs from original receipt")
		}
		expected[id] = binding
	}
	users, turns := map[string]bool{}, map[string]bool{}
	var fingerprint []string
	for i, item := range result.Items {
		if i > 0 && item.Sequence == result.Items[i-1].Sequence && item.SequenceIndex == result.Items[i-1].SequenceIndex {
			return result, errors.New("ambiguous item ordering")
		}
		var body struct {
			Kind, Role string
			Blocks     []native.AgentTextBlock
		}
		if json.Unmarshal(item.Body, &body) != nil {
			return result, errors.New("invalid history body")
		}
		if body.Kind == "message" && body.Role == "user" {
			binding, ok := expected[item.ItemID]
			if !ok || users[item.ItemID] || len(body.Blocks) != 1 || body.Blocks[0].Type != "text" {
				return result, errors.New("unexpected original user input")
			}
			sum := sha256.Sum256([]byte(body.Blocks[0].Text))
			if hex.EncodeToString(sum[:]) != binding.InputDigest {
				return result, errors.New("original input text changed")
			}
			users[item.ItemID] = true
			fingerprint = append(fingerprint, item.ItemID+":"+binding.InputDigest)
		}
		if body.Kind == "turn" {
			var turn native.AgentTurn
			if json.Unmarshal(item.Body, &turn) != nil {
				return result, errors.New("invalid original turn")
			}
			binding, ok := expected[turn.UserItemID]
			if !ok || turns[turn.UserItemID] || turn.TurnID != binding.TurnID || item.ItemID != binding.TurnItemID || turn.StartedAt <= 0 {
				return result, errors.New("original turn identity changed")
			}
			turns[turn.UserItemID] = true
			if binding.MessageID == run.Binding.MessageID {
				result.CurrentTurn = turn
			}
		}
	}
	for id := range expected {
		if !users[id] || !turns[id] {
			return result, errors.New("original user or turn proof missing")
		}
	}
	currentMatches := 0
	for _, sub := range result.Tail.Submissions {
		if sub.ClientMessageID == run.Binding.MessageID {
			currentMatches++
			if sub.DispatchState != "accepted" || sub.ProviderItemID != run.Binding.ProviderItemID || sub.PayloadFingerprint != run.Binding.BodyFingerprint {
				return result, errors.New("original submission changed")
			}
		}
	}
	if currentMatches != 1 {
		return result, errors.New("original submission proof missing or ambiguous")
	}
	sort.Strings(fingerprint)
	raw, _ := json.Marshal(fingerprint)
	sum := sha256.Sum256(raw)
	result.Inputs = len(users)
	result.Fingerprint = fmt.Sprintf("%d:%x", result.Inputs, sum)
	return result, nil
}

// FrozenStructuredFingerprint is usable after close even if the native host no
// longer serves that history. It proves durable input identity, not wire count.
func FrozenStructuredFingerprint(run StructuredRun) string {
	values := map[string]string{}
	for _, binding := range append(append([]StructuredBinding{}, run.Inputs...), run.Binding) {
		values["orca:"+binding.MessageID] = binding.InputDigest
	}
	var parts []string
	for id, hash := range values {
		parts = append(parts, id+":"+hash)
	}
	sort.Strings(parts)
	raw, _ := json.Marshal(parts)
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%d:%x", len(parts), sum)
}
