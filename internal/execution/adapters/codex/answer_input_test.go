package codexworker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

func TestAnswerInputPreservesNativeBytesAndOwnership(t *testing.T) {
	fixed := b.SessionContext{ProjectID: "p", WorkflowRunID: "w", ExecutionID: "e", AttemptID: "a", Generation: "g", SessionID: "00000000-0000-0000-0000-000000000001", CurrentTurnID: "00000000-0000-0000-0000-000000000002"}
	answer := b.Answer{Answers: map[string][]string{"q": {"中文😀\n\""}}}
	message := "原始回答😀\n下一行"
	for _, delivery := range []string{"", "async"} {
		t.Run("delivery="+delivery, func(t *testing.T) {
			payload, _ := json.Marshal(AskRouteData{RequestID: json.RawMessage(`7`), Delivery: delivery, SourceCommandID: "00000000000000000000000000000001", ThreadID: fixed.SessionID, TurnID: fixed.CurrentTurnID, CallID: "question"})
			route, err := b.NewQuestionRoute(QuestionRouteImplementationKey, 1, b.QuestionRouteIdentity{ProjectID: fixed.ProjectID, WorkflowRunID: fixed.WorkflowRunID, ExecutionID: fixed.ExecutionID, AttemptID: fixed.AttemptID, Generation: fixed.Generation, TurnID: fixed.CurrentTurnID, QuestionID: "question"}, payload)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal([]any{route, fixed, answer})
			// A zero lifecycle has no process/transport capabilities: preparation
			// must remain pure, including original-route validation.
			var owner b.LifecycleAnswerInputPort = ExecutionLifecycle{}
			parts, err := owner.PrepareLifecycleAnswer(route, fixed, answer, message)
			if err != nil {
				t.Fatal(err)
			}
			text, budget, err := m.FreezeInput(parts...)
			want, caller := `{"answers":{"q":["中文😀\n\""]}}`, 14
			if delivery == "async" {
				want, caller = message, len(message)
			}
			digest := sha256.Sum256([]byte(want))
			if err != nil || text != want || budget.SHA256 != hex.EncodeToString(digest[:]) || budget.CallerBytes != caller || budget.RunnerBytes != len(want)-caller {
				t.Fatal("native answer bytes or ownership changed", text, budget, err)
			}
			after, _ := json.Marshal([]any{route, fixed, answer})
			if string(before) != string(after) {
				t.Fatal("preparation mutated original facts")
			}
			foreign := fixed
			foreign.Generation = "another-generation"
			if parts, err := owner.PrepareLifecycleAnswer(route, foreign, answer, message); err == nil || parts != nil {
				t.Fatal("foreign original route produced input")
			}
			unsupported := route.Clone()
			unsupported.CodecVersion = 99
			if parts, err := owner.PrepareLifecycleAnswer(unsupported, fixed, answer, message); err == nil || parts != nil {
				t.Fatal("unknown route codec produced input")
			}
		})
	}
}
