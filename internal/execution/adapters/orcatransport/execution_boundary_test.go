package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestRetiredExecutionFactoryKeepsFrozenProof(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		for _, mode := range []string{"absent", "live", "incomplete", "foreign-runtime", "missing-host"} {
			t.Run(environment+"/"+mode, func(t *testing.T) {
				l, host, s := sharedNativeFixture(t, environment)
				s = frozenWorkspaceFixture(t, l, host, s)
				setup := l.setup.Clone()
				setup.Snapshot = s
				calls := 0
				setup.ProcessHost = &creationTestHost{ManagedProcessHost: host, output: func(_ context.Context, c b.LaunchCommand) ([]byte, error) {
					calls++
					if len(c.Args) < 2 || c.Args[0] != "terminal" || c.Args[1] != "list" {
						t.Fatal("frozen proof dispatched an effect", c.Args)
					}
					rows := []any{}
					if mode == "live" {
						rows = append(rows, map[string]any{"handle": s.Ref.External.Terminal, "incarnationId": s.Ref.External.Incarnation, "worktreeId": s.Ref.External.WorktreeID, "status": "running"})
					}
					runtime := "runtime"
					if mode == "foreign-runtime" {
						runtime = "replacement"
					}
					hosts := []string{"local"}
					if mode == "missing-host" {
						hosts = nil
					}
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": rows, "totalCount": len(rows), "truncated": mode == "incomplete", "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": runtime}})
				}}
				before, _ := json.Marshal(setup)
				port, err := NewLifecycle(setup)
				var failure *b.BackendError
				if port != nil || !errors.As(err, &failure) || failure.Code != b.BackendUnsupported || calls != 0 {
					t.Fatal("retired execution was constructed", port, err, calls)
				}
				scope, err := StopEvidenceScope(setup)
				if err != nil || scope != "orca_terminal" || calls != 0 {
					t.Fatal("scope activated execution", scope, err, calls)
				}
				err = VerifyStopped(context.Background(), setup)
				if (err == nil) != (mode == "absent") || calls != 1 {
					t.Fatal("frozen proof lost original scope", mode, calls, err)
				}
				after, _ := json.Marshal(setup)
				if string(before) != string(after) {
					t.Fatal("proof modified saved facts")
				}
			})
		}
	}
}
