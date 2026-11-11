package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

const agentQuestionRouteKey = "orca.agent-session-question"

type agentPrompt struct {
	ID                 string `json:"id"`
	Header             string `json:"header"`
	Text               string `json:"question"`
	MultiSelect        bool   `json:"multiSelect"`
	FreeTextQuestionID string `json:"freeTextQuestionId"`
	Options            []struct {
		ID, Label, Description string
	} `json:"options"`
}

type agentQuestion struct {
	agentPrompt
	Kind       string
	Questions  []agentPrompt
	Resolution struct {
		State   string
		Answers []native.AgentQuestionAnswer
	}
}

type agentQuestionRoute struct {
	ItemID, SessionID, Epoch, TurnItemID string
	Revision                             int64
	CallID                               string
	Questions                            []b.Question
}

func agentQuestionIdentity(s b.SessionContext, id string) b.QuestionRouteIdentity {
	return b.QuestionRouteIdentity{ProjectID: s.ProjectID, WorkflowRunID: s.WorkflowRunID, ExecutionID: s.ExecutionID,
		AttemptID: s.AttemptID, Generation: s.Generation, TurnID: s.CurrentTurnID, QuestionID: id}
}

func readAgentQuestion(item native.AgentItem) (agentQuestion, error) {
	var body agentQuestion
	if err := json.Unmarshal(item.Body, &body); err != nil {
		return body, errors.New("invalid original Orca prompt item")
	}
	return body, nil
}

func (q agentQuestion) runnerQuestions() ([]b.Question, error) {
	prompts := q.Questions
	if len(prompts) == 0 {
		prompt := q.agentPrompt
		prompt.ID = prompt.FreeTextQuestionID
		if prompt.ID == "" {
			prompt.ID = "q1"
		}
		prompts = []agentPrompt{prompt}
	}
	if len(prompts) > 4 {
		return nil, errors.New("original Orca question group exceeds the supported size")
	}
	questions := make([]b.Question, 0, len(prompts))
	seen := map[string]bool{}
	for _, p := range prompts {
		if p.ID == "" || seen[p.ID] || strings.TrimSpace(p.Text) == "" || p.MultiSelect {
			return nil, errors.New("unsupported original Orca question shape")
		}
		seen[p.ID] = true
		question := b.Question{ID: p.ID, Header: p.Header, Question: p.Text, AllowFreeText: p.FreeTextQuestionID != ""}
		if len(p.Options) == 0 && !question.AllowFreeText {
			return nil, errors.New("original Orca question has no answer choices")
		}
		optionIDs, labels := map[string]bool{}, map[string]bool{}
		for _, option := range p.Options {
			if option.ID == "" || strings.TrimSpace(option.Label) == "" || optionIDs[option.ID] || labels[option.Label] {
				return nil, errors.New("ambiguous original Orca question options")
			}
			optionIDs[option.ID], labels[option.Label] = true, true
			question.Options = append(question.Options, b.DecisionOption{ID: option.ID, Label: option.Label, Detail: option.Description})
		}
		questions = append(questions, question)
	}
	return questions, nil
}

func (l *Lifecycle) agentQuestionFact(ctx context.Context, s b.LifecycleSnapshot, state agentSessionState, page native.AgentPage, items []native.AgentItem) (*b.EventFact, error) {
	if !l.agentDevelopment || s.Ref.Agent.Name != "codex" {
		return nil, nil
	}
	for _, id := range page.ActionablePromptIDs {
		var item native.AgentItem
		if index := slices.IndexFunc(items, func(item native.AgentItem) bool { return item.ItemID == id }); index >= 0 {
			item = items[index]
		} else {
			var err error
			item, err = l.agentClient(state.RuntimeID).Item(ctx, state.SessionID, id)
			if err != nil {
				return nil, err
			}
		}
		if item.TurnScope.Kind != "turn" || item.TurnScope.TurnItemID != state.TurnItemID || item.ItemID != id || item.Revision <= 0 {
			continue
		}
		body, err := readAgentQuestion(item)
		if err != nil {
			return nil, err
		}
		if body.Kind != "question" || body.Resolution.State != "pending" {
			continue
		}
		questions, err := body.runnerQuestions()
		if err != nil {
			return nil, err
		}
		return makeAgentQuestionFact(s, state, item, "", questions)
	}
	if len(page.ActionablePromptIDs) != 0 {
		return nil, nil
	}
	return agentAsyncQuestion(s, state, items, "")
}

