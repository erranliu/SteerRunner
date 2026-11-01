package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

type answerInputPort struct {
	b.LifecyclePort
	preparations, sends, queries int
	unknown, reject, oversize    bool
	request                      b.LifecycleRequest
}

func (p *answerInputPort) PrepareLifecycleAnswer(route b.ImplementationDataEnvelope, fixed b.SessionContext, answer b.Answer, message string) ([]m.InputPart, error) {
	p.preparations++
	if p.reject || route.ImplementationKey != "future.answer" || route.Generation != fixed.Generation || answer.Answers["q"][0] != message {
		return nil, errors.New("original answer route unavailable")
	}
	if p.oversize {
		return []m.InputPart{{Source: "native.answer_frame", Owner: m.RunnerInput, Text: strings.Repeat("x", 1001)}}, nil
	}
	return []m.InputPart{{Source: "native.answer_frame", Owner: m.RunnerInput, Text: "future representation:"}, {Source: "caller.answer", Owner: m.CallerInput, Text: message}}, nil
}
func (p *answerInputPort) Execute(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.sends++
	p.request = request.Clone()
	status := b.OperationAccepted
	if p.unknown {
		status = b.OperationUnknown
	}
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: status}, nil, nil
}
func (p *answerInputPort) Reconcile(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.queries++
	if request.Operation.ID != p.request.Operation.ID || !reflect.DeepEqual(request.Input, p.request.Input) {
		return b.LifecycleReceipt{}, nil, errors.New("original answer changed")
	}
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationAccepted}, nil, nil
}
func TestAnswerInputCapabilityKeepsIntentAndUnknownBoundaries(t *testing.T) {
	for _, mode := range []string{"accepted", "unknown", "failed-commit", "unsupported", "invalid-route", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			e, w, db := operationEngine(t)
			e.epoch = "current"
			r := e.runs["run"].Clone()
			r.Status = "waiting_input"
			r.Snapshot.Session.SessionID = "session"
			r.Snapshot.Session.Generation = "generation"
			r.Snapshot.Session.CurrentTurnID = "turn"
			route, err := b.NewQuestionRoute("future.answer", 99, b.QuestionRouteIdentity{ProjectID: r.Snapshot.Session.ProjectID, WorkflowRunID: "run", ExecutionID: "run", AttemptID: r.Snapshot.Session.AttemptID, Generation: "generation", TurnID: "turn", QuestionID: "question"}, []byte(`{"future_field":"no application decoder"}`))
			if err != nil {
				t.Fatal(err)
			}
			r.Snapshot.QuestionRoute = &route
			r.Snapshot.PendingQuestions = []b.Question{{ID: "q", AllowFreeText: true}}
			r.Question = &contracts.QuestionView{ID: "question"}
			r.PendingAnswer = &runstate.Answer{Epoch: e.epoch, CommandID: "reply", QuestionID: "question", Text: "回答😀"}
			r.CommandStates["reply"] = "pending"
			if err := e.commitLocked(r, nil, "question_fixture"); err != nil {
				t.Fatal(err)
			}
			p := &answerInputPort{unknown: mode == "unknown", reject: mode == "invalid-route", oversize: mode == "oversize"}
			w.port = p
			if mode == "unsupported" {
				w.port = &operationCommitPort{}
			}
			if mode == "failed-commit" {
				failOperationCommit(t, db)
			}
			w.gate.Lock()
			advanced := e.answer("run", w, w.port)
			w.gate.Unlock()
			current := e.runs["run"]
			if mode == "unsupported" || mode == "invalid-route" || mode == "failed-commit" || mode == "oversize" {
				if advanced || len(current.Intents) != 0 || p.sends != 0 || p.queries != 0 || current.Question == nil {
					t.Fatal("unprepared or uncommitted answer crossed the effect boundary")
				}
				return
			}
			if p.preparations != 1 || p.sends != 1 || len(current.Intents) != 1 || !reflect.DeepEqual(current.Intents[0].Request.Snapshot.QuestionRoute, &route) {
				t.Fatal("opaque route did not stay original")
			}
			budget := p.request.Input.Feedback.Budget
			if err := m.CheckInput("future representation:回答😀", budget); err != nil || budget.CallerBytes != len("回答😀") {
				t.Fatal("application replaced adapter fragments", budget, err)
			}
			if mode == "unknown" {
				if advanced || current.PendingAnswer == nil || current.Question == nil {
					t.Fatal("unknown answer lost its original question")
				}
				e.advance("run", w)
				if p.sends != 1 || p.queries != 1 || e.runs["run"].CommandStates["reply"] != "completed" {
					t.Fatal("unknown original answer was resent or not reconciled")
				}
			} else if !advanced || current.CommandStates["reply"] != "completed" {
				t.Fatal("confirmed answer did not settle its command")
			}
		})
	}
}
