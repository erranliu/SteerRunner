package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestCodexStoppedRequiresNativeExitProofBeyondPTY(t *testing.T) {
	for _, name := range []string{"live-root", "missing-root", "no-readiness", "remote-live-root"} {
		t.Run(name, func(t *testing.T) {
			environment := ""
			if name == "remote-live-root" {
				environment = "paired-host"
			}
			l, _, s := fixtureOn(t, environment)
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, receipt)
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

func TestCodexStoppedRequiresSignedExitAndFixedControl(t *testing.T) {
	for _, mode := range []string{"complete", "live-business", "live-control", "changed-signature", "missing-control", "pending-control"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			r, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
			if err != nil || !r.State.Stopped {
				t.Fatal(r, err)
			}
			state, _ := l.state(s)
			if mode == "missing-control" {
				state.Control = nil
				s.Implementation, _ = l.envelope(s, state)
			}
			if mode == "pending-control" {
				j, e := l.loadControlCheckpoint(state)
				if e != nil {
					t.Fatal(e)
				}
				j.Response = nil
				if e = l.saveControlCheckpoint(state, j); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "changed-signature" {
				original := l.readStopArtifact
				l.readStopArtifact = func(ctx context.Context, a, b, c, d string) ([]byte, error) {
					raw, e := original(ctx, a, b, c, d)
					var v map[string]json.RawMessage
					_ = json.Unmarshal(raw, &v)
					v["body"] = json.RawMessage(`{}`)
					changed, _ := json.Marshal(v)
					return changed, e
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
			err = l.verifyStopped(context.Background(), s)
			if (err == nil) != (mode == "complete") {
				t.Fatal(mode, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("readonly verification changed snapshot")
			}
		})
	}
}
