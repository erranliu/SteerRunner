package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
)

type inspectionEntry struct {
	ID          string           `json:"id"`
	Command     string           `json:"command"`
	InputDigest string           `json:"input_digest"`
	Inspection  NativeInspection `json:"inspection"`
}
type inspectionJournal struct {
	Version int               `json:"version"`
	Binding string            `json:"binding"`
	Entries []inspectionEntry `json:"entries"`
}

// These are caller-owned implementation checkpoints, not remote filesystem or
// PID observations. Intent precedes create; its returned identity precedes any
// read/close. A killed observer therefore cannot lose a known auxiliary owner.
func (l *Lifecycle) journalPath(state State) (string, string, error) {
	if !filepath.IsAbs(l.setup.RunDirectory) || state.RemoteReadiness == nil {
		return "", "", errors.New("caller readonly checkpoint directory or binding missing")
	}
	raw, _ := json.Marshal(struct {
		Project, Execution, Attempt, Generation, Config string
		Process                                         b.ProcessIdentity
		Ready                                           any
	}{l.setup.Snapshot.Session.ProjectID, l.setup.Snapshot.Session.ExecutionID, l.setup.Snapshot.Session.AttemptID, l.setup.Snapshot.ResourceGeneration, l.setup.Snapshot.ConfigurationDigest, state.Process, state.RemoteReadiness})
	binding := digest(raw)
	return filepath.Join(l.setup.RunDirectory, "orca-native-inspections", binding+".json"), binding, nil
}
func (l *Lifecycle) loadInspections(state State) (inspectionJournal, error) {
	path, binding, e := l.journalPath(state)
	if e != nil {
		return inspectionJournal{}, e
	}
	j := inspectionJournal{Version: 1, Binding: binding}
	raw, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return j, nil
	}
	if e != nil {
		return j, e
	}
	if len(raw) > 1<<20 || b.DecodeStrictJSON(raw, &j, "trailing readonly checkpoint") != nil || j.Version != 1 || j.Binding != binding || len(j.Entries) > 512 {
		return j, errors.New("caller readonly checkpoint invalid or foreign")
	}
	seen := map[string]bool{}
	for _, entry := range j.Entries {
		t := turnFor(&state, entry.Command)
		a := entry.Inspection
		if !b.StableExecutionID(entry.ID) || seen[entry.ID] || t == nil || t.WireFormat != remoteWireCommand || entry.InputDigest != digest([]byte(wirePrompt(t))) || !a.Attempted || a.At.IsZero() || a.Closed && (a.Unknown || a.Process == nil) {
			return j, errors.New("readonly checkpoint input binding changed")
		}
		seen[entry.ID] = true
		if p := a.Process; p != nil && (p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == "" || p.Terminal == state.Process.Terminal || p.Incarnation == "" || p.Kind != "orca_original_turn_reader") {
			return j, errors.New("readonly checkpoint resource identity changed")
		}
	}
	return j, nil
}
func (l *Lifecycle) saveInspections(state State, j inspectionJournal) error {
	path, binding, e := l.journalPath(state)
	if e != nil {
		return e
	}
	if j.Binding != binding || j.Version != 1 {
		return errors.New("readonly checkpoint binding changed")
	}
	raw, e := json.Marshal(j)
	if e != nil {
		return e
	}
	return l.setup.ProcessHost.WriteExecutionFile(path, raw)
}
func (l *Lifecycle) inspectionGate(ctx context.Context) (func(), error) {
	l.inspectionOnce.Do(func() { l.inspectionSerial = make(chan struct{}, 1) })
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	select {
	case l.inspectionSerial <- struct{}{}:
		return func() { <-l.inspectionSerial }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// The caller must hold inspectionGate. Recovery closes/reconciles only the
// saved original readers, never a business terminal or a replacement reader.
func (l *Lifecycle) settleInspections(ctx context.Context, state *State) (inspectionJournal, error) {
	j, e := l.loadInspections(*state)
	if e != nil {
		return j, e
	}
	for i := range j.Entries {
		a := &j.Entries[i].Inspection
		if a.Closed {
			continue
		}
		if a.Process == nil {
			return j, errors.New("original readonly creation outcome unknown; no replacement created")
		}
		proof, ce := l.client.CloseReadonlyTerminal(ctx, *a.Process)
		if ce != nil || proof.Disposition != "confirmed" {
			return j, errors.Join(ce, errors.New("original readonly cleanup remains unverified"))
		}
		a.Closed, a.Unknown = true, false
		if e = l.saveInspections(*state, j); e != nil {
			return j, e
		}
	}
	for i := range state.Turns {
		a := state.Turns[i].Inspection
		if a == nil || a.Process == nil {
			continue
		}
		for _, entry := range j.Entries {
			if entry.Inspection.Closed && entry.Inspection.Process != nil && *entry.Inspection.Process == *a.Process {
				a.Closed, a.Unknown = true, false
			}
		}
	}
	return j, nil
}

func (l *Lifecycle) cleanupInspections(ctx context.Context, state *State) error {
	if state.RemoteReadiness == nil {
		return nil
	}
	release, e := l.inspectionGate(ctx)
	if e != nil {
		return e
	}
	defer release()
	_, e = l.settleInspections(ctx, state)
	if e != nil {
		return e
	}
	for _, turn := range state.Turns {
		if turn.Inspection != nil && !turn.Inspection.Closed {
			return errors.New("original native inspector cleanup remains unverified")
		}
	}
	return nil
}
