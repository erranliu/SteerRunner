package application

import (
	"reflect"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

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
	event := b.LifecycleEvent{Observation: &b.ObservationStatus{Diagnostic: "original final unavailable"}, Fact: b.EventFact{Kind: b.EventStatus, Source: "orca.terminal.observation", SourceEventID: "gap", SessionID: "session", StartedTurnID: "turn"}}
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
