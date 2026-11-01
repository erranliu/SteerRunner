package orcatransport

import (
	"os"
	"testing"
)

// Query checkpoint integrity remains required for normal history/audit reads.
func TestHostNativeControlCheckpointSurvivesPreparedSnapshot(t *testing.T) {
	l, _, snapshot, c := fixtureFixedControl(t)
	snapshot = prepareFixedControl(t, l, snapshot, c)
	state, _ := l.state(snapshot)
	path, binding, err := l.journalPath(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.saveControlCheckpoint(state, controlCheckpoint{Version: 3, Binding: binding}); err != nil {
		t.Fatal(err)
	}
	l.setup.Snapshot = snapshot
	if _, err := l.loadControlCheckpoint(state); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":3,"binding":"foreign"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.loadControlCheckpoint(state); err == nil {
		t.Fatal("foreign query checkpoint accepted")
	}
}
