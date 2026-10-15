package application

import (
	"context"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestSRNativeAnswerRejectsAmbiguityBeforeIntent(t *testing.T) {
	questions := []b.Question{
		{ID: "choice", Options: []b.DecisionOption{{ID: "yes", Label: "Yes"}}},
		{ID: "note", AllowFreeText: true},
	}
	route, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{
		CodecVersion: 1, Purpose: b.ImplementationQuestionPurpose, ImplementationKey: "codex",
		ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt",
		Generation: "generation", TurnID: "turn", QuestionID: "native",
	}, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		`{"choice":["yes"],"note":["note"],"extra":["unexpected"]}`,
		`{"choice":["yes","unexpected"],"note":["note"]}`,
		`{"choice":["yes"],"choice":["yes"],"note":["note"]}`,
		`{"choice":["yes"]}`,
		`{"choice":["unknown"],"note":["note"]}`,
		`{"choice":["yes"],"note":[" "]}`,
	} {
		t.Run(text, func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			r := &runstate.Record{ID: "run", Status: "waiting_input", Allowed: true,
				Question:      &contracts.QuestionView{ID: "native"},
				PendingAnswer: &runstate.Answer{Epoch: "epoch", CommandID: "reply", Text: text},
				CommandStates: map[string]string{"reply": "pending"},
			}
			r.Snapshot.PendingQuestions = questions
			r.Snapshot.QuestionRoute = &route
			e := &Engine{ctx: context.Background(), epoch: "epoch", store: store, runs: map[string]*runstate.Record{"run": r}}
			// A nil port proves rejection occurs before any native effect.
			if e.answer("run", &worker{}, nil) {
				t.Fatal("ambiguous answer advanced")
			}
			got := e.runs["run"]
			if len(got.Intents) != 0 || got.PendingAnswer != nil || got.CommandStates["reply"] != "failed" || got.Question == nil || got.Question.ID != "native" {
				t.Fatal("invalid answer altered the original question or created a native intent")
			}
		})
	}
	answers, err := nativeAnswers(questions, `{"choice":["yes"],"note":["details"]}`)
	if err != nil || answers["choice"][0] != "Yes" || answers["note"][0] != "details" || questions[0].Options[0].ID != "yes" {
		t.Fatal("valid native answer lost option labels or mutated question", answers, err)
	}
	if _, err := nativeAnswers(nil, "answer"); err == nil {
		t.Fatal("missing native questions accepted")
	}
}
