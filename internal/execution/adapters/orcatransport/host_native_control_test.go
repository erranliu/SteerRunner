package orcatransport

import (
	"context"
	"errors"
	"testing"
)

func TestHostNativeControlClosureWithoutQuery(t *testing.T) {
	for _, early := range []bool{false, true} {
		t.Run(map[bool]string{true: "initialization", false: "ready"}[early], func(t *testing.T) {
			l, _, snapshot, c := fixtureFixedControl(t)
			snapshot = prepareFixedControl(t, l, snapshot, c)
			state, err := l.state(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if early {
				state.RemoteReadiness = nil
				state.Control.Ready = nil
			}
			if err = l.closeControl(context.Background(), state); err != nil {
				t.Fatal(err)
			}
			proof, err := ReadControlClosureEvidence(l.setup.RunDirectory, l.setup.Snapshot, state)
			if err != nil || !proof.Closed || c.closes != 1 || c.sends != 0 {
				t.Fatal("control closure lost or extra query sent", err, c.closes, c.sends)
			}
			if err = l.closeControl(context.Background(), state); err != nil || c.closes != 1 {
				t.Fatal("close replayed", err, c.closes)
			}
		})
	}
}

func TestHostNativeControlClosureLostCloseNeverResends(t *testing.T) {
	l, _, snapshot, c := fixtureFixedControl(t)
	snapshot = prepareFixedControl(t, l, snapshot, c)
	state, _ := l.state(snapshot)
	c.mode = "lost-close"
	_ = l.closeControl(context.Background(), state) // The existing primitive may reconcile the lost receipt by exact inventory.
	c.mode = ""
	if err := l.closeControl(context.Background(), state); err != nil || c.closes != 1 {
		t.Fatal("lost close was replayed", err, c.closes)
	}
}

func TestHostNativeControlClosureRequiresDurableIntent(t *testing.T) {
	l, host, snapshot, c := fixtureFixedControl(t)
	snapshot = prepareFixedControl(t, l, snapshot, c)
	state, _ := l.state(snapshot)
	host.callerFileWriter = func(string, []byte) error { return errors.New("blocked persistence") }
	if err := l.closeControl(context.Background(), state); err == nil || c.closes != 0 {
		t.Fatal("uncommitted close sent", err, c.closes)
	}
}