func makeAgentQuestionFact(s b.LifecycleSnapshot, state agentSessionState, item native.AgentItem, callID string, questions []b.Question) (*b.EventFact, error) {
	data := agentQuestionRoute{ItemID: item.ItemID, SessionID: state.SessionID, Epoch: state.Epoch, TurnItemID: state.TurnItemID,
		Revision: item.Revision, CallID: callID, Questions: questions}
	if callID != "" {
		data.Revision = 0 // async calls retain their identity as their containing message grows
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	id := digest(payload)[:32]
	session := s.Session
	session.CurrentTurnID, session.Generation = state.TurnID, s.ResourceGeneration
	route, err := b.NewQuestionRoute(agentQuestionRouteKey, 1, agentQuestionIdentity(session, id), payload)
	if err != nil {
		return nil, err
	}
	return &b.EventFact{Kind: b.EventQuestion, Source: agentResultSource, SourceIdentity: state.SessionID, SourceSegment: state.TurnID,
		SourceEventID: "question:" + id, SessionID: state.SessionID, StartedTurnID: state.TurnID, QuestionText: questions[0].Question,
		Questions: questions, QuestionRoute: &route}, nil
}

func decodeAgentQuestionRoute(route b.ImplementationDataEnvelope, s b.SessionContext) (agentQuestionRoute, error) {
	var data agentQuestionRoute
	if route.ImplementationKey != agentQuestionRouteKey || route.CodecVersion != 1 ||
		route.ValidateIdentity(agentQuestionIdentity(s, route.QuestionID)) != nil ||
		b.DecodeStrictJSON(route.Payload, &data, "trailing structured question route") != nil ||
		data.SessionID != s.SessionID || data.ItemID == "" || data.TurnItemID == "" || data.Epoch == "" || data.CallID == "" && data.Revision <= 0 {
		return data, errors.New("original Orca question route is invalid")
	}
	return data, nil
}

func (l *Lifecycle) PrepareLifecycleAnswer(route b.ImplementationDataEnvelope, s b.SessionContext, answer b.Answer, _ string) ([]m.InputPart, error) {
	data, err := decodeAgentQuestionRoute(route, s)
	if err != nil {
		return nil, err
	}
	if _, err := agentQuestionAnswers(data, answer); err != nil {
		return nil, err
	}
	return agentAnswerParts(data, answer), nil
}

func agentQuestionAnswers(data agentQuestionRoute, answer b.Answer) ([]native.AgentQuestionAnswer, error) {
	if len(answer.Answers) != len(data.Questions) || len(data.Questions) == 0 {
		return nil, errors.New("answer must match the original Orca question group")
	}
	answers := make([]native.AgentQuestionAnswer, 0, len(data.Questions))
	for _, q := range data.Questions {
		values := answer.Answers[q.ID]
		if len(values) == 0 {
			return nil, errors.New("answer omitted an original Orca question")
		}
		reply := native.AgentQuestionAnswer{QuestionID: q.ID, OptionIDs: []string{}}
		for _, v := range values {
			option := slices.IndexFunc(q.Options, func(o b.DecisionOption) bool { return o.Label == v })
			switch {
			case option >= 0 && len(reply.OptionIDs) == 0:
				reply.OptionIDs = append(reply.OptionIDs, q.Options[option].ID)
			case option < 0 && q.AllowFreeText && reply.Other == nil && strings.TrimSpace(v) != "" && len(v) <= 64<<10:
				text := v
				reply.Other = &text
			default:
				return nil, errors.New("answer differs from the original Orca choices")
			}
		}
		if len(reply.OptionIDs) > 0 && reply.Other != nil {
			return nil, errors.New("choose one option or enter a free-text answer")
		}
		answers = append(answers, reply)
	}
	return answers, nil
}

func (l *Lifecycle) deliverAgentQuestion(ctx context.Context, r b.LifecycleRequest, reconcile bool) (b.LifecycleReceipt, error) {
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	if !reconcile {
		receipt.Disposition = b.OperationRejected
	}
	feedback, route := r.Input.Feedback, r.Snapshot.QuestionRoute
	if !l.agentDevelopment || r.Snapshot.Ref.Agent.Name != "codex" || feedback == nil || feedback.Answer == nil || route == nil || feedback.QuestionID != route.QuestionID {
		return receipt, errors.New("original Orca question is unavailable")
	}
	data, err := decodeAgentQuestionRoute(*route, r.Snapshot.Session)
	if err != nil {
		return receipt, err
	}
	state, err := l.agentState(r.Snapshot)
	if err != nil {
		return receipt, err
	}
	if data.SessionID != state.SessionID || data.Epoch != state.Epoch || data.TurnItemID != state.TurnItemID || state.TurnID != r.Snapshot.Session.CurrentTurnID || state.CloseAttempted {
		return receipt, errors.New("original Orca question binding changed")
	}
	answers, err := agentQuestionAnswers(data, *feedback.Answer)
	if err != nil {
		return receipt, err
	}
	parts := agentAnswerParts(data, *feedback.Answer)
	if prepared, budget, err := m.FreezeInput(parts...); err != nil || prepared != feedback.PreparedAnswer || !reflect.DeepEqual(budget, feedback.Budget) {
		return receipt, errors.New("original Orca answer budget changed")
	}
	if data.CallID != "" {
		return l.deliverAgentAsyncAnswer(ctx, r, state, data, reconcile)
	}
	client := l.agentClient(state.RuntimeID)
	page, err := client.History(ctx, state.SessionID, nil, 1)
	if err != nil {
		return receipt, err
	}
	if page.Epoch != state.Epoch || page.Fence <= 0 {
		return receipt, errors.New("original Orca question epoch or runtime fence is unavailable")
	}
	item, err := client.Item(ctx, state.SessionID, data.ItemID)
	if err != nil {
		return receipt, err
	}
	body, err := readAgentQuestion(item)
	if err != nil {
		return receipt, err
	}
	if item.ItemID != data.ItemID || item.TurnScope.Kind != "turn" || item.TurnScope.TurnItemID != data.TurnItemID || body.Kind != "question" {
		return receipt, errors.New("original Orca question item changed")
	}
	if reconcile {
		if item.Revision > data.Revision && body.Resolution.State == "resolved" && reflect.DeepEqual(body.Resolution.Answers, answers) {
			receipt.Disposition = b.OperationConfirmed
		}
		return receipt, nil
	}
	if item.Revision != data.Revision || body.Resolution.State != "pending" || !slices.Contains(page.ActionablePromptIDs, item.ItemID) {
		return receipt, errors.New("original Orca question is no longer pending")
	}
	request := native.AgentQuestionReply{Envelope: native.AgentMutationEnvelope{SessionID: state.SessionID,
		ClientOperationID: agentSendID(r.Operation), ExpectedRuntimeFence: &page.Fence}, ItemID: data.ItemID, ExpectedRevision: data.Revision, Answers: answers}
	request.Envelope.PayloadFingerprint, err = request.Fingerprint()
	if err != nil {
		return receipt, err
	}
	receipt.Disposition = b.OperationUnknown
	receipt.Operation.Receipt, err = client.RespondToQuestion(ctx, request)
	if err != nil {
		return receipt, fmt.Errorf("original Orca question response: %w", err)
	}
	receipt.Disposition = b.OperationConfirmed
	return receipt, nil
}
