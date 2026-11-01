package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestLocalCodexStopWithoutProofFiles(t *testing.T) {
	for _, checkpoint := range []string{"missing", "corrupt", "pending"} {
		t.Run(checkpoint, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			state, _ := l.state(s)
			path, binding, _ := l.journalPath(state)
			if checkpoint == "corrupt" {
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
				exit, err := codex.RemoteExitPath(*state.RemoteReadiness)
				if err != nil {
					t.Fatal(err)
				}
				exit = filepath.Join(s.Workspace.Path, filepath.FromSlash(exit))
				if err := os.MkdirAll(filepath.Dir(exit), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(exit, []byte("corrupt signed exit artifact"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if checkpoint == "pending" {
				// A structurally invalid pending query is deliberately not stop proof.
				if err := l.saveControlCheckpoint(state, controlCheckpoint{Version: 3, Binding: binding}); err != nil {
					t.Fatal(err)
				}
			}
			request := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
			r, err := l.StopLifecycle(context.Background(), request, nil)
			if err != nil || !r.State.Stopped || r.State.WritersAbsent || l.StopEvidenceScope() != LocalCodexStopScope {
				t.Fatal(r, err)
			}
			before := len(f.calls)
			if err := l.verifyStopped(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			if len(f.calls) != before || c.sends != 0 || c.closes != 1 {
				t.Fatal("verification sent input or control close repeated", c.sends, c.closes)
			}
			if _, err := l.callControl(context.Background(), state, "original-turn", ""); err == nil {
				t.Fatal("new control query admitted after stop")
			}
			if c.sends != 0 {
				t.Fatal("stopped control dispatched a query")
			}
		})
	}
}

func TestLocalCodexStopCooperationAndLostClose(t *testing.T) {
	l, f, s, c := fixtureFixedControl(t)
	s = prepareFixedControl(t, l, s, c)
	live, uploads := true, 0
	l.probeProcess = func(*b.ProcessIdentity) b.ProcessProbeFacts {
		if live {
			return b.ProcessProbeFacts{State: "matches"}
		}
		return b.ProcessProbeFacts{State: "absent"}
	}
	l.uploadStopArtifact = func(_ context.Context, environment, runtime, worktree, path string, body io.Reader) error {
		uploads++
		if environment != "" || runtime != s.Ref.External.RuntimeID || worktree != "id:"+s.Ref.External.WorktreeID {
			t.Fatal("stop request target changed")
		}
		live = false
		return errors.New("request response lost")
	}
	c.mode = "lost-close"
	q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	r, err := l.StopLifecycle(context.Background(), q, nil)
	if err != nil || !r.State.Stopped || uploads != 1 || c.closes != 1 {
		t.Fatal("lost response not reconciled by facts", r, err, uploads, c.closes)
	}
	count := len(f.calls)
	for i := 0; i < 2; i++ {
		r, _, err = l.Reconcile(context.Background(), q)
		if err != nil || !r.State.Stopped {
			t.Fatal(r, err)
		}
	}
	if uploads != 1 || c.closes != 1 || len(f.calls) != count {
		t.Fatal("reconciliation repeated an effect", uploads, c.closes)
	}
}

func TestLocalCodexStopReconcileNeverClosesLiveResources(t *testing.T) {
	l, f, s, c := fixtureFixedControl(t)
	s = prepareFixedControl(t, l, s, c)
	q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	before := len(f.calls)
	r, _, err := l.Reconcile(context.Background(), q)
	if err == nil || r.State.Stopped || c.closes != 0 || c.sends != 0 {
		t.Fatal("reconcile executed cleanup", r, err)
	}
	for _, call := range f.calls[before:] {
		if len(call) > 1 && call[1] != "list" {
			t.Fatal("reconcile caused an effect", call)
		}
	}
}

func TestLocalCodexStoppedRootsAndInventory(t *testing.T) {
	for _, mode := range []string{"helper-live", "root-live", "pid-reused", "incomplete", "foreign-incarnation", "missing-ready", "unknown-create"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			state, _ := l.state(s)
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) < 2 || args[1] != "list" {
					t.Fatal("verification effect", args)
				}
				v := map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": mode == "incomplete", "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}
				if mode == "foreign-incarnation" {
					v["terminals"] = []any{map[string]any{"handle": state.Process.Terminal, "incarnationId": "replacement", "worktreeId": state.Process.WorktreeID}}
					v["totalCount"] = 1
				}
				return json.Marshal(map[string]any{"ok": true, "result": v, "_meta": map[string]string{"runtimeId": state.Process.RuntimeID}})
			}})
			l.probeProcess = func(p *b.ProcessIdentity) b.ProcessProbeFacts {
				if mode == "pid-reused" {
					return b.ProcessProbeFacts{State: "pid-reused"}
				}
				if mode == "helper-live" && p.PID == state.RemoteReadiness.Owner.PID || mode == "root-live" && p.PID == state.RemoteReadiness.Process.PID {
					return b.ProcessProbeFacts{State: "matches"}
				}
				return b.ProcessProbeFacts{State: "absent"}
			}
			if mode == "missing-ready" {
				state.RemoteReadiness = nil
				state.LaunchAttempted = true
				state.LaunchDigest = digest([]byte(l.setup.AgentLaunchCommand))
				state.LaunchRequest = nativeRequest("original")
				s.Implementation, _ = l.envelope(s, state)
			}
			if mode == "unknown-create" {
				s.Operations = append(s.Operations, b.Operation{Action: string(b.LifecycleCreateResource), Status: b.OperationUnknown})
			}
			if err := l.verifyStopped(context.Background(), s); err == nil {
				t.Fatal("missing scoped evidence accepted", mode)
			}
		})
	}
}

