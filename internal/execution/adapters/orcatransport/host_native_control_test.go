package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func TestHostNativeControlCheckpointSurvivesPreparedSnapshot(t *testing.T) {
	l, _, snapshot, c := fixtureFixedControl(t)
	snapshot = prepareFixedControl(t, l, snapshot, c)
	state, _ := l.state(snapshot)
	if err := l.closeControl(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	// The production port is cached from before prepare_workspace. A fresh
	// service or read-only verifier receives the final prepared snapshot.
	if _, err := ReadControlClosureEvidence(l.setup.RunDirectory, snapshot, state); err != nil {
		t.Fatal("prepared snapshot lost original closure", err)
	}
	if c.closes != 1 {
		t.Fatal("read-only verification repeated close")
	}
}

func TestHostNativeControlRejectsV6EmptyWorkspaceCheckpoint(t *testing.T) {
	l, _, snapshot, c := fixtureFixedControl(t)
	snapshot = prepareFixedControl(t, l, snapshot, c)
	state, _ := l.state(snapshot)
	oldPath, oldBinding, err := l.journalPathForWorkspace(state, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(oldPath), 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(controlCheckpoint{Version: 3, Binding: oldBinding, Closing: true, Closed: true})
	if err = os.WriteFile(oldPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadControlClosureEvidence(l.setup.RunDirectory, snapshot, state); err == nil {
		t.Fatal("retired v6 checkpoint admitted")
	}
	// A present canonical checkpoint is authoritative; a foreign or corrupt
	// one cannot be masked by falling back to the older filename.
	path, _, _ := l.journalPath(state)
	if path == oldPath {
		t.Fatal("fixture did not exercise distinct prepared path")
	}
	if err = os.WriteFile(path, []byte(`{"version":3,"binding":"foreign","closed":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadControlClosureEvidence(l.setup.RunDirectory, snapshot, state); err == nil {
		t.Fatal("foreign checkpoint hidden by old closure")
	}
	if c.closes != 0 || c.sends != 0 {
		t.Fatal("closure reader performed an effect")
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
