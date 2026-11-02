package orcatransport

import (
	"context"
	"errors"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestObserverSingleAckPreservesWaitFailureAndCancellation(t *testing.T) {
	for _, name := range []string{"single_success", "commit_failure", "close_while_waiting"} {
		t.Run(name, func(t *testing.T) {
			// Use a valid original preparation binding without creating resources
			// or sending input. The first optional event precedes source queries.
			l, _, snapshot := fixture(t)
			o := l.observe(snapshot, &b.PublicMessage{Text: "first"}).(*observer)
			t.Cleanup(func() {
				_ = o.Close(context.Background())
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := o.awaitCleanup(ctx); err != nil {
					t.Error("observer leaked while waiting for ack", err)
				}
			})
			var event b.LifecycleEvent
			select {
			case event = <-o.Events():
			case <-time.After(time.Second):
				t.Fatal("first event unavailable")
			}
			if event.PublicMessage == nil || event.Persisted == nil {
				t.Fatal("first event lost its acknowledgement boundary")
			}
			select {
			case <-o.Events():
				t.Fatal("observer advanced before acknowledgement")
			case <-o.finished:
				t.Fatal("observer ended before acknowledgement")
			case <-time.After(40 * time.Millisecond):
			}
			switch name {
			case "single_success":
				event.Persisted <- nil
				select {
				case next := <-o.Events():
					if next.Persisted == nil || next.Observation == nil {
						t.Fatal("one ack did not release original observation")
					}
				case err := <-o.Done():
					t.Fatal("observer ended instead of continuing", err)
				case <-time.After(time.Second):
					t.Fatal("observer required a second acknowledgement")
				}
			case "commit_failure":
				failure := errors.New("consumer commit failed")
				event.Persisted <- failure
				select {
				case err := <-o.Done():
					if !errors.Is(err, failure) {
						t.Fatal("consumer error was lost", err)
					}
				case <-time.After(time.Second):
					t.Fatal("consumer failure did not end observation")
				}
				if next, ok := <-o.Events(); ok {
					t.Fatal("failed event advanced observation", next)
				}
			case "close_while_waiting":
				closed := make(chan struct{})
				go func() { _ = o.Close(context.Background()); close(closed) }()
				select {
				case <-closed:
				case <-time.After(time.Second):
					t.Fatal("passive close blocked on an unacknowledged event")
				}
			}
		})
	}
}
