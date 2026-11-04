package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func agentSendID(op b.Operation) string {
	return fmt.Sprintf("%d-%s", op.StartedAt.UnixMilli(), op.ID)
}

// Use only accepted bindings already committed with the original input
// operations. Current observation can add evidence only for its own message.
func developmentAgentInputs(s b.LifecycleSnapshot, state agentSessionState) ([]agentSessionState, error) {
	var inputs []agentSessionState
	seenCommands := map[string]bool{}
	for _, op := range s.Operations {
		action := b.LifecycleAction(op.Action)
		if op.Status != b.OperationConfirmed || action != b.LifecycleStartTurn && action != b.LifecycleSubmitInput {
			continue
		}
		candidate, err := b.CandidateForOperation(op)
		var receipt b.LifecycleReceipt
		if err != nil || candidate.Input.Turn == nil || op.PayloadDigest != digest(op.Request) || json.Unmarshal(op.Receipt, &receipt) != nil ||
			receipt.ValidateFor(b.LifecycleRequest{Snapshot: s, Operation: op, Input: candidate.Input}) != nil || receipt.Disposition != b.OperationConfirmed || !receipt.State.InputAccepted || receipt.State.SessionID != state.SessionID {
			return nil, errors.New("invalid committed structured input receipt")
		}
		input := candidate.Input.Turn
		if seenCommands[input.CommandID] {
			return nil, errors.New("duplicate committed structured command")
		}
		seenCommands[input.CommandID] = true
		before := len(inputs)
		for _, e := range receipt.Implementation {
			if e.ImplementationKey != AgentSessionStateKey {
				continue
			}
			var saved agentSessionState
			if e.CodecVersion != AgentSessionVersion || e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest) != nil ||
				b.DecodeStrictJSON(e.Payload, &saved, "trailing structured input receipt") != nil || saved.SessionID != state.SessionID || saved.RuntimeID != state.RuntimeID || saved.Worktree.ID != state.Worktree.ID || saved.Worktree.InstanceID != state.Worktree.InstanceID ||
				saved.CommandID != input.CommandID || saved.InputDigest != digest([]byte(input.Prompt)) || saved.MessageID == "" || saved.TurnID == "" || saved.TurnID != receipt.State.TurnID || saved.TurnItemID == "" || saved.Epoch == "" || state.Epoch != "" && saved.Epoch != state.Epoch {
				return nil, errors.New("committed structured input binding changed")
			}
			inputs = append(inputs, saved)
			break
		}
		if len(inputs) == before {
			return nil, errors.New("committed structured input binding unavailable")
		}
	}
	return inputs, nil
}

func verifyDevelopmentHistory(s b.LifecycleSnapshot, current agentSessionState, items []native.AgentItem) (map[string]bool, error) {
	history, err := developmentAgentInputs(s, current)
	if err != nil {
		return nil, err
	}
	users := map[string]agentSessionState{"orca:" + current.MessageID: current}
	turns := map[string]bool{}
	nativeTurns := map[string]bool{}
	seenUsers, seenTurns := map[string]bool{}, map[string]bool{}
	for _, prior := range history {
		if prior.CommandID == current.CommandID {
			if prior.MessageID != current.MessageID || prior.InputDigest != current.InputDigest || prior.BodyFingerprint != current.BodyFingerprint || prior.TurnID != current.TurnID || prior.TurnItemID != current.TurnItemID || prior.ProviderItemID != current.ProviderItemID {
				return nil, errors.New("current structured input differs from committed receipt")
			}
			continue
		}
		id := "orca:" + prior.MessageID
		if _, exists := users[id]; exists || nativeTurns[prior.TurnID] || turns[prior.TurnItemID] {
			return nil, errors.New("duplicate original structured input or turn")
		}
		users[id], nativeTurns[prior.TurnID], turns[prior.TurnItemID] = prior, true, true
	}
	for _, item := range items {
		var message struct {
			Kind, Role string
			Blocks     []native.AgentTextBlock
		}
		if err := json.Unmarshal(item.Body, &message); err != nil {
			return nil, err
		}
		if message.Kind == "message" && message.Role == "user" {
			binding, ok := users[item.ItemID]
			if !ok || len(message.Blocks) != 1 || message.Blocks[0].Type != "text" || digest([]byte(message.Blocks[0].Text)) != binding.InputDigest {
				return nil, errors.New("unexpected user input in original development session")
			}
			seenUsers[item.ItemID] = true
		}
		if message.Kind == "turn" {
			var turn native.AgentTurn
			if err := json.Unmarshal(item.Body, &turn); err != nil {
				return nil, err
			}
			binding, ok := users[turn.UserItemID]
			if !ok || seenTurns[turn.UserItemID] || turn.TurnID == "" || binding.TurnID != "" && (turn.TurnID != binding.TurnID || item.ItemID != binding.TurnItemID) || binding.CommandID == current.CommandID && nativeTurns[turn.TurnID] {
				return nil, errors.New("unexpected or reused native turn in original development session")
			}
			if binding.CommandID != current.CommandID && turn.State != "completed" && turn.State != "interrupted" {
				return nil, errors.New("historical structured turn is not terminal")
			}
			seenTurns[turn.UserItemID] = true
		}
	}
	for id, binding := range users {
		if binding.CommandID != current.CommandID && (!seenUsers[id] || !seenTurns[id]) {
			return nil, errors.New("original development history lost a committed input or turn")
		}
	}
	return turns, nil
}

