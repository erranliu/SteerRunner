package orcatransport

import (
	"context"
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"testing"
)

func TestTerminalStopReconcilesOnlyProvenUnattemptedClose(t *testing.T) {
	for _, attempted := range []bool{false, true} {
		t.Run(map[bool]string{false: "offline-before-close", true: "lost-close-response"}[attempted], func(t *testing.T) {
			l, f, s := fixture(t)
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
			f.failShow = !attempted
			f.unknownClose = attempted
			r, err = l.StopLifecycle(context.Background(), q, nil)
			if err == nil || r.State.Stopped {
				t.Fatal("fault did not remain unverified", r, err)
			}
			q.Operation.Receipt, _ = json.Marshal(r)
			f.failShow = false
			f.unknownClose = false
			before := len(f.calls)
			r, _, err = l.Reconcile(context.Background(), q)
			closes := 0
			for _, c := range f.calls[before:] {
				if c[0] == "terminal" && c[1] == "close" {
					closes++
				}
			}
			if attempted {
				if closes != 0 || r.State.Stopped {
					t.Fatal("unknown close was repeated or overstated", closes, r, err)
				}
			} else if err != nil || !r.State.Stopped || closes != 1 {
				t.Fatal("proven unattempted original stop did not finish", closes, r, err)
			}
			if !attempted {
				q.Operation.Receipt, _ = json.Marshal(r)
				before = len(f.calls)
				again, _, replayErr := l.Reconcile(context.Background(), q)
				if replayErr != nil || !again.State.Stopped || len(f.calls) != before {
					t.Fatal("confirmed stop replay repeated a close or lost proof")
				}
			}
			// Missing, corrupt and foreign progress must never authorize a close.
			for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"operation":{"id":"foreign"}}`)} {
				q.Operation.Receipt = bad
				before = len(f.calls)
				r, _, _ = l.Reconcile(context.Background(), q)
				if r.State.Stopped || len(f.calls) != before {
					t.Fatal("unbound stop progress caused an effect")
				}
			}
		})
	}
}

func TestTerminalStopProgressCannotCrossIncarnation(t *testing.T) {
	l, f, s := fixture(t)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	f.failShow = true
	r, _ = l.StopLifecycle(context.Background(), q, nil)
	q.Operation.Receipt, _ = json.Marshal(r)
	f.failShow = false
	f.incarnation = "replacement"
	before := len(f.calls)
	r, _, err = l.Reconcile(context.Background(), q)
	if err == nil || r.State.Stopped {
		t.Fatal("replacement incarnation was accepted")
	}
	for _, c := range f.calls[before:] {
		if c[0] == "terminal" && c[1] == "close" {
			t.Fatal("replacement incarnation was closed")
		}
	}
}
