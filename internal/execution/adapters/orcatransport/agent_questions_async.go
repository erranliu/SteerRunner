package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

const codexAsyncQuestion = "request_user_input_async"

var questionLineBreaks = regexp.MustCompile(`[\r\n]+`)

type agentHistoryMessage struct {
	Kind, Role string
	Blocks     []agentHistoryBlock
}

func (msg agentHistoryMessage) inTurn(item native.AgentItem, turnItem string) bool {
	return msg.Kind == "message" && msg.Role == "assistant" && item.TurnScope.Kind == "turn" && item.TurnScope.TurnItemID == turnItem
}

// The public history projects Codex's async ask as an assistant tool call.
// Its tool result acknowledges the request, rather than answering the human question.
type agentHistoryBlock struct {
	Type, Text, Name, CallID, State string
	Input                           json.RawMessage
	IsError                         bool
}

func isAsyncAgentQuestion(block agentHistoryBlock) bool {
	return block.Type == "tool-call" && block.Name == codexAsyncQuestion
}

func asyncAgentCalls(items []native.AgentItem, turnItem string) map[string]bool {
	calls := map[string]bool{}
	for _, item := range items {
		var message agentHistoryMessage
		if json.Unmarshal(item.Body, &message) != nil || !message.inTurn(item, turnItem) {
			continue
		}
		for _, block := range message.Blocks {
			if isAsyncAgentQuestion(block) && block.CallID != "" {
				calls[block.CallID] = true
			}
		}
	}
	return calls
}

func agentAsyncQuestion(s b.LifecycleSnapshot, state agentSessionState, items []native.AgentItem, questionID string) (*b.EventFact, error) {
	answered := map[string]bool{}
	for _, op := range s.Operations {
		if op.Action != string(b.LifecycleDeliverFeedback) || op.Status != b.OperationConfirmed {
			continue
		}
		if candidate, err := b.CandidateForOperation(op); err == nil {
			answered[candidate.Input.Feedback.QuestionID] = true
		}
	}
	failed := map[string]bool{}
	for _, item := range items {
		var message agentHistoryMessage
		if json.Unmarshal(item.Body, &message) != nil || !message.inTurn(item, state.TurnItemID) {
			continue
		}
		for _, block := range message.Blocks {
			if block.Type == "tool-result" && block.IsError {
				failed[block.CallID] = true
			}
		}
	}
	for _, item := range slices.Backward(items) {
		var message agentHistoryMessage
		if json.Unmarshal(item.Body, &message) != nil || !message.inTurn(item, state.TurnItemID) {
			continue
		}
		for _, block := range slices.Backward(message.Blocks) {
			if !isAsyncAgentQuestion(block) || block.CallID == "" || block.State == "failed" || failed[block.CallID] {
				continue
			}
			questions := parseAsyncAgentQuestions(block.Input)
			if len(questions) == 0 {
				continue
			}
			fact, err := makeAgentQuestionFact(s, state, item, block.CallID, questions)
			if err != nil {
				return nil, err
			}
			if (questionID == "" || questionID == fact.QuestionRoute.QuestionID) && !answered[fact.QuestionRoute.QuestionID] {
				fact.InteractionKind = "async"
				return fact, nil
			}
		}
	}
	return nil, nil
}

func parseAsyncAgentQuestions(raw json.RawMessage) []b.Question {
	var input struct {
		Questions []struct {
			Title   string
			Options []string
		}
	}
	if json.Unmarshal(raw, &input) != nil {
		return nil
	}
	var result []b.Question
	for i, rawQuestion := range input.Questions {
		if strings.TrimSpace(rawQuestion.Title) == "" {
			return nil
		}
		q := b.Question{ID: fmt.Sprintf("q%d", i+1), Question: rawQuestion.Title, AllowFreeText: true}
		labels := map[string]bool{}
		for j, label := range rawQuestion.Options {
			if strings.TrimSpace(label) != "" && !labels[label] {
				q.Options = append(q.Options, b.DecisionOption{ID: fmt.Sprintf("option%d", j+1), Label: label})
				labels[label] = true
			}
		}
		result = append(result, q)
	}
	return result
}

