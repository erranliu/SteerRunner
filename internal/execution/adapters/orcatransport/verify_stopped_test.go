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
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestCodexStoppedRequiresNativeExitProofBeyondPTY(t *testing.T) {
	for _, name := range []string{"live-root", "missing-root", "no-readiness"} {
		t.Run(name, func(t *testing.T) {
			environment := ""
			l, host, s := fixtureOn(t, environment)
			s = frozenWorkspaceFixture(t, l, host, s)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			current, err := process.CurrentIdentity()
			if err != nil {
				t.Fatal(err)
			}
			if name != "no-readiness" {
				state.RemoteReadiness = &codex.RemoteReadiness{SessionID: "original-session", Process: *current}
			}
			if name == "missing-root" {
				state.RemoteReadiness.Process = b.ProcessIdentity{}
			}
			s.Implementation, err = l.envelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s)
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) != 5 || args[0] != "terminal" || args[1] != "list" {
					t.Fatalf("verification caused an effect: %v", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			err = l.verifyStopped(context.Background(), s)
			if err == nil || !strings.Contains(err.Error(), "PTY exit is insufficient") {
				t.Fatal("native exit was inferred from PTY absence", err)
			}
			if process.Probe(current).State != "matches" {
				t.Fatal("live root fixture exited")
			}
			facts, err := l.ShutdownLifecycle(context.Background(), s, nil)
			if err == nil || facts.Stopped {
				t.Fatal("shutdown overstated native resource settlement", facts, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("read-only probe changed snapshot")
			}
		})
	}
}

func TestLocalCodexStoppedUsesRegisteredResources(t *testing.T) {
	for _, mode := range []string{"complete", "live-business", "live-control", "missing-exit-file", "missing-control", "pending-control"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			// The frozen resources have exited; verification itself only reads inventory.
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) < 2 || args[1] != "list" {
					t.Fatal("verification effect", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			state, _ := l.state(s)
			if mode == "missing-control" {
				state.Control = nil
				s.Implementation, _ = l.envelope(s, state)
			}
			if mode == "pending-control" {
				// An unfinished historical query journal is not stop evidence.
				if e := os.WriteFile(filepath.Join(l.setup.RunDirectory, "orca-control.json"), []byte(`{"pending":true}`), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "live-business" || mode == "live-control" {
				handle, inc := state.Process.Terminal, state.Process.Incarnation
				if mode == "live-control" {
					handle, inc = state.Control.Process.Terminal, state.Control.Process.Incarnation
				}
				l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
					if len(args) < 2 || args[1] != "list" {
						t.Fatal("offline verification effect", args)
					}
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{map[string]any{"handle": handle, "incarnationId": inc, "worktreeId": state.Process.WorktreeID, "status": "running"}}, "totalCount": 1, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": state.Process.RuntimeID}})
				}})
			}
			before, _ := json.Marshal(s)
			err := l.verifyStopped(context.Background(), s)
			if (err == nil) != (mode == "complete" || mode == "missing-exit-file" || mode == "pending-control") {
				t.Fatal(mode, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("readonly verification changed snapshot")
			}
		})
	}
}

func TestHostNativeReadinessRejectsRetiredFullAccessContract(t *testing.T) {
	l, _, s, c := fixtureFixedControl(t)
	s = prepareFixedControl(t, l, s, c)
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	state.RemoteReadiness.Contract = codex.ReadOnlyPolicyContract
	if _, err = l.codexReadiness(state, s.Workspace.Path); err == nil {
		t.Fatal("retired full-access contract admitted")
	}
}
