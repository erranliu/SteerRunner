package orcatransport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestTerminalOfflineStopIncludesOriginalReaders(t *testing.T) {
	for _, name := range []string{"absent", "active", "reader-active", "reader-absent", "reader-unknown", "reader-missing-incarnation", "foreign-journal", "missing-binding", "generic-without-exit"} {
		t.Run(name, func(t *testing.T) {
			l, _, s := fixture(t)
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			var journalPath string
			if strings.HasPrefix(name, "reader-") || name == "foreign-journal" {
				state.RemoteReadiness = &codex.RemoteReadiness{SessionID: "original-session"}
				turn := Turn{Command: strings.Repeat("c", 32), WireFormat: remoteWireCommand, Prompt: "report", Attempted: true, Accepted: true}
				turn.InputDigest = digest([]byte(turn.Prompt))
				turn.Native.Pane = "pane"
				state.Turns = []Turn{turn}
				path, binding, e := l.journalPath(state)
				if e != nil {
					t.Fatal(e)
				}
				journalPath = path
				entry := inspectionEntry{ID: strings.Repeat("d", 32), Command: turn.Command, InputDigest: digest([]byte(wirePrompt(&turn))), Inspection: NativeInspection{Attempted: true, Unknown: true, At: time.Now().UTC()}}
				if name != "reader-unknown" {
					entry.Inspection.Process = &b.ProcessIdentity{Kind: "orca_original_turn_reader", RuntimeID: state.Process.RuntimeID, Terminal: "reader", Incarnation: "reader-inc", WorktreeID: state.Process.WorktreeID}
				}
				if name == "reader-missing-incarnation" {
					copy := entry.Inspection
					copy.Closed, copy.Unknown = true, false
					p := *copy.Process
					p.Incarnation = ""
					copy.Process = &p
					state.Turns[0].Inspection = &copy
				}
				if name == "foreign-journal" {
					binding = "foreign"
				}
				raw, _ := json.Marshal(inspectionJournal{Version: 1, Binding: binding, Entries: []inspectionEntry{entry}})
				if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if name == "missing-binding" {
				state.Process = b.ProcessIdentity{}
			}
			if name == "generic-without-exit" {
				l.profile.Terminal = &TerminalConfig{}
			}
			s.Implementation, err = l.envelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s)
			var journalBefore []byte
			if journalPath != "" {
				journalBefore, _ = os.ReadFile(journalPath)
			}
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) != 5 || args[0] != "terminal" || args[1] != "list" {
					t.Fatalf("offline query caused an effect: %v", args)
				}
				terms := []native.Terminal{}
				if name == "active" {
					terms = append(terms, native.Terminal{Handle: state.Process.Terminal, IncarnationID: state.Process.Incarnation, WorktreeID: state.Process.WorktreeID, Status: "running"})
				}
				if name == "reader-active" {
					terms = append(terms, native.Terminal{Handle: "reader", IncarnationID: "reader-inc", WorktreeID: state.Process.WorktreeID, Status: "running"})
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": terms, "truncated": false, "totalCount": len(terms), "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			err = l.verifyStopped(context.Background(), s)
			pass := name == "absent" || name == "reader-absent"
			if (err == nil) != pass {
				t.Fatalf("%s: %v", name, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("offline verification changed snapshot")
			}
			if journalPath != "" {
				after, _ := os.ReadFile(journalPath)
				if string(after) != string(journalBefore) {
					t.Fatal("offline verification changed journal")
				}
			}
			// Shutdown may detach an active execution, but it must never claim it stopped.
			if name == "active" || name == "absent" {
				facts, e := l.ShutdownLifecycle(context.Background(), s, nil)
				if (e == nil) != pass || facts.Stopped != pass {
					t.Fatalf("shutdown overstated facts: %+v %v", facts, e)
				}
			}
		})
	}
}