func TestLocalCodexStopDoesNotRequireBusinessProofs(t *testing.T) {
	for _, mode := range []string{"reader-evidence", "control-not-ready", "business-conflict"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			state, _ := l.state(s)
			state.Turns = []Turn{{Recorded: &codex.RemoteTurnReceipt{ReaderWritersAbsent: false}}}
			if mode == "control-not-ready" {
				state.Turns[0].Recorded = nil
				state.Control.Ready = nil
			}
			if mode == "business-conflict" {
				state.Turns[0].Native.Conflict = "unverified input"
				state.Turns[0].Native.Waiting = "pending observation"
			}
			s.Implementation, _ = l.envelope(s, state)
			if _, err := l.state(s); err == nil {
				t.Fatal("business proof must remain rejected")
			}
			r, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
			if err != nil || !r.State.Stopped || r.State.WritersAbsent || c.sends != 0 || c.closes != 1 {
				t.Fatal("business proof still gates original resource stop", r, err)
			}
			facts, err := l.ShutdownLifecycle(context.Background(), s, nil)
			if err != nil || !facts.Stopped {
				t.Fatal("business proof migrated to shutdown", err)
			}
			if _, err := l.state(s); err == nil {
				t.Fatal("stop promoted invalid business proof")
			}
		})
	}
}

func TestLocalCodexStopAcceptsEquivalentLocalWorkspacePath(t *testing.T) {
	l, _, s, c := fixtureFixedControl(t)
	s = prepareFixedControl(t, l, s, c)
	original := l.client
	l.client = native.New(native.Config{Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
		raw, err := original.Raw(ctx, d, args...)
		if err == nil && len(args) > 1 && args[1] == "show" {
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			terminal := value["result"].(map[string]any)["terminal"].(map[string]any)
			terminal["worktreePath"] = strings.ToUpper(strings.ReplaceAll(s.Workspace.Path, "\\", "/"))
			return json.Marshal(value)
		}
		return raw, err
	}})
	r, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
	if err != nil || !r.State.Stopped {
		t.Fatal("equivalent local workspace path blocked stop", err)
	}
}