func agentAnswerParts(data agentQuestionRoute, answer b.Answer) []m.InputPart {
	if data.CallID == "" {
		return m.AnswerParts(answer.Answers)
	}
	// Match Orca's async-answer composer. The question text frames the
	// caller's reply so several questions in one card are unambiguous.
	var parts []m.InputPart
	for _, q := range data.Questions {
		prefix := "> "
		if len(parts) != 0 {
			prefix = "\n\n> "
		}
		title := questionLineBreaks.ReplaceAllString(q.Question, " ")
		parts = append(parts, m.InputPart{Source: "native.answer_frame", Owner: m.RunnerInput, Text: prefix},
			m.InputPart{Source: "caller.material", Owner: m.CallerInput, Text: title},
			m.InputPart{Source: "native.answer_frame", Owner: m.RunnerInput, Text: "\n\n"},
			m.InputPart{Source: "caller.answer", Owner: m.CallerInput, Text: strings.Join(answer.Answers[q.ID], ", ")})
	}
	return parts
}

func (l *Lifecycle) deliverAgentAsyncAnswer(ctx context.Context, r b.LifecycleRequest, state agentSessionState, data agentQuestionRoute, reconcile bool) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	if !reconcile {
		receipt.Disposition = b.OperationRejected
	}
	id := agentSendID(r.Operation)
	items, page, err := l.agentHistory(ctx, state, id)
	if err != nil {
		return receipt, err
	}
	body := agentBody(r.Input.Feedback.PreparedAnswer)
	fields := map[string]any{"body": body}
	fingerprint, err := native.AgentFingerprint("agentSession.send", state.SessionID, fields)
	if err != nil {
		return receipt, err
	}
	if !reconcile {
		fact, err := agentAsyncQuestion(r.Snapshot, state, items, r.Input.Feedback.QuestionID)
		if err != nil {
			return receipt, err
		}
		active := false
		for _, item := range items {
			if item.ItemID != data.TurnItemID {
				continue
			}
			var turn native.AgentTurn
			active = json.Unmarshal(item.Body, &turn) == nil && turn.TurnID == state.TurnID && turn.State == "running"
			break
		}
		if !active || !page.Working || page.Fence <= 0 || len(page.ActionablePromptIDs) != 0 ||
			fact == nil || fact.QuestionRoute.Digest != r.Snapshot.QuestionRoute.Digest {
			return receipt, errors.New("original Orca async question is no longer available in its active turn")
		}
		request := native.AgentSend{Body: body, Envelope: native.AgentMutationEnvelope{SessionID: state.SessionID, ClientOperationID: id,
			ExpectedRuntimeFence: &page.Fence, PayloadFingerprint: fingerprint}}
		receipt.Disposition = b.OperationUnknown
		receipt.Operation.Receipt, err = l.agentClient(state.RuntimeID).Send(ctx, request)
		if err != nil {
			return receipt, err
		}
		// Orca can queue a steer while the native turn is working. Neither a
		// queued nor a transport acknowledgment proves delivery to that turn.
		items, page, err = l.agentHistory(ctx, state, id)
		if err != nil {
			return receipt, err
		}
	}
	for _, submission := range page.Submissions {
		if submission.ClientMessageID != id && submission.QueuedMessageID != id {
			continue
		}
		if submission.PayloadFingerprint != fingerprint {
			return receipt, errors.New("original Orca answer submission fingerprint changed")
		}
		if submission.DispatchState != "accepted" {
			continue
		}
		for _, item := range items {
			if item.ItemID != "orca:"+submission.ClientMessageID {
				continue
			}
			inTurn := submission.AnsweredTurn.TurnItemID == data.TurnItemID ||
				item.TurnScope.Kind == "turn" && item.TurnScope.TurnItemID == data.TurnItemID
			var message native.AgentMessage
			if inTurn && json.Unmarshal(item.Body, &message) == nil && message.Kind == "message" && message.Role == "user" &&
				len(message.Blocks) == 1 && message.Blocks[0] == body.Blocks[0] {
				receipt.DeliveryID = item.ItemID
				receipt.Disposition = b.OperationConfirmed
				return receipt, nil
			}
		}
	}
	return receipt, nil
}

func acceptedAgentAnswerInputs(s b.LifecycleSnapshot) map[string]string {
	replies := map[string]string{}
	for _, op := range s.Operations {
		if op.Action != string(b.LifecycleDeliverFeedback) || op.Status != b.OperationConfirmed {
			continue
		}
		candidate, err := b.CandidateForOperation(op)
		if err != nil || digest(op.Request) != op.PayloadDigest {
			continue
		}
		var receipt b.LifecycleReceipt
		if json.Unmarshal(op.Receipt, &receipt) != nil || receipt.DeliveryID == "" || receipt.Disposition != b.OperationConfirmed ||
			receipt.ValidateFor(b.LifecycleRequest{Snapshot: s, Operation: op, Input: candidate.Input}) != nil {
			continue
		}
		replies[receipt.DeliveryID] = digest([]byte(candidate.Input.Feedback.PreparedAnswer))
	}
	return replies
}
