package application

import (
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaAsyncQuestionProjectionAndResolution(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Status, r.Phase, r.Allowed = "running", "observing", true
	r.Snapshot.Ref.Protocol = b.ProtocolOrcaAgentSessionV1
	s := &r.Snapshot.Session
	s.SessionID, s.Generation, s.CurrentTurnID = "session", "generation", "turn"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	route, err := b.NewQuestionRoute("orca.agent-session-question", 1, b.QuestionRouteIdentity{
		ProjectID: s.ProjectID, WorkflowRunID: r.ID, ExecutionID: r.ID, AttemptID: s.AttemptID,
		Generation: s.Generation, TurnID: s.CurrentTurnID, QuestionID: "current-question",
	}, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	fact := b.EventFact{Kind: b.EventQuestion, Source: "orca", SourceEventID: "shown", SessionID: s.SessionID,
		StartedTurnID: s.CurrentTurnID, InteractionKind: "async", QuestionRoute: &route, QuestionText: "Choose a color",
		Questions: []b.Question{{ID: "color", Question: "Choose a color", Options: []b.DecisionOption{{ID: "blue", Label: "Blue"}}}}}
	if err := e.acceptEvent(r.ID, b.LifecycleEvent{Fact: fact}); err != nil {
		t.Fatal(err)
	}
	view := func() contracts.RunView { return e.runs[r.ID].View() }
	if got := view(); got.Status != "running" || !got.Controls.CanReply || got.Question == nil || got.Question.InteractionKind != "async" ||
		len(got.Question.NativeQuestions) != 1 || got.Question.NativeQuestions[0].Options[0].ID != "blue" {
		t.Fatal("async question did not reach the public running Run", got)
	}
	resolved := b.EventFact{Kind: b.EventQuestionResolved, Source: "orca", SourceEventID: "resolved", SessionID: s.SessionID,
		StartedTurnID: s.CurrentTurnID, QuestionRoute: &route}
	if err := e.acceptEvent(r.ID, b.LifecycleEvent{Fact: resolved}); err != nil {
		t.Fatal(err)
	}
	if got := view(); got.Status != "running" || got.Phase != "observing" || got.Controls.CanReply || got.Question != nil {
		t.Fatal("resolved question remained actionable or changed the native turn state", got)
	}
}
