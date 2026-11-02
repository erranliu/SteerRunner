package application

import (
	"context"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type ackBoundaryObserver struct{ events chan b.LifecycleEvent }

func (o *ackBoundaryObserver) Events() <-chan b.LifecycleEvent { return o.events }
func (*ackBoundaryObserver) Done() <-chan error                { return nil }
func (*ackBoundaryObserver) StopHandle() b.StopHandle          { return nil }
func (*ackBoundaryObserver) Close(context.Context) error       { return nil }

func TestSREventAckPreservesOriginalCommitBoundary(t *testing.T) {
	for _, name := range []string{"commit", "commit_failure", "duplicate", "optional_message"} {
		t.Run(name, func(t *testing.T) {
			e, w, db := operationEngine(t)
			r := e.runs["run"].Clone()
			r.Status, r.Phase = "pr_ready", "observing"
			r.Snapshot.Session.SessionID, r.Snapshot.Session.CurrentTurnID = "session", "turn"
			r.Snapshot.Session.Generation = "session-generation"
			if err := e.commitLocked(r, nil, "original_binding"); err != nil {
				t.Fatal(err)
			}
			event := b.LifecycleEvent{
				Observation: &b.ObservationStatus{Diagnostic: "original observation gap"},
				Fact:        b.EventFact{Kind: b.EventStatus, Source: "native", SourceEventID: "ack-event", SessionID: "session", StartedTurnID: "turn"},
				Persisted:   make(chan error, 1),
			}
			if name == "duplicate" {
				if err := e.acceptEvent("run", event); err != nil {
					t.Fatal(err)
				}
			}
			if name == "optional_message" {
				event = b.LifecycleEvent{PublicMessage: &b.PublicMessage{AttemptID: "attempt", Generation: "session-generation", SessionID: "session", TurnID: "turn", Kind: "assistant", Text: "original message", ObservedAt: time.Now().UTC()}, Persisted: make(chan error, 1)}
			}
			before := e.runs["run"].Revision
			if name != "commit" {
				failOperationCommit(t, db)
			}
			ctx, cancel := context.WithCancel(context.Background())
			e.ctx = ctx
			o := &ackBoundaryObserver{events: make(chan b.LifecycleEvent, 1)}
			w.observer, w.port = o, nil
			finished := make(chan struct{})
			go func() { defer close(finished); e.run("run", w) }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-finished:
				case <-time.After(2 * time.Second):
					t.Error("event worker did not stop")
				}
			})
			// The actual worker must not reply while acceptEvent is unable to
			// complete. This also detects a reply moved before the consumer call.
			e.mu.Lock()
			o.events <- event
			select {
			case <-event.Persisted:
				e.mu.Unlock()
				t.Fatal("event acknowledged before its consumer completed")
			case <-time.After(40 * time.Millisecond):
			}
			e.mu.Unlock()
			var ack error
			select {
			case ack = <-event.Persisted:
			case <-time.After(2 * time.Second):
				t.Fatal("event acknowledgement unavailable")
			}
			if (ack != nil) != (name == "commit_failure") {
				t.Fatalf("wrong consumer result: %v", ack)
			}
			cancel()
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("event worker failed to detach")
			}
			runs, err := e.store.Runs()
			if err != nil || len(runs) != 1 {
				t.Fatal("durable run unavailable", err)
			}
			want := before
			if name == "commit" {
				want++
			}
			if runs[0].Revision != want || e.runs["run"].Revision != want {
				t.Fatal("ack did not match atomic durable and published state", want, runs[0].Revision, e.runs["run"].Revision)
			}
			_, seen := runs[0].SeenNative["attempt:native:ack-event"]
			if seen != (name == "commit" || name == "duplicate") {
				t.Fatal("failed or optional event changed durable source facts", seen)
			}
			if name == "optional_message" && e.lastMessages["run"].Text != "original message" {
				t.Fatal("optional message became a transaction gate")
			}
			select {
			case <-event.Persisted:
				t.Fatal("consumer replied more than once")
			default:
			}
		})
	}
}
