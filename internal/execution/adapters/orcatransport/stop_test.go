package orcatransport

import (
	"context"
	"encoding/json"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalStopResumesOnlyOriginalReaderCleanup(t *testing.T) {
	for _, known := range []bool{true, false} {
		t.Run(map[bool]string{true: "known-reader", false: "unknown-create"}[known], func(t *testing.T) {
			l, f, s := fixture(t)
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			state.RemoteReadiness = &codex.RemoteReadiness{SessionID: "original-session"}
			turn := Turn{Command: strings.Repeat("c", 32), WireFormat: remoteWireCommand, Prompt: "report", Attempted: true, Accepted: true}
			turn.InputDigest = digest([]byte(turn.Prompt))
			turn.Native.Pane = "original-pane"
			state.Turns = []Turn{turn}
			s.Implementation, _ = l.envelope(s, state)
			path, binding, err := l.journalPath(state)
			if err != nil {
				t.Fatal(err)
			}
			entry := inspectionEntry{ID: strings.Repeat("d", 32), Command: turn.Command, InputDigest: digest([]byte(wirePrompt(&turn))), Inspection: NativeInspection{Attempted: true, Unknown: true, At: time.Now().UTC()}}
			if known {
				entry.Inspection.Process = &b.ProcessIdentity{Kind: "orca_original_turn_reader", RuntimeID: state.Process.RuntimeID, Terminal: "inspector", Incarnation: "inspector-inc", WorktreeID: state.Process.WorktreeID}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(inspectionJournal{Version: 1, Binding: binding, Entries: []inspectionEntry{entry}})
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			f.callerFileWriter = func(path string, raw []byte) error { return os.WriteFile(path, raw, 0600) }
			original := l.client
			readerCloses := 0
			l.client = native.New(native.Config{Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
				if !strings.Contains(strings.Join(args, " "), "inspector") {
					return original.Raw(ctx, d, args...)
				}
				var result any
				if args[1] == "show" {
					result = map[string]any{"terminal": map[string]any{"handle": "inspector", "incarnationId": "inspector-inc", "worktreeId": state.Process.WorktreeID, "connected": true, "status": "running"}}
				} else if args[1] == "close" {
					readerCloses++
					result = map[string]any{"close": map[string]any{"handle": "inspector", "ptyKilled": true}}
				} else {
					t.Fatal(args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": state.Process.RuntimeID}})
			}})
			q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
			progress, _ := json.Marshal(stopProgress{Version: 1, Phase: "business-confirmed"})
			saved := b.LifecycleReceipt{Operation: q.Operation, Disposition: b.OperationUnknown, State: b.LifecycleStateFacts{WorkerAbsent: true}}
			saved.Operation.Receipt = progress
			q.Operation.Receipt, _ = json.Marshal(saved)
			before := len(f.calls)
			got, _, err := l.Reconcile(context.Background(), q)
			if len(f.calls) != before {
				t.Fatal("confirmed business close was repeated")
			}
			if known {
				if err != nil || !got.State.Stopped || readerCloses != 1 {
					t.Fatal(got, err, readerCloses)
				}
			} else if err == nil || got.State.Stopped || readerCloses != 0 {
				t.Fatal("unknown creation was hidden", got, err)
			}
		})
	}
}

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
