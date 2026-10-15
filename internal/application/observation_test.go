package application

import (
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestSRObservationUnchangedProjectionStillCommitsIdentity(t *testing.T) {
	for _, kind := range []b.EventKind{b.EventCompleted, b.EventQuestion} {
		t.Run(string(kind), func(t *testing.T) {
			e, _, db := operationEngine(t)
			event := b.LifecycleEvent{Fact: b.EventFact{Kind: kind, Source: "native", SourceEventID: "new-event"}}
			if kind == b.EventQuestion {
				r := e.runs["run"].Clone()
				r.Snapshot.Session.Generation = "session-generation"
				r.Snapshot.Session.CurrentTurnID = "turn"
				if err := e.commitLocked(r, nil, "turn_started"); err != nil {
					t.Fatal(err)
				}
				route, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: 1, Purpose: b.ImplementationQuestionPurpose, ImplementationKey: "codex", ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt", Generation: "session-generation", TurnID: "turn", QuestionID: "question"}, []byte(`{}`))
				if err != nil {
					t.Fatal(err)
				}
				event.Fact.QuestionRoute = &route
				event.Fact.QuestionText = "Original question"
				if err := e.acceptEvent("run", event); err != nil {
					t.Fatal(err)
				}
				event.Fact.SourceEventID = "same-question-new-event"
			}
			revision, seen := e.runs["run"].Revision, len(e.runs["run"].SeenNative)
			failOperationCommit(t, db)
			if err := e.acceptEvent("run", event); err == nil {
				t.Fatal("event with unchanged public projection was acknowledged without committing its new identity")
			}
			if e.runs["run"].Revision != revision || len(e.runs["run"].SeenNative) != seen {
				t.Fatal("failed event commit changed memory")
			}
			if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
				t.Fatal(err)
			}
			if err := e.acceptEvent("run", event); err != nil {
				t.Fatal(err)
			}
			accepted := e.runs["run"]
			if len(accepted.SeenNative) != seen+1 {
				t.Fatal("replayed event did not retain its identity")
			}
			event.Fact.QuestionText = "conflicting payload"
			if err := e.acceptEvent("run", event); err == nil || e.runs["run"].Revision != accepted.Revision {
				t.Fatal("conflicting duplicate escaped detection because the projection was unchanged")
			}
		})
	}
}

func TestSRObservationCommitFailureKeepsEventReplayable(t *testing.T) {
	e, _, db := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Session.SessionID = "session"
	r.Snapshot.Session.PendingCommandID = "command"
	r.Snapshot.Session.PendingTurnID = "turn"
	if err := e.commitLocked(r, nil, "input_accepted"); err != nil {
		t.Fatal(err)
	}
	revision := e.runs["run"].Revision
	event := b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventStarted, SessionID: "session", StartedTurnID: "turn", Source: "native", SourceEventID: "original-event"}}
	failOperationCommit(t, db)
	if err := e.acceptEvent("run", event); err == nil {
		t.Fatal("event transaction did not fail")
	}
	failed := e.runs["run"]
	if failed.Revision != revision || len(failed.SeenNative) != 0 || failed.Snapshot.Session.CurrentTurnID != "" || failed.Snapshot.Session.PendingCommandID != "command" {
		t.Fatal("uncommitted event consumed its identity or changed the accepted command")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
		t.Fatal(err)
	}
	if err := e.acceptEvent("run", event); err != nil {
		t.Fatal(err)
	}
	accepted := e.runs["run"]
	if accepted.Snapshot.Session.CurrentCommandID != "command" || accepted.Snapshot.Session.CurrentTurnID != "turn" || !accepted.NativeStarted || len(accepted.SeenNative) != 1 {
		t.Fatal("replayed original start lost its accepted command binding")
	}
	if err := e.acceptEvent("run", event); err != nil || e.runs["run"].Revision != accepted.Revision {
		t.Fatal("duplicate native event was not idempotent", err)
	}
	event.Fact.QuestionText = "conflicting contents under the same source event"
	if err := e.acceptEvent("run", event); err == nil || e.runs["run"].Revision != accepted.Revision {
		t.Fatal("conflicting duplicate overwrote the original event")
	}
}
