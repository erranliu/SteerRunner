package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"io"
	"log"
	"strings"
	"testing"
)

type noMessageProbePort struct{ auditPort }

func (noMessageProbePort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{}, errors.New("message must not probe the original session")
}

func TestDevelopmentRunMessageStopCancelsOnlyUnsentInput(t *testing.T) {
	for _, flow := range []string{"development", "development-audited"} {
		for _, window := range []string{"saved", "frozen", "input-intent", "input-rejected", "input-accepted", "answer", "answer-intent", "answer-rejected"} {
			t.Run(flow+"/"+window, func(t *testing.T) {
				e, w := contentEngine(t, "conversation")
				e.finish("run", contentEvidence(e, "original result", "succeeded", true))
				r := e.runs["run"].Clone()
				p, _ := workflow.Lookup(flow, 2)
				r.Request.Workflow = contracts.Workflow{ID: flow, Version: 2}
				r.Request.Conversation = nil
				r.Workflow.Config, r.Workflow.StepID, r.Status = &p, workflow.StepPRReady, "pr_ready"
				r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: "original", ReasoningEffort: "medium"}
				if p.AuditModelRequired {
					r.Workflow.AuditModel = &workflow.ModelSettings{Model: "auditor", ReasoningEffort: "high"}
				}
				if err := e.commitLocked(r, nil, "published"); err != nil {
					t.Fatal(err)
				}
				if response := contentControl(e, "message", "pending-message", contracts.MessageInput{ExpectedRevision: r.Revision, Text: "original message"}); !response.OK {
					t.Fatal(response)
				}
				command := e.runs["run"].PendingFeedback.CommandID
				if window != "saved" && !e.feedback("run", w, w.port) {
					t.Fatal("message did not freeze")
				}
				if strings.HasPrefix(window, "input-") {
					r = e.runs["run"]
					if _, err := e.saveIntent("run", r, b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: r.Snapshot.PendingTurn}}); err != nil {
						t.Fatal(err)
					}
				}
				if strings.HasPrefix(window, "answer") {
					_ = contentEvidence(e, "", "succeeded", true)
					r = e.runs["run"].Clone()
					r.Snapshot.PendingTurn = nil
					r.CommandStates[command] = "completed"
					r.Status, r.Question = "waiting_input", &contracts.QuestionView{ID: "original-question"}
					if err := e.commitLocked(r, nil, "question"); err != nil {
						t.Fatal(err)
					}
					if response := contentControl(e, "reply", "pending-answer", contracts.ReplyInput{ExpectedRevision: r.Revision, Kind: "answer", QuestionID: "original-question", Text: "original answer"}); !response.OK {
						t.Fatal(response)
					}
					command = e.runs["run"].PendingAnswer.CommandID
					if window != "answer" {
						if _, err := e.saveIntent("run", e.runs["run"], b.LifecycleCandidate{Action: b.LifecycleDeliverFeedback, Input: b.LifecycleInput{Feedback: &b.FeedbackInput{DecisionID: command, Message: "original answer"}}}); err != nil {
							t.Fatal(err)
						}
					}
				}
				r = e.runs["run"]
				if strings.HasSuffix(window, "rejected") {
					r = r.Clone()
					r.Intents[len(r.Intents)-1].Request.Operation.Status = b.OperationRejected
					if err := e.commitLocked(r, nil, "input_rejected"); err != nil {
						t.Fatal(err)
					}
				}
				if window == "input-accepted" {
					r = r.Clone()
					intent := &r.Intents[len(r.Intents)-1]
					intent.Request.Operation.Status = b.OperationAccepted
					intent.Receipt = &b.LifecycleReceipt{State: b.LifecycleStateFacts{InputAccepted: true, ResourcePending: true}}
					r.CommandStates[command] = "completed"
					if err := e.commitLocked(r, nil, "input_accepted"); err != nil {
						t.Fatal(err)
					}
				}
				intents := string(canonical(r.Intents))
				if response := contentControl(e, "stop", "stop-message", contracts.RevisionInput{ExpectedRevision: r.Revision}); !response.OK {
					t.Fatal(response)
				}
				r = e.runs["run"]
				committed := window == "input-intent" || window == "answer-intent" || window == "input-accepted"
				want := "failed"
				if committed {
					want = "unknown"
				}
				if window == "input-accepted" {
					want = "completed"
				}
				if r.CommandStates[command] != want || string(canonical(r.Intents)) != intents {
					t.Fatal("stop confused unsent input with an original native intent", r.CommandStates)
				}
				if !committed && (r.PendingFeedback != nil || r.PendingAnswer != nil || r.Snapshot.PendingTurn != nil || r.Snapshot.Session.PendingCommandID != "" || r.Snapshot.Session.PendingTurnID != "") {
					t.Fatal("stop left unsent input pending")
				}
				if committed && (strings.HasPrefix(window, "input-") && r.Snapshot.PendingTurn == nil || window == "answer-intent" && r.PendingAnswer == nil) {
					t.Fatal("stop lost original uncertain input")
				}
				settleRunStop(r)
				r.CommandStates[r.PendingStop], r.PendingStop = "completed", ""
				if r.Workflow.StepID != workflow.StepPRReady || r.View().Controls.CanMessage || r.Allowed {
					t.Fatal("stop changed the milestone or authorized new input")
				}
				if err := e.commitLocked(r, nil, "stop_confirmed"); err != nil {
					t.Fatal(err)
				}
				restarted, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(restarted.cancel)
				var inspected struct {
					Command struct {
						ActionStatus string `json:"action_status"`
					} `json:"command"`
				}
				request := "pending-message"
				if strings.HasPrefix(window, "answer") {
					request = "pending-answer"
				}
				response := restarted.Handle("inspect", "", request, "", "", 0, nil)
				if !response.OK || json.Unmarshal(canonical(response.Data), &inspected) != nil || inspected.Command.ActionStatus != r.CommandStates[command] {
					t.Fatal("original input settlement did not survive restart", response)
				}
			})
		}
	}
}