func (l *Lifecycle) sendStructuredDevelopment(ctx context.Context, r b.LifecycleRequest, prior agentSessionState, reconcile bool) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	if !reconcile {
		receipt.Disposition = b.OperationRejected
	}
	input := r.Input.Turn
	if input == nil || r.Snapshot.PendingTurn == nil || input.CommandID != r.Snapshot.PendingTurn.CommandID || input.Prompt != r.Snapshot.PendingTurn.Prompt || !prior.Attempted || prior.TurnID == "" || prior.Epoch == "" || prior.CloseAttempted {
		return receipt, nil, errors.New("structured follow-up has no original input or live session binding")
	}
	state := prior
	state.CommandID, state.InputDigest, state.MessageID = input.CommandID, digest([]byte(input.Prompt)), agentSendID(r.Operation)
	state.TurnID, state.TurnItemID, state.ProviderItemID = "", "", ""
	profile, err := decodeAgentSessionProfile(l.setup)
	if err != nil {
		return receipt, nil, err
	}
	selection := &native.AgentSelection{Model: profile.Model, Effort: profile.ReasoningEffort}
	body := agentBody(input.Prompt)
	fields := map[string]any{"body": body}
	state.BodyFingerprint, err = native.AgentFingerprint("agentSession.send", state.SessionID, fields)
	if err != nil {
		return receipt, nil, err
	}
	if selection.Model == "" && selection.Effort == "" {
		selection = nil
	} else {
		fields["selection"] = selection
	}
	if !reconcile {
		if r.Snapshot.Session.LastStoppedTurnID != prior.TurnID {
			return receipt, nil, errors.New("structured follow-up requires a distinct input and the original terminal turn")
		}
		items, page, err := l.agentHistory(ctx, prior)
		if err != nil {
			return receipt, nil, err
		}
		observed, _, err := l.verifyAgentActivity(r.Snapshot, prior, items, page)
		if err != nil {
			return receipt, nil, err
		}
		if !observed.State.TurnCompleted || page.Working || page.Fence <= 0 || len(page.ActionablePromptIDs) != 0 {
			return receipt, nil, errors.New("original structured session is not verified idle")
		}
		fingerprint, err := native.AgentFingerprint("agentSession.send", state.SessionID, fields)
		if err != nil {
			return receipt, nil, err
		}
		request := native.AgentSend{Body: body, Selection: selection, Envelope: native.AgentMutationEnvelope{SessionID: state.SessionID, ClientOperationID: state.MessageID, PayloadFingerprint: fingerprint, ExpectedRuntimeFence: &page.Fence}}
		receipt.Disposition = b.OperationUnknown
		if _, err := l.agentClient(state.RuntimeID).Send(ctx, request); err != nil {
			return receipt, nil, err
		}
	}
	snapshot := r.Snapshot.Clone()
	snapshot.Implementation, err = l.agentEnvelope(snapshot, state)
	if err != nil {
		return receipt, nil, err
	}
	observed, err := l.readStructured(ctx, snapshot)
	if err != nil {
		return receipt, nil, err
	}
	observed.Operation = r.Operation
	if !observed.State.InputAccepted {
		return observed, nil, nil
	}
	observed.Disposition = b.OperationConfirmed
	snapshot.Implementation = observed.Implementation
	snapshot.Session.CurrentTurnID = ""
	if observed.State.TurnStarted {
		snapshot.Session.CurrentTurnID = observed.State.TurnID
	}
	return observed, l.observeStructured(snapshot), nil
}
