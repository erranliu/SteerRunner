//go:build windows

package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func creationObservationFixture(t *testing.T, dir, runtime string) {
	t.Helper()
	// SerializeCLI uses the user cache. This fixture has no shared Orca files;
	// isolate its lock too so an intentional held response cannot block other
	// test processes or the user's real CLI.
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "cache"))
	endpoint := fmt.Sprintf(`\\.\pipe\sr-create-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var request struct{ ID, Method, AuthToken string }
					if json.Unmarshal(scanner.Bytes(), &request) != nil || request.AuthToken != "creation-fixture-token" {
						return
					}
					result := map[string]any{}
					if request.Method == "worktree.ps" {
						result = map[string]any{"worktrees": []any{}, "totalCount": 0, "truncated": false}
					}
					_ = json.NewEncoder(conn).Encode(map[string]any{"id": request.ID, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}})
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	raw, _ := json.Marshal(map[string]any{"runtimeId": runtime, "authToken": "creation-fixture-token", "transports": []any{map[string]string{"kind": "named-pipe", "endpoint": endpoint}}})
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func awaitCreationFile(t *testing.T, pattern string) string {
	t.Helper()
	for until := time.Now().Add(60 * time.Second); time.Now().Before(until); time.Sleep(30 * time.Millisecond) {
		paths, _ := filepath.Glob(pattern)
		if len(paths) == 1 {
			return paths[0]
		}
		if len(paths) > 1 {
			t.Fatal("multiple original creation owners", paths)
		}
	}
	t.Fatal("original creation fixture window not reached", pattern)
	return ""
}

// An intentionally unknown external creation cannot truthfully settle as
// stopped. This fixture owns no real Orca resources, so cleanup verifies only
// its exact service and finite creation-owner processes, retaining that verdict.
func cleanupCreationService(t *testing.T, dir string, info contracts.ServiceInfo) {
	t.Helper()
	t.Cleanup(func() {
		response, exit, _, err := invoke(runnerEXE, []string{"service", "status", "--state-dir", dir, "--json"}, "")
		var current contracts.ServiceInfo
		raw, _ := json.Marshal(response.Data)
		_ = json.Unmarshal(raw, &current)
		if err == nil && exit == 0 && current.InstanceID == info.InstanceID {
			_, _, _, _ = invoke(runnerEXE, []string{"service", "stop", "--state-dir", dir, "--input", "-", "--json"}, stopRequest("cleanup-"+info.InstanceID, info.InstanceID))
			if err := waitOwnedService(info); err != nil {
				if killErr := killOwnedService(info); killErr != nil {
					t.Error(err, killErr)
				}
			}
		}
		paths, _ := filepath.Glob(filepath.Join(dir, "orca-creations", "*", "create-started.json"))
		for _, path := range paths {
			var started struct{ Process b.ProcessIdentity }
			raw, _ := os.ReadFile(path)
			if json.Unmarshal(raw, &started) != nil {
				t.Error("unreadable fixture owner identity")
				continue
			}
			until := time.Now().Add(5 * time.Second)
			for process.InspectProcess(&started.Process) == "matching process still exists" && time.Now().Before(until) {
				time.Sleep(30 * time.Millisecond)
			}
			if process.InspectProcess(&started.Process) == "matching process still exists" {
				p, err := os.FindProcess(started.Process.PID)
				if err != nil || p.Kill() != nil {
					t.Error("cannot clean exact fixture creation owner", err)
				}
			}
		}
	})
}

func TestSRResourceCreationRecovery(t *testing.T) {
	testCount.Add(1)
	for _, mode := range []string{"graceful", "kill", "receipt-before-commit", "wire-loss", "owner-crash", "planned-initial-terminal", "missing-initial-plan"} {
		t.Run(mode, func(t *testing.T) {
			dir, request := setupRun(t)
			state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
			runtime := "creation-runtime-" + filepath.Base(filepath.Dir(state))
			t.Setenv("STEERRUNNER_CREATION_RUNTIME", runtime)
			creationObservationFixture(t, state, runtime)
			marker := "hold-create-response"
			if mode == "receipt-before-commit" {
				marker = "hold-show-response"
			}
			if err := os.WriteFile(filepath.Join(state, marker), nil, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "wire-loss" {
				_ = os.WriteFile(filepath.Join(state, "lose-create-response"), nil, 0600)
			}
			if mode == "planned-initial-terminal" || mode == "missing-initial-plan" {
				_ = os.WriteFile(filepath.Join(state, "omit-create-terminal-identity"), nil, 0600)
			}
			var input contracts.RunRequest
			_ = json.Unmarshal(request.Input, &input)
			// Hold the original show until the service is killed; a two-second
			// query timeout could commit an unknown receipt between inspect and kill,
			// invalidating the fault window's captured revision under disk load.
			settings, _ := json.Marshal(map[string]any{"transport": map[string]any{"executable": helperEXE, "args": []string{"--orca-create-fixture"}, "wait_timeout": "15s"}})
			input.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: settings}
			request.Input, _ = json.Marshal(input)
			service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupCreationService(t, dir, service)
			// Release gates before service cleanup even if a fault-window assertion
			// fails. The original owner remains subject to its production deadline.
			t.Cleanup(func() { _ = os.Remove(filepath.Join(state, marker)) })
			raw, _ := json.Marshal(request)
			accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			id := accepted.Data.(map[string]any)["run_id"].(string)
			var effect struct {
				Worktree native.Worktree
				Terminal native.Terminal
				CLI      b.ProcessIdentity
			}
			path := awaitCreationFile(t, filepath.Join(state, "worktree-created.json"))
			body, _ := os.ReadFile(path)
			if json.Unmarshal(body, &effect) != nil {
				t.Fatal("fixture did not record actual creation identity")
			}
			ownerPath := awaitCreationFile(t, filepath.Join(dir, "orca-creations", "*", "owner-ready.json"))
			var owner struct{ Process b.ProcessIdentity }
			body, _ = os.ReadFile(ownerPath)
			if json.Unmarshal(body, &owner) != nil || process.InspectProcess(&owner.Process) != "matching process still exists" && mode != "receipt-before-commit" {
				t.Fatal("original independent creation owner not observed")
			}
			if mode == "receipt-before-commit" {
				awaitCreationFile(t, filepath.Join(state, "show-entered"))
				awaitCreationFile(t, filepath.Join(filepath.Dir(ownerPath), "create-result.json"))
			}
			// Observe through the service's serialized public read. OpenReader is
			// reserved for an offline writer lease, not a second active DB client.
			observed := call(t, dir, []string{"inspect", "--run-id", id}, "", 0)
			var before struct {
				Run contracts.RunView `json:"run"`
			}
			body, _ = json.Marshal(observed.Data)
			if json.Unmarshal(body, &before) != nil || before.Run.Execution.Workspace != "" || before.Run.Execution.SessionID != "" || len(before.Run.Operations) != 1 || before.Run.Operations[0].Status != "in_flight" {
				t.Fatal("fault did not precede the original receipt commit", before.Run)
			}
			if mode == "graceful" || mode == "wire-loss" {
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("detach-create", service.InstanceID), 0)
				if err := waitOwnedService(service); err != nil {
					t.Fatal(err)
				}
			} else if err := killOwnedService(service); err != nil {
				t.Fatal(err)
			}
			if err := waitOwnedService(service); err != nil {
				t.Fatal("original service did not exit at the creation gate", err)
			}
			if mode == "missing-initial-plan" {
				plans, _ := filepath.Glob(filepath.Join(dir, "orca-initial-terminals", "*.json"))
				if len(plans) != 1 {
					t.Fatal("original terminal preparation plan missing", plans)
				}
				if err := os.Remove(plans[0]); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "receipt-before-commit" && (process.InspectProcess(&owner.Process) != "matching process still exists" || process.InspectProcess(&effect.CLI) != "matching process still exists") {
				t.Fatal("service exit killed or replaced the independent original CLI")
			}
			if mode == "owner-crash" {
				p, err := os.FindProcess(owner.Process.PID)
				if err != nil || process.InspectProcess(&owner.Process) != "matching process still exists" || p.Kill() != nil {
					t.Fatal("exact original owner crash failed", err)
				}
				until := time.Now().Add(5 * time.Second)
				for (process.InspectProcess(&owner.Process) != "process absent" || process.InspectProcess(&effect.CLI) != "process absent") && time.Now().Before(until) {
					time.Sleep(30 * time.Millisecond)
				}
				if process.InspectProcess(&owner.Process) != "process absent" || process.InspectProcess(&effect.CLI) != "process absent" {
					t.Fatal("original owner/CLI did not exit at the crash gate")
				}
			}
			if err := os.Remove(filepath.Join(state, marker)); err != nil {
				t.Fatal(err)
			}
			if mode != "owner-crash" {
				awaitCreationFile(t, filepath.Join(filepath.Dir(ownerPath), "create-result.json"))
			}
			var stop contracts.Request
			if mode != "graceful" && mode != "wire-loss" {
				// The service was killed while its original create response was held;
				// no later receipt could advance this observed durable revision.
				// A readonly SQLite connection need not recover a just-crashed WAL;
				// the replacement's exclusive writer performs normal recovery.
				raw, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: before.Run.Revision})
				stop = contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-original-creation", Input: raw}
			} else {
				stop = offlineContentStop(t, dir, id, "stop-original-creation")
			}
			start := contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: id, Request: stop}}}
			next := serviceInfo(t, call(t, dir, []string{"service", "start", "--input", "-"}, string(start.JSON()), 0))
			cleanupCreationService(t, dir, next)
			if mode == "wire-loss" || mode == "owner-crash" || mode == "missing-initial-plan" {
				view := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "reconciling" && !v.NewInputsAllowed })
				time.Sleep(1200 * time.Millisecond)
				view = waitRun(t, dir, id, func(v contracts.RunView) bool { return !v.NewInputsAllowed })
				if view.StopStatus == "stopped" || view.Execution.SessionID != "" || countNative(t, "orca/terminal/close") != 0 || countNative(t, "orca/worktree/list") != 0 {
					t.Fatal("unproven external creation was adopted, closed or settled", view)
				}
			} else {
				view := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
				if view.Execution.Workspace != effect.Worktree.Path || view.Execution.SessionID != "orca-terminal:"+runtime+":"+effect.Terminal.Handle+":"+effect.Terminal.IncarnationID || view.NewInputsAllowed || countNative(t, "orca/terminal/close") != 1 {
					t.Fatal("original creation identity/stop was not recovered", view)
				}
			}
			if countNative(t, "orca/worktree/create") != 1 || countNative(t, "orca/terminal/create") != 0 || countNative(t, "orca/terminal/send") != 0 {
				t.Fatal("recovery repeated resource creation or sent business input")
			}
			t.Logf("original=%s runtime=%s handle=%s incarnation=%s create=1 terminal-create=0 business-send=0", effect.Worktree.ID, runtime, effect.Terminal.Handle, effect.Terminal.IncarnationID)
		})
	}
}
