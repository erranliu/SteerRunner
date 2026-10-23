package application

import (
	"context"
	"errors"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestSRObservationDiagnosticIsDurableAndRecoverable(t *testing.T) {
	e, _, db := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Phase = "observing"
	r.Snapshot.Session.SessionID = "session"
	r.Snapshot.Session.CurrentTurnID = "turn"
	r.Snapshot.Session.CurrentCommandID = "command"
	if err := e.commitLocked(r, nil, "turn_started"); err != nil {
		t.Fatal(err)
	}
	event := b.LifecycleEvent{Observation: &b.ObservationStatus{Diagnostic: "original metadata incomplete"}, Fact: b.EventFact{Kind: b.EventStatus, Source: "native", SourceEventID: "gap-1", SessionID: "session", StartedTurnID: "turn"}}
	before := e.runs["run"].Revision
	failOperationCommit(t, db)
	if err := e.acceptEvent("run", event); err == nil || e.runs["run"].Revision != before || e.runs["run"].Diagnostic != "" {
		t.Fatal("uncommitted diagnostic escaped transaction")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
		t.Fatal(err)
	}
	for i, diagnostic := range []string{"original metadata incomplete", "original query unavailable", ""} {
		event.Observation = &b.ObservationStatus{Diagnostic: diagnostic}
		event.Fact.SourceEventID = []string{"gap-1", "gap-2", "recovered"}[i]
		if err := e.acceptEvent("run", event); err != nil {
			t.Fatal(err)
		}
		current := e.runs["run"]
		view := current.View()
		if view.Diagnostic != diagnostic || view.Phase != current.Phase {
			t.Fatal("public Run view lost observation status", view.Diagnostic, view.Phase)
		}
		if current.Status != "running" || current.Diagnostic != diagnostic || !current.Allowed || current.ResumeRequired {
			t.Fatal("observation diagnostic changed authorization or disappeared", current.Diagnostic)
		}
		if (current.Phase == "observation_unverified") != (diagnostic != "") {
			t.Fatal("incorrect public observation phase", current.Phase)
		}
		stored, err := e.store.Runs()
		if err != nil || len(stored) != 1 || stored[0].Diagnostic != diagnostic {
			t.Fatal("diagnostic not durable", err)
		}
		if err := e.acceptEvent("run", event); err != nil || e.runs["run"].Revision != current.Revision {
			t.Fatal("duplicate observation was not idempotent", err)
		}
		conflicting := event.Clone()
		conflicting.Observation.Diagnostic = "different diagnostic under the same event ID"
		if err := e.acceptEvent("run", conflicting); err == nil {
			t.Fatal("conflicting diagnostic accepted")
		}
	}
	// Historical transitions expose the gap, its changed cause and its recovery.
	page, err := e.store.Events("run", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, event := range page.Events {
		if event.Transition != nil {
			seen[event.Transition.To.Diagnostic] = true
		}
	}
	if !seen["original metadata incomplete"] || !seen["original query unavailable"] || !seen[""] {
		t.Fatal("public transition history lost observation diagnostics", seen)
	}
}

func TestSRObservationDiagnosticBindingAndControlIsolation(t *testing.T) {
	for _, kind := range []string{"missing-session", "foreign-session", "missing-turn", "foreign-turn", "blocked", "stopped", "unrelated-phase"} {
		t.Run(kind, func(t *testing.T) {
			e, _, _ := operationEngine(t)
			r := e.runs["run"].Clone()
			r.Phase, r.Diagnostic = "observing", "existing"
			r.Snapshot.Session.SessionID, r.Snapshot.Session.CurrentTurnID = "session", "turn"
			event := b.LifecycleEvent{Observation: &b.ObservationStatus{Diagnostic: "gap"}, Fact: b.EventFact{Kind: b.EventStatus, Source: "native", SourceEventID: "gap", SessionID: "session", StartedTurnID: "turn"}}
			switch kind {
			case "missing-session":
				event.Fact.SessionID = ""
			case "foreign-session":
				event.Fact.SessionID = "foreign"
			case "missing-turn":
				event.Fact.StartedTurnID = ""
			case "foreign-turn":
				event.Fact.StartedTurnID = "foreign"
			case "blocked", "stopped":
				r.Status, r.Allowed, r.ResumeRequired = kind, false, true
			case "unrelated-phase":
				r.Phase = "parser_receipt_conflict"
			}
			if err := e.commitLocked(r, nil, "prepared"); err != nil {
				t.Fatal(err)
			}
			err := e.acceptEvent("run", event)
			invalid := kind == "missing-session" || kind == "foreign-session" || kind == "missing-turn" || kind == "foreign-turn"
			if invalid != (err != nil) {
				t.Fatal("incorrect observation binding verdict", err)
			}
			current := e.runs["run"]
			if current.Diagnostic != r.Diagnostic || current.Status != r.Status || current.Phase != r.Phase || current.Allowed != r.Allowed || current.ResumeRequired != r.ResumeRequired {
				t.Fatal("observation overwrote an unrelated control state")
			}
		})
	}
}

func TestSRObservationAttachmentUpdatesDiagnostic(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Phase = "observing"
	r.Snapshot.Session.SessionID, r.Snapshot.Session.CurrentTurnID = "session", "turn"
	if err := e.commitLocked(r, nil, "turn_started"); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range []string{"metadata unavailable", ""} {
		original := e.runs["run"].Clone()
		receipt := b.LifecycleReceipt{Diagnostic: diagnostic, State: b.LifecycleStateFacts{SessionID: "session", TurnID: "turn", InputAccepted: true}}
		if !e.applyObservation("run", original, receipt) || e.runs["run"].Diagnostic != diagnostic {
			t.Fatal("reattachment lost or retained a stale diagnostic", diagnostic)
		}
	}
}

func TestSRObservationGapSurvivesStartAcknowledgement(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Phase = "input_accepted_waiting_start"
	r.Snapshot.Session.SessionID = "session"
	r.Snapshot.Session.PendingCommandID, r.Snapshot.Session.PendingTurnID = "command", "turn"
	if err := e.commitLocked(r, nil, "input_accepted"); err != nil {
		t.Fatal(err)
	}
	gap := b.LifecycleEvent{Observation: &b.ObservationStatus{Diagnostic: "metadata incomplete"}, Fact: b.EventFact{Kind: b.EventStatus, Source: "native", SourceEventID: "gap", SessionID: "session", StartedTurnID: "turn"}}
	if err := e.acceptEvent("run", gap); err != nil {
		t.Fatal(err)
	}
	started := b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventStarted, Source: "native", SourceEventID: "started", SessionID: "session", StartedTurnID: "turn"}}
	if err := e.acceptEvent("run", started); err != nil {
		t.Fatal(err)
	}
	if r := e.runs["run"]; !r.NativeStarted || r.Phase != "observation_unverified" || r.Diagnostic != gap.Observation.Diagnostic {
		t.Fatal("start acknowledgement hid the observation gap")
	}
	gap.Fact.SourceEventID, gap.Observation.Diagnostic = "recovered", ""
	if err := e.acceptEvent("run", gap); err != nil {
		t.Fatal(err)
	}
	if r := e.runs["run"]; r.Phase != "observing" || r.Diagnostic != "" {
		t.Fatal("recovered observation did not return to the started turn")
	}
}

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
	return b.LifecycleReceipt{State: b.LifecycleStateFacts{SessionID: "session", SessionGeneration: "observed-generation"}}, p.observer, p.err
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
