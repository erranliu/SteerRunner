package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitialTerminalRequiresEmptyInventoryAndSurvivesCallerRestart(t *testing.T) {
	for _, mode := range []string{"success", "unknown-create", "foreign-runtime", "incomplete", "changed-incarnation", "corrupt-checkpoint", "multiple-after-create", "planned-recovery", "local-omits-remote", "omits-local", "omits-original-runtime", "wrong-worktree"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s := fixture(t)
			l.setup.AgentLaunchCommand, f.commandMode = "opaque-start", true
			f.callerFileWriter = func(path string, data []byte) error {
				if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
					return e
				}
				return os.WriteFile(path, data, 0600)
			}
			r, e := l.client.MetaCall(context.Background(), "worktree", "create", "--name", "sl-test", "--base-branch", s.WorkspacePlan.BaseCommit, "--json")
			if e != nil {
				t.Fatal(e)
			}
			var c workspaceCreation
			if json.Unmarshal(r.Payload, &c) != nil {
				t.Fatal("invalid fixture")
			}
			c.RuntimeID = r.RuntimeID
			c.AgentTerminalHandle = ""
			original := l.client
			creates := 0
			created := false
			l.client = native.New(native.Config{Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
				if args[0] == "terminal" && args[1] == "create" {
					creates++
					if mode == "unknown-create" {
						return nil, errors.New("lost creation receipt")
					}
					created = true
					return original.Raw(ctx, d, "terminal", "show", "--terminal", "terminal", "--json")
				}
				if args[0] == "terminal" && args[1] == "list" && (!created || mode == "multiple-after-create") {
					runtime := "runtime"
					if mode == "foreign-runtime" {
						runtime = "foreign"
					}
					value := map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"host"}, "omittedHostIds": []string{}}}
					if mode == "incomplete" {
						delete(value, "hostScope")
					}
					if mode == "local-omits-remote" || mode == "omits-local" || mode == "omits-original-runtime" {
						omitted := "runtime:another-paired-runtime"
						if mode == "omits-local" {
							omitted = "local"
						}
						if mode == "omits-original-runtime" {
							omitted = "runtime:" + c.RuntimeID
						}
						value["hostScope"] = map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{omitted}}
					}
					if created {
						value["terminals"] = []any{map[string]any{"handle": "terminal"}, map[string]any{"handle": "other"}}
						value["totalCount"] = 2
					}
					return json.Marshal(map[string]any{"ok": true, "result": value, "_meta": map[string]any{"runtimeId": runtime}})
				}
				return original.Raw(ctx, d, args...)
			}})
			budget := 3 * time.Second
			if mode == "incomplete" || mode == "foreign-runtime" || mode == "omits-local" || mode == "omits-original-runtime" {
				budget = 30 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			if mode == "planned-recovery" {
				if _, _, err := l.initialTerminalPlan(s, true); err != nil {
					t.Fatal(err)
				}
			}
			got, e := l.initialWorkspaceTerminalRecovery(ctx, s, c, mode == "planned-recovery")
			cancel()
			successful := mode == "success" || mode == "changed-incarnation" || mode == "corrupt-checkpoint" || mode == "planned-recovery" || mode == "local-omits-remote" || mode == "wrong-worktree"
			if successful && (e != nil || got.Handle != "terminal" || creates != 1) {
				t.Fatal("initial terminal not bound", got, e, creates)
			}
			if !successful && e == nil {
				t.Fatal("unverified initial terminal admitted")
			}
			if mode == "incomplete" || mode == "foreign-runtime" || mode == "omits-local" || mode == "omits-original-runtime" {
				if creates != 0 {
					t.Fatal("incomplete or foreign inventory authorized create")
				}
				return
			}
			if mode == "changed-incarnation" {
				f.incarnation = "replacement"
			}
			if mode == "wrong-worktree" {
				c.Worktree.InstanceID = "replacement-worktree"
			}
			if mode == "corrupt-checkpoint" {
				files, _ := filepath.Glob(filepath.Join(l.setup.RunDirectory, "orca-initial-terminals", "*.json"))
				raw, _ := os.ReadFile(files[0])
				var j initialTerminalCheckpoint
				_ = json.Unmarshal(raw, &j)
				j.Binding = "foreign"
				raw, _ = json.Marshal(j)
				_ = os.WriteFile(files[0], raw, 0600)
			}
			restoredPort, e := NewLifecycleWithStateFactory(l.setup, f.stateFactory)
			if e != nil {
				t.Fatal(e)
			}
			restored := restoredPort.(*Lifecycle)
			restored.client = l.client
			before := len(f.calls)
			_, e = restored.initialWorkspaceTerminal(context.Background(), s, c)
			wantSuccess := mode == "success" || mode == "planned-recovery" || mode == "local-omits-remote"
			if creates != 1 || wantSuccess && e != nil || !wantSuccess && e == nil {
				t.Fatal("caller restart recreated or changed original terminal", e, creates)
			}
			for _, call := range f.calls[before:] {
				if call[1] != "show" && call[1] != "list" {
					t.Fatal("restart performed a mutation", call)
				}
			}
			if f.nativeSends != 0 || f.launched {
				t.Fatal("workspace preparation launched model input")
			}
		})
	}
}
