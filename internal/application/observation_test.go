package application

import (
	"context"
	"errors"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type observationTestHandle struct{ owned bool }

func (h *observationTestHandle) Identity() b.ProcessIdentity {
	return b.ProcessIdentity{Kind: "test-observation"}
}
func (h *observationTestHandle) Kill()              {}
func (h *observationTestHandle) Close()             {}
func (h *observationTestHandle) OwnedProcess() bool { return h.owned }

type attachmentObserver struct {
	handle b.StopHandle
	closed int
}

func (o *attachmentObserver) Events() <-chan b.LifecycleEvent { return nil }
func (o *attachmentObserver) Done() <-chan error              { return nil }
func (o *attachmentObserver) StopHandle() b.StopHandle        { return o.handle }
func (o *attachmentObserver) Close(context.Context) error {
	o.closed++
	return nil
}

type observationAttachmentPort struct {
	b.LifecyclePort
	observer b.LifecycleObserver
	err      error
	calls    int
}

func (p *observationAttachmentPort) ObserveLifecycle(context.Context, b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.calls++
	return b.LifecycleReceipt{State: b.LifecycleStateFacts{SessionID: "session"}}, p.observer, p.err
}

func TestSRObservationAttachmentCommitsBeforePublishingObserver(t *testing.T) {
	for _, outcome := range []string{"native_error", "commit_failure", "success"} {
		t.Run(outcome, func(t *testing.T) {
			e, w, db := operationEngine(t)
			r := e.runs["run"].Clone()
			r.Snapshot.Session.SessionID = "session"
			r.Snapshot.Session.CurrentTurnID = "original-turn"
			r.Allowed, r.ResumeRequired = false, true
			bindTestWorkflow(r)
			if err := e.commitLocked(r, nil, "turn_started"); err != nil {
				t.Fatal(err)
			}
			revision := e.runs["run"].Revision
			owned := &observationTestHandle{owned: true}
			w.handle = owned
			observer := &attachmentObserver{handle: &observationTestHandle{}}
			p := &observationAttachmentPort{observer: observer}
			w.port = p
			if outcome == "native_error" {
				p.err = errors.New("observation unavailable")
			} else if outcome == "commit_failure" {
				failOperationCommit(t, db)
			}
			if e.advance("run", w) || p.calls != 1 || w.handle != owned || e.runs["run"].Allowed || !e.runs["run"].ResumeRequired {
				t.Fatal("read-only observation changed authorization or replaced the owned writer handle")
			}
			if outcome == "success" {
				if w.observer != observer || observer.closed != 0 || e.runs["run"].Revision != revision+1 {
					t.Fatal("committed observation was not attached")
				}
				e.advance("run", w)
				if p.calls != 1 {
					t.Fatal("an existing observation was attached twice")
				}
			} else if w.observer != nil || observer.closed != 1 || outcome == "commit_failure" && e.runs["run"].Revision != revision {
				t.Fatal("failed observation was published or its temporary observer leaked")
			}
		})
	}
}

func TestSRObservationUnchangedProjectionStillCommitsIdentity(t *testing.T) {
	for _, kind := range []b.EventKind{b.EventCompleted, b.EventQuestion} {
		t.Run(string(kind), func(t *testing.T) {
			e, _, db := operationEngine(t)
			event := b.LifecycleEvent{Fact: b.EventFact{Kind: kind, Source: "native", SourceEventID: "new-event"}}
			if kind == b.EventQuestion {
				r := e.runs["run"].Clone()
				r.Snapshot.Session.Generation = "session-generation"
				r.Snapshot.Session.CurrentTurnID = "turn"
				bindTestWorkflow(r)
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
	bindTestWorkflow(r)
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
