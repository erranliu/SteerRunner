package application

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestOrcaObservationProjectionTransitionsAndPriority(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Phase = "observing"
	r.Snapshot.Session.SessionID = "original"
	r.Snapshot.Session.CurrentTurnID = "turn"
	r.Snapshot.Session.CurrentCommandID = "turn"
	r.Snapshot.Session.Generation = "session-generation"
	if err := e.commitLocked(r, nil, "started"); err != nil {
		t.Fatal(err)
	}
	event := b.LifecycleEvent{InputWait: true, Fact: b.EventFact{Kind: b.EventStatus, SessionID: "original", StartedTurnID: "turn"}}
	for _, message := range []string{"missing prompt", "", "missing prompt", "missing body", ""} {
		event.Diagnostic = message
		before := e.runs["run"].Clone()
		if err := e.acceptEvent("run", event); err != nil {
			t.Fatal(err)
		}
		next := e.runs["run"]
		wantPhase := observationUnverified
		if message == "" {
			wantPhase = "observing"
		}
		if next.Revision != before.Revision+1 || next.Status != "running" || next.Phase != wantPhase || next.Diagnostic != message || !next.Allowed || next.ResumeRequired || !reflect.DeepEqual(next.SeenNative, before.SeenNative) {
			t.Fatal("projection changed authority or lost transition", next)
		}
		if err := e.acceptEvent("run", event); err != nil || e.runs["run"].Revision != next.Revision {
			t.Fatal("duplicate observation wrote a revision", err)
		}
	}
	for _, status := range []string{"blocked", "reconciling", "waiting_input", "stopped", "pr_ready"} {
		t.Run(status, func(t *testing.T) {
			next := e.runs["run"].Clone()
			next.Status, next.Phase, next.Diagnostic = status, "owned-by-other-stage", "keep original cause"
			e.runs["run"] = next
			before := next.Clone()
			for _, d := range []string{"native problem", ""} {
				event.Diagnostic = d
				if err := e.acceptEvent("run", event); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(before, e.runs["run"]) {
				t.Fatal("another stage's cause was overwritten")
			}
		})
	}
	// A current-value retry after the higher-priority stage is gone is not
	// suppressed by any permanent native-event consumption key.
	next := e.runs["run"].Clone()
	next.Status, next.Phase, next.Diagnostic = "running", "observing", ""
	e.runs["run"] = next
	event.Diagnostic = "native problem"
	if err := e.acceptEvent("run", event); err != nil || e.runs["run"].Phase != observationUnverified {
		t.Fatal(err)
	}
	stale := event
	stale.Fact.StartedTurnID = "other turn"
	before := e.runs["run"].Clone()
	if err := e.acceptEvent("run", stale); !errors.Is(err, reliable.ErrStale) || !reflect.DeepEqual(before, e.runs["run"]) {
		t.Fatal("stale diagnostic accepted", err)
	}
	// Reattachment of unchanged accepted input must retain the observation
	// phase until an explicit clear arrives.
	if err := merge(next, b.LifecycleReceipt{State: b.LifecycleStateFacts{SessionID: "original", TurnID: "turn", InputAccepted: true}}); err != nil {
		t.Fatal(err)
	}
	projected := e.runs["run"].Clone()
	if err := merge(projected, b.LifecycleReceipt{State: b.LifecycleStateFacts{SessionID: "original", TurnID: "turn", InputAccepted: true}}); err != nil || projected.Phase != observationUnverified || projected.Diagnostic != "native problem" {
		t.Fatal("reattachment erased diagnostic", err)
	}
}

func TestOrcaObservationProjectionAndPublicMessageRemainSeparate(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Phase = "observing"
	r.Snapshot.Session.SessionID, r.Snapshot.Session.Generation, r.Snapshot.Session.CurrentTurnID = "session", "native-generation", "turn"
	if err := e.commitLocked(r, nil, "started"); err != nil {
		t.Fatal(err)
	}
	before := e.runs["run"].Clone()
	message := b.PublicMessage{AttemptID: before.Snapshot.Session.AttemptID, Generation: "native-generation", SessionID: "session", TurnID: "turn", Kind: "assistant", Text: "public native message", ObservedAt: time.Now().UTC()}
	if err := e.acceptEvent("run", b.LifecycleEvent{PublicMessage: &message}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, e.runs["run"]) {
		t.Fatal("message changed aggregate")
	}
	for i := 0; i < 2; i++ {
		v := e.Handle("last-message", "run", "", "", "", 0, false, nil).Data.(contracts.LastMessageView)
		if v.Message == nil || !v.Message.ObservedAt.Equal(message.ObservedAt) {
			t.Fatal(v)
		}
	}
	if !reflect.DeepEqual(before, e.runs["run"]) {
		t.Fatal("message query changed aggregate")
	}
	event := b.LifecycleEvent{InputWait: true, Diagnostic: "original final unavailable", Fact: b.EventFact{SessionID: "session", StartedTurnID: "turn"}}
	if err := e.acceptEvent("run", event); err != nil || e.runs["run"].Revision != before.Revision+1 {
		t.Fatal("independent diagnostic was lost", err)
	}
	if e.lastMessages["run"].Text != message.Text {
		t.Fatal("diagnostic replaced message")
	}
}

func TestLastMessageSameTextFromNewTurnRetainsOriginalIdentities(t *testing.T) {
	e, _, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Session.SessionID, r.Snapshot.Session.Generation, r.Snapshot.Session.CurrentTurnID = "session", "session-generation", "first"
	r.Snapshot.ResourceGeneration = "resource-generation"
	e.runs["run"] = r
	m := b.PublicMessage{AttemptID: r.Snapshot.Session.AttemptID, Generation: "session-generation", SessionID: "session", TurnID: "first", Kind: "assistant", Text: "same native reply", ObservedAt: time.Now().UTC()}
	if err := e.acceptEvent("run", b.LifecycleEvent{PublicMessage: &m}); err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Session.CurrentTurnID = "second"
	view := func() *contracts.LastMessage {
		return e.Handle("last-message", "run", "", "", "", 0, false, nil).Data.(contracts.LastMessageView).Message
	}
	if view() == nil || view().TurnID != "first" {
		t.Fatal("previous sample was relabelled before new evidence")
	}
	m.TurnID, m.ObservedAt = "second", m.ObservedAt.Add(time.Second)
	wrong := m
	wrong.Generation = r.Snapshot.ResourceGeneration
	if err := e.acceptEvent("run", b.LifecycleEvent{PublicMessage: &wrong}); err != nil || view().TurnID != "first" {
		t.Fatal("resource generation bypassed session fence", err)
	}
	if err := e.acceptEvent("run", b.LifecycleEvent{PublicMessage: &m}); err != nil || view().TurnID != "second" || !view().ObservedAt.Equal(m.ObservedAt) {
		t.Fatal("same-text new turn was suppressed", err)
	}
	m.ObservedAt = m.ObservedAt.Add(time.Second)
	if err := e.acceptEvent("run", b.LifecycleEvent{PublicMessage: &m}); err != nil || view().ObservedAt.Equal(m.ObservedAt) {
		t.Fatal("same-turn final refresh changed time", err)
	}
}
