//go:build windows

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Real service/CLI/SQLite with the existing isolated native wire fixture. Its
// private cache has no real Orca files and never locks a production cache.
func TestSROrcaCLIWaitRetry(t *testing.T) {
	testCount.Add(1)
	for _, mode := range []string{"release-after-15s", "graceful-restart", "kill-restart", "stop-queued", "transient-backoff", "restart-stop-queued"} {
		t.Run(mode, func(t *testing.T) {
			dir, req := setupRun(t)
			state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
			runtime := "wait-runtime-" + filepath.Base(state)
			t.Setenv("STEERRUNNER_CREATION_RUNTIME", runtime)
			creationObservationFixture(t, state, runtime)
			var input contracts.RunRequest
			_ = json.Unmarshal(req.Input, &input)
			settings, _ := json.Marshal(map[string]any{"access_mode": "full_access", "transport": map[string]any{"executable": helperEXE, "args": []string{"--orca-create-fixture"}, "wait_timeout": "15s"}})
			input.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: settings}
			req.Input, _ = json.Marshal(input)
			service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, service)
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
			defer cancel()
			locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			go func() {
				_, err := native.SerializeCLI(ctx, func(context.Context) ([]byte, error) {
					close(locked)
					select {
					case <-release:
					case <-ctx.Done():
					}
					return nil, nil
				})
				done <- err
			}()
			<-locked
			held := true
			t.Cleanup(func() {
				if held {
					close(release)
				}
				<-done
			})
			if mode == "transient-backoff" {
				if err := os.WriteFile(filepath.Join(state, "query-transient"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			raw, _ := json.Marshal(req)
			accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			id := accepted.Data.(map[string]any)["run_id"].(string)
			before := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.ExecutionWait != nil && v.ExecutionWait.Stage == "queue" })
			q := *before.ExecutionWait
			defer func() {
				latest, _, _, _ := invoke(runnerEXE, []string{"observe", "--run-id", id, "--state-dir", dir, "--json"}, "")
				record, _ := json.MarshalIndent(map[string]any{"mode": mode, "queue": before, "final_observation": latest}, "", "  ")
				if *evidenceFlag != "" {
					if err := os.WriteFile(filepath.Join(*evidenceFlag, "cli-wait-"+mode+".json"), record, 0600); err != nil {
						t.Error(err)
					}
				}
			}()

			if q.CallsStarted != 0 || before.ResumeRequired || !before.NewInputsAllowed || len(before.Operations) != 1 || q.OperationID != before.Operations[0].ID {
				t.Fatal("invalid queue snapshot", before)
			}
			for i := 0; i < 2; i++ {
				v := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.ExecutionWait != nil })
				if v.Revision != before.Revision || !v.ExecutionWait.UpdatedAt.Equal(q.UpdatedAt) {
					t.Fatal("GET changed durable wait")
				}
			}
			if mode == "graceful-restart" || mode == "kill-restart" || mode == "restart-stop-queued" {
				if mode == "kill-restart" {
					if err := killOwnedService(service); err != nil {
						t.Fatal(err)
					}
				} else {
					call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("detach-wait", service.InstanceID), 0)
				}
				if err := waitOwnedService(service); err != nil {
					t.Fatal(err)
				}
				reader, err := storage.OpenReader(dir)
				if err != nil {
					t.Fatal(err)
				}
				rows, err := reader.Runs()
				reader.Close()
				if err != nil || len(rows) != 1 {
					t.Fatal(err)
				}
				saved := rows[0].Intents[0].PreparationQuery
				if saved == nil || saved.Wait.OperationID != q.OperationID || !saved.Wait.DeadlineAt.Equal(q.DeadlineAt) || !rows[0].Allowed {
					t.Fatal("shutdown reset budget/authorization")
				}
				if mode == "restart-stop-queued" {
					pending := contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: id, Request: contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-original-query-before-workers", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, rows[0].Revision))}}}}
					service = serviceInfo(t, call(t, dir, []string{"service", "start", "--input", "-"}, string(pending.JSON()), 0))
				} else {
					service = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				}
				cleanupService(t, dir, service)
				if mode == "restart-stop-queued" {
					stopped := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
					if stopped.ExecutionWait != nil || stopped.NewInputsAllowed || stopped.Execution.Workspace != "" {
						t.Fatal("restart admitted a canceled query", stopped)
					}
					close(release)
					held = false
					call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("finish-queued-revocation", service.InstanceID), 0)
					if err := waitOwnedService(service); err != nil {
						t.Fatal(err)
					}
					reader, err := storage.OpenReader(dir)
					if err != nil {
						t.Fatal(err)
					}
					rows, err := reader.Runs()
					reader.Close()
					if err != nil || len(rows) != 1 {
						t.Fatal(err)
					}
					q2 := rows[0].Intents[0].PreparationQuery
					if q2.Wait.CallsStarted != 0 || !q2.Wait.DeadlineAt.Equal(q.DeadlineAt) || q2.Wait.OperationID != q.OperationID || rows[0].Intents[0].Request.Operation.Status != "rejected" {
						t.Fatal("revoked restart changed budget or left unknown creation", q2)
					}
					if _, err := os.Stat(filepath.Join(state, "worktree-created.json")); !os.IsNotExist(err) {
						t.Fatal("revoked restart created a workspace")
					}
					return
				}
				after := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.ExecutionWait != nil && v.ExecutionWait.Stage == "queue" })
				if after.ExecutionWait.OperationID != q.OperationID || !after.ExecutionWait.DeadlineAt.Equal(q.DeadlineAt) || after.ExecutionWait.CallsStarted != 0 {
					t.Fatal("restart renewed or replaced query", after)
				}
			}
			stop := func() {
				for tries := 0; tries < 8; tries++ {
					v := waitRun(t, dir, id, func(contracts.RunView) bool { return true })
					r := contracts.Request{Protocol: contracts.Protocol, RequestID: fmt.Sprintf("stop-wait-%s-%d", id, v.Revision), Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, v.Revision))}
					body, _ := json.Marshal(r)
					response, _, _, err := invoke(runnerEXE, []string{"stop", "--run-id", id, "--state-dir", dir, "--input", "-", "--json"}, string(body))
					if err == nil && response.OK {
						return
					}
					if response.Error == nil || response.Error.Code != "stale_revision" {
						t.Fatal("stop failed", response, err)
					}
				}
				t.Fatal("stop never acquired current revision")
			}
			if mode == "stop-queued" {
				stop()
				stopped := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
				if stopped.ExecutionWait != nil || stopped.NewInputsAllowed {
					t.Fatal("stop retained wait/authorization")
				}
				close(release)
				held = false
				time.Sleep(150 * time.Millisecond)
				if _, err := os.Stat(filepath.Join(state, "worktree-created.json")); !os.IsNotExist(err) {
					t.Fatal("stop allowed creation")
				}
				return
			}
			if mode == "release-after-15s" {
				time.Sleep(16 * time.Second)
				still := waitRun(t, dir, id, func(contracts.RunView) bool { return true })
				if still.ExecutionWait == nil || still.ExecutionWait.Stage != "queue" || still.ExecutionWait.CallsStarted != 0 || still.ResumeRequired {
					t.Fatal("old 15s queue cutoff remains", still)
				}
			}
			close(release)
			held = false
			if mode == "transient-backoff" {
				backoff := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.ExecutionWait != nil && v.ExecutionWait.Stage == "backoff" })
				if backoff.ExecutionWait.CallsStarted != 1 || backoff.ExecutionWait.NextAttemptAt == nil || backoff.ResumeRequired {
					t.Fatal("retry not bounded/automatic", backoff)
				}
				if err := os.Remove(filepath.Join(state, "query-transient")); err != nil {
					t.Fatal(err)
				}
			}
			after := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.Workspace != "" })
			if after.ExecutionWait != nil || after.Operations[0].ID != q.OperationID || after.Execution.AttemptID != before.Execution.AttemptID {
				t.Fatal("query did not continue original operation", after)
			}
			stop()
			waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
		})
	}
}
