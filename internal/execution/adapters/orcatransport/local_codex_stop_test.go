package orcatransport

import (
	"context"
	"encoding/json"

	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

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
				state.LaunchRequest = "0123456789abcdef0123456789abcdef"
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