func TestDevelopmentRunMessagesRetainOriginalWorkflow(t *testing.T) {
	for _, flow := range []string{"development", "development-audited"} {
		t.Run(flow, func(t *testing.T) {
			e, w := contentEngine(t, "conversation")
			e.finish("run", contentEvidence(e, "original PR result", "succeeded", true))
			r := e.runs["run"].Clone()
			p, _ := workflow.Lookup(flow, 2)
			r.Request.Workflow = contracts.Workflow{ID: flow, Version: 2}
			r.Request.Conversation = nil
			r.Snapshot.Session.Generation = r.Snapshot.ResourceGeneration
			r.Request.Execution.Settings = []byte(`{"access_mode":"full_access"}`)
			r.Workflow.Config, r.Workflow.StepID = &p, workflow.StepPRReady
			r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: "original", ReasoningEffort: "medium"}
			if p.AuditModelRequired {
				r.Workflow.AuditModel = &workflow.ModelSettings{Model: "auditor", ReasoningEffort: "high"}
			}
			r.Workflow.ImplementationResultID, r.Workflow.AuditResultID, r.Workflow.PublicationResultID = "implementation", "audit", "publication"
			r.PR = &contracts.PRFact{URL: "https://example.test/repo/pull/1", State: "open"}
			r.Status = "pr_ready"
			w.port = noMessageProbePort{}
			if err := e.commitLocked(r, nil, "published"); err != nil {
				t.Fatal(err)
			}
			session, attempt, workspace := r.Snapshot.Session.SessionID, r.Snapshot.Session.AttemptID, r.Snapshot.Workspace.Path
			for _, request := range []string{"first-message", "second-message"} {
				r = e.runs["run"]
				if !r.View().Controls.CanMessage || r.View().Controls.CanReply {
					t.Fatal("wrong milestone controls", r.View())
				}
				input := contracts.MessageInput{ExpectedRevision: r.Revision, Text: "caller-only instruction"}
				if response := contentControl(e, "message", request, input); !response.OK {
					t.Fatal(response)
				}
				if e.runs["run"].View().Final != nil {
					t.Fatal("old final presented as current message")
				}
				if !contentControl(e, "message", request, input).OK {
					t.Fatal("original receipt did not replay")
				}
				if response := contentControl(e, "message", request+"-concurrent", contracts.MessageInput{ExpectedRevision: e.runs["run"].Revision, Text: "parallel"}); response.OK {
					t.Fatal("concurrent message accepted")
				}
				if request == "first-message" {
					restarted, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(restarted.cancel)
					restarted.workers["run"] = w
					e = restarted
				}
				before := len(e.runs["run"].Workflow.StageInputs)
				if !e.feedback("run", w, w.port) {
					t.Fatal("original message did not freeze")
				}
				r = e.runs["run"]
				stage := r.Workflow.StageInputs[len(r.Workflow.StageInputs)-1]
				if stage.Prompt != input.Text || stage.Model != "original" || r.Workflow.StepID != workflow.StepPRReady || len(r.Workflow.StageInputs) != before+1 {
					t.Fatal("message gained rules or reran workflow", stage)
				}
				if err := e.checkDispatchSource(r, stage); err != nil {
					t.Fatal("message added material/source preflight", err)
				}
				outcome := "succeeded"
				if request == "second-message" {
					outcome = "unreported"
				}
				evidence := contentEvidence(e, "message result", outcome, true)
				if request == "first-message" {
					r = e.runs["run"]
					session := r.Snapshot.Session
					route, err := b.NewQuestionRoute("future.answer", 99, b.QuestionRouteIdentity{ProjectID: session.ProjectID, WorkflowRunID: "run", ExecutionID: "run", AttemptID: session.AttemptID, Generation: session.Generation, TurnID: session.CurrentTurnID, QuestionID: "message-question"}, []byte(`{}`))
					if err != nil {
						t.Fatal(err)
					}
					if err := e.acceptEvent("run", b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventQuestion, SessionID: session.SessionID, StartedTurnID: session.CurrentTurnID, QuestionRoute: &route, Questions: []b.Question{{ID: "q", AllowFreeText: true}}}}); err != nil {
						t.Fatal(err)
					}
					r = e.runs["run"]
					if !r.View().Controls.CanReply || r.View().Controls.CanMessage {
						t.Fatal("question did not serialize original message input")
					}
					if response := contentControl(e, "reply", "message-answer", contracts.ReplyInput{ExpectedRevision: r.Revision, Kind: "answer", QuestionID: route.QuestionID, Text: "回答😀"}); !response.OK {
						t.Fatal(response)
					}
					port := &answerInputPort{LifecyclePort: w.port}
					w.gate.Lock()
					answered := e.answer("run", w, port)
					w.gate.Unlock()
					r = e.runs["run"]
					if !answered || port.sends != 1 || port.request.Snapshot.QuestionRoute.QuestionID != route.QuestionID || port.request.Snapshot.Session.CurrentTurnID != session.CurrentTurnID || r.Question != nil || r.PendingAnswer != nil || len(r.Workflow.StageInputs) != before+1 {
						t.Fatal("answer lost its original question or restarted the message")
					}
					history := r.History
					if len(history) < 2 || history[len(history)-2].Role != "question" || history[len(history)-1].Role != "answer" || history[len(history)-1].Text != "回答😀" {
						t.Fatal("development message lost question/answer history")
					}
				}
				if !e.finish("run", evidence) {
					t.Fatal("message result lost")
				}
				r = e.runs["run"]
				if !r.View().Controls.CanMessage || r.Status != "pr_ready" || r.Snapshot.Session.SessionID != session || r.Snapshot.Session.AttemptID != attempt || r.Snapshot.Workspace.Path != workspace || r.Workflow.AuditResultID != "audit" || r.Workflow.ImplementationResultID != "implementation" || r.Workflow.PublicationResultID != "publication" || r.PR.URL != "https://example.test/repo/pull/1" || r.LastFinal.Text != "message result" || r.ContentResult != nil {
					t.Fatal("original run changed", r.View())
				}
			}
			if response := e.Handle("history", "run", "", "", "", 100, nil); !response.OK {
				t.Fatal("message history unavailable", response)
			}
			r = e.runs["run"].Clone()
			r.Allowed = false
			r.PendingStop = "original-stop"
			if err := e.commitLocked(r, nil, "stop"); err != nil {
				t.Fatal(err)
			}
			if r.View().Controls.CanMessage || contentControl(e, "message", "after-stop", contracts.MessageInput{ExpectedRevision: r.Revision, Text: "no"}).OK {
				t.Fatal("stop allowed new message")
			}
		})
	}
}
