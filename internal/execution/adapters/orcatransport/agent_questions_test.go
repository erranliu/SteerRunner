package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

func agentQuestionFixture(t *testing.T, async bool) (*Lifecycle, b.LifecycleSnapshot, *agentFixture) {
	t.Helper()
	l, snapshot, fixture := structuredFixture(t)
	l.agentDevelopment = true
	fixture.pages[0].Fence = 7
	fixture.pages[0].Working = true
	turn := &fixture.pages[1].Items[1]
	var turnBody native.AgentTurn
	if err := json.Unmarshal(turn.Body, &turnBody); err != nil {
		t.Fatal(err)
	}
	turn.Body = mustAgentJSON(map[string]any{"kind": "turn", "turnId": turnBody.TurnID, "userItemId": turnBody.UserItemID, "state": "running", "startedAt": 10})
	item := native.AgentItem{ItemID: "question-item", Revision: 3, Sequence: 8}
	item.TurnScope.Kind, item.TurnScope.TurnItemID = "turn", turn.ItemID
	if async {
		item.Body = json.RawMessage(`{"kind":"message","role":"assistant","blocks":[{"type":"tool-call","name":"request_user_input_async","callId":"question-call","input":{"questions":[{"title":"Which mode?","options":["Fast","Safe"]}]}},{"type":"tool-result","callId":"question-call"}]}`)
	} else {
		item.Body = json.RawMessage(`{"kind":"question","question":"Which mode?","options":[{"id":"fast","label":"Fast"},{"id":"safe","label":"Safe"}],"freeTextQuestionId":"mode","freeTextInput":{},"resolution":{"state":"pending"}}`)
		fixture.pages[0].ActionablePromptIDs = []string{item.ItemID}
		fixture.detail = item
	}
	fixture.pages[0].Items = append(fixture.pages[0].Items, item)
	return l, snapshot, fixture
}

func prepareAgentQuestionReply(t *testing.T, l *Lifecycle, snapshot b.LifecycleSnapshot) b.LifecycleRequest {
	t.Helper()
	receipt, err := l.readStructured(context.Background(), snapshot)
	if err != nil || !receipt.State.InputAccepted || len(receipt.Facts) != 1 {
		t.Fatal("verified original question was not observed", receipt.State, receipt.Facts, err)
	}
	fact := receipt.Facts[0]
	if fact.QuestionText != "Which mode?" || len(fact.Questions) != 1 || !fact.Questions[0].AllowFreeText || len(fact.Questions[0].Options) != 2 {
		t.Fatal("question presentation differs from the Orca request", fact)
	}
	snapshot.Implementation = receipt.Implementation
	snapshot.Session.SessionID, snapshot.Session.Generation, snapshot.Session.CurrentTurnID = receipt.State.SessionID, receipt.State.SessionGeneration, receipt.State.TurnID
	snapshot.QuestionRoute = fact.QuestionRoute
	answer := b.Answer{Answers: map[string][]string{fact.Questions[0].ID: {"Safe"}}}
	parts, err := l.PrepareLifecycleAnswer(*fact.QuestionRoute, snapshot.Session, answer, "Safe")
	if err != nil {
		t.Fatal(err)
	}
	prepared, budget, err := m.FreezeInput(parts...)
	if err != nil {
		t.Fatal(err)
	}
	input := &b.FeedbackInput{DecisionID: strings.Repeat("b", 32), QuestionID: fact.QuestionRoute.QuestionID, Message: "Safe", Answer: &answer, PreparedAnswer: prepared, Budget: budget}
	r := request(snapshot, b.LifecycleDeliverFeedback, b.LifecycleInput{Feedback: input})
	r.Operation.ID = strings.Repeat("a", 32)
	r.Operation.StartedAt = time.UnixMilli(1793680000123)
	r.Operation.Request = mustAgentJSON(input)
	r.Operation.PayloadDigest = digest(r.Operation.Request)
	return r
}

