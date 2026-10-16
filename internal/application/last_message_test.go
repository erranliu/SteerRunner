package application

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLastMessageIsBoundedReadOnlyTelemetry(t *testing.T) {
	t.Setenv("GH_TOKEN", "fixture-private-credential")
	r := &runstate.Record{ID: "run", Revision: 7, Status: "running"}
	r.Snapshot.Session = b.SessionContext{AttemptID: "attempt", Generation: "gen", SessionID: "session", CurrentTurnID: "turn"}
	e := &Engine{runs: map[string]*runstate.Record{"run": r}}
	before := r.Clone()
	at := time.Now().UTC()
	m := b.PublicMessage{AttemptID: "attempt", Generation: "gen", SessionID: "session", TurnID: "turn", Kind: "assistant", Text: "fixture-private-credential bearer abc123 token=unknown " + strings.Repeat("消息", 600), ObservedAt: at}
	m.Text = `{"password":"hidden-value"} ` + m.Text
	if err := e.acceptEvent("run", b.LifecycleEvent{PublicMessage: &m}); err != nil {
		t.Fatal(err)
	}
	original := e.lastMessages["run"]
	if len(original.Text) > 1203 || strings.Contains(original.Text, "hidden-value") || strings.Contains(original.Text, "fixture-private-credential") || strings.Contains(original.Text, "abc123") || strings.Contains(original.Text, "unknown") {
		t.Fatal("unbounded or unredacted telemetry", original.Text)
	}
	for _, change := range []func(*b.PublicMessage){func(x *b.PublicMessage) { x.AttemptID = "old" }, func(x *b.PublicMessage) { x.Generation = "old" }, func(x *b.PublicMessage) { x.SessionID = "other" }, func(x *b.PublicMessage) { x.TurnID = "old" }, func(x *b.PublicMessage) { x.Kind = "reasoning" }, func(x *b.PublicMessage) { x.ObservedAt = at.Add(-time.Second) }} {
		next := m
		next.Text = "replacement"
		next.ObservedAt = at.Add(time.Second)
		change(&next)
		if err := e.acceptMessage("run", next); err != nil {
			t.Fatal("telemetry interrupted execution", err)
		}
		if e.lastMessages["run"] != original {
			t.Fatal("stale/private telemetry replaced sample")
		}
	}
	for i := 0; i < 2; i++ {
		response := e.Handle("last-message", "run", "", "", "", 0, false, nil)
		view := response.Data.(contracts.LastMessageView)
		if view.Message == nil || !view.Message.ObservedAt.Equal(at) {
			t.Fatal("query changed timestamp")
		}
	}
	if !reflect.DeepEqual(before, r.Clone()) {
		t.Fatal("telemetry changed execution aggregate")
	}
	r.Snapshot.Session.AttemptID = "new"
	if e.Handle("last-message", "run", "", "", "", 0, false, nil).Data.(contracts.LastMessageView).Message != nil {
		t.Fatal("old Attempt leaked")
	}
	restarted := &Engine{runs: e.runs}
	if restarted.Handle("last-message", "run", "", "", "", 0, false, nil).Data.(contracts.LastMessageView).Message != nil {
		t.Fatal("restart invented sample")
	}
}