func TestOrcaNativeQuestionAnswer(t *testing.T) {
	for _, mode := range []string{"direct", "lost-response", "stale"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			l, s, f := agentQuestionFixture(t, false)
			req := prepareAgentQuestionReply(t, l, s)
			writes := 0
			f.respond = func(reply native.AgentQuestionReply) (json.RawMessage, error) {
				writes++
				if reply.ItemID != "question-item" || reply.ExpectedRevision != 3 || *reply.Envelope.ExpectedRuntimeFence != 7 ||
					len(reply.Answers) != 1 || reply.Answers[0].QuestionID != "mode" || len(reply.Answers[0].OptionIDs) != 1 || reply.Answers[0].OptionIDs[0] != "safe" {
					t.Fatalf("reply lost native identity or answer: %+v", reply)
				}
				f.detail.Revision++
				f.detail.Body = json.RawMessage(`{"kind":"question","resolution":{"state":"resolved","answers":[{"questionId":"mode","optionIds":["safe"]}]}}`)
				if mode == "lost-response" {
					return nil, errors.New("lost reply")
				}
				return json.RawMessage(`{"ok":true}`), nil
			}
			if mode == "stale" {
				f.pages[0].ActionablePromptIDs = nil
			}
			result, _, err := l.Execute(ctx, req)
			switch mode {
			case "stale":
				if result.Disposition != b.OperationRejected || writes != 0 || err == nil {
					t.Fatal("stale question sent input", result.Disposition, writes, err)
				}
				return
			case "lost-response":
				if result.Disposition != b.OperationUnknown || writes != 1 || err == nil {
					t.Fatal("lost response was incorrectly settled", result.Disposition, writes, err)
				}
				result, _, err = l.Reconcile(ctx, req)
			}
			if result.Disposition != b.OperationConfirmed || writes != 1 || err != nil {
				t.Fatal("original reply not confirmed exactly once", result.Disposition, writes, err)
			}
		})
	}
}

func TestOrcaAsyncQuestionAnswerUsesOriginalTurn(t *testing.T) {
	for _, mode := range []string{"direct", "queued", "wrong-turn", "completed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			l, s, f := agentQuestionFixture(t, true)
			req := prepareAgentQuestionReply(t, l, s)
			if req.Input.Feedback.PreparedAnswer != "> Which mode?\n\nSafe" {
				t.Fatal("reply did not use the Orca async answer message format")
			}
			writes := 0
			var send native.AgentSend
			bindReply := func() {
				queuedID, messageID := "", send.Envelope.ClientOperationID
				if mode == "queued" {
					queuedID, messageID = messageID, "host-allocated-steer-message"
				}
				submission := native.AgentSubmission{ClientMessageID: messageID, QueuedMessageID: queuedID,
					PayloadFingerprint: send.Envelope.PayloadFingerprint, DispatchState: "accepted"}
				item := native.AgentItem{ItemID: "orca:" + messageID, Sequence: 11, Body: mustAgentJSON(send.Body)}
				item.TurnScope.Kind, item.TurnScope.TurnItemID = "turn", "original-turn-item"
				if mode == "wrong-turn" {
					item.TurnScope.TurnItemID = "different-turn-item"
				}
				f.pages[0].Submissions = append(f.pages[0].Submissions, submission)
				f.pages[0].Items = append(f.pages[0].Items, item)
			}
			f.send = func(request native.AgentSend) error {
				send = request
				writes++
				if send.Envelope.SessionID != req.Snapshot.Session.SessionID || *send.Envelope.ExpectedRuntimeFence != 7 ||
					send.Body.Blocks[0].Text != req.Input.Feedback.PreparedAnswer {
					t.Fatal("async answer lost its original session binding", send)
				}
				if mode != "queued" {
					bindReply()
				}
				return nil
			}
			if mode == "completed" {
				f.pages[0].Working = false
			}
			got, _, err := l.Execute(ctx, req)
			if mode == "completed" {
				if got.Disposition != b.OperationRejected || err == nil || writes != 0 {
					t.Fatal("completed turn authorized new user input", got.Disposition, writes, err)
				}
				return
			}
			if mode == "queued" {
				if got.Disposition != b.OperationUnknown || err != nil || writes != 1 {
					t.Fatal("queued draft was counted as delivered", got.Disposition, writes, err)
				}
				bindReply()
				got, _, err = l.Reconcile(ctx, req)
			}
			if mode == "wrong-turn" {
				if got.Disposition != b.OperationUnknown || writes != 1 || err != nil {
					t.Fatal("different turn accepted as the answer's original turn", got.Disposition, writes, err)
				}
				return
			}
			if got.Disposition != b.OperationConfirmed || writes != 1 || err != nil || got.DeliveryID == "" {
				t.Fatal("async reply was not confirmed on the original turn", got.Disposition, writes, err)
			}
			operation := req.Operation
			operation.Status = b.OperationConfirmed
			operation.Receipt = mustAgentJSON(got)
			req.Snapshot.Operations = append(req.Snapshot.Operations, operation)
			result, err := l.readStructured(ctx, req.Snapshot)
			if err != nil || len(result.Facts) != 0 || !result.State.InputAccepted {
				t.Fatal("confirmed async reply broke original turn observation or resurfaced the question", result.State, result.Facts, err)
			}
		})
	}
}
