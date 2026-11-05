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
	orcatransport "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Workspace preparation and recovery cannot dispatch legacy terminal input.
func assertCreationBootstrapOnly(t *testing.T, dir, state string) {
	t.Helper()
	assertCreationReadOnlyRPC(t, state)
	f, err := os.Open(filepath.Join(state, "calls.jsonl"))
	if os.IsNotExist(err) {
		// Cancellation before the first CLI dispatch produces no call log.
		// Require the original durable queue proof rather than treating any
		// missing log (including a dispatched/unknown operation) as no work.
		store, readErr := storage.OpenReader(dir)
		if readErr != nil {
			t.Error(readErr)
			return
		}
		defer store.Close()
		runs, readErr := store.Runs()
		if readErr != nil || len(runs) == 0 {
			t.Error("missing original queue records", readErr)
			return
		}
		for _, run := range runs {
			queries := 0
			for _, intent := range run.Intents {
				if intent.PreparationQuery != nil {
					queries++
					if intent.PreparationQuery.Wait.CallsStarted != 0 {
						t.Error("dispatched original query lost its call audit")
					}
				}
			}
			if queries != 1 || run.Allowed || run.StopStatus != "stopped" || run.Snapshot.Ref.External.Kind != "" || run.Snapshot.Workspace.Path != "" {
				t.Error("missing audit without a settled original zero-dispatch queue", run.ID)
			}
		}
		return
	}
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	var sends [][]string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry struct {
			Method string
			Args   []string
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Error(err)
			return
		}
		if entry.Method == "orca/terminal/send" {
			sends = append(sends, entry.Args)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Error(err)
		return
	}
	if len(sends) == 0 {
		store, err := storage.OpenReader(dir)
		if err != nil {
			t.Error(err)
			return
		}
		defer store.Close()
		runs, err := store.Runs()
		if err != nil || len(runs) == 0 {
			t.Error("missing original structured creation records", err)
			return
		}
		for _, run := range runs {
			if run.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || run.StopEvidenceScope != "orca_agent_session" || run.LaunchCommand != "" || run.LaunchInput != nil {
				t.Error("unverified structured bootstrap scope", run.ID)
			}
		}
		return
	}
	t.Error("workspace recovery dispatched legacy input", sends)
}

func assertCreationReadOnlyRPC(t *testing.T, state string) {
	t.Helper()
	f, err := os.Open(filepath.Join(state, "rpc-methods.jsonl"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var method string
		if err := json.Unmarshal(scanner.Bytes(), &method); err != nil {
			t.Error(err)
			return
		}
		switch method {
		case "status.get", "worktree.ps", "terminal.show":
		default:
			t.Error("creation-only fixture received a business RPC", method)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Error(err)
	}
}

func assertOriginalCreationShellClosed(t *testing.T, state, handle string) {
	t.Helper()
	f, err := os.Open(filepath.Join(state, "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry struct {
			Method string
			Args   []string
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Method != "orca/terminal/close" {
			continue
		}
		count++
		found := false
		for i := 0; i+1 < len(entry.Args); i++ {
			if entry.Args[i] == "--terminal" && entry.Args[i+1] == handle {
				found = true
			}
		}
		if !found {
			t.Fatal("closed a foreign creation shell", entry.Args)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("original creation shell close count", count)
	}
	if _, err := os.Stat(filepath.Join(state, "original-terminal-closed")); err != nil {
		t.Fatal("original creation shell cleanup was not confirmed", err)
	}
}

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
					var request struct {
						ID, Method, AuthToken string
						Params                struct{ Terminal string }
					}
					if json.Unmarshal(scanner.Bytes(), &request) != nil || request.AuthToken != "creation-fixture-token" {
						return
					}
					log, err := os.OpenFile(filepath.Join(dir, "rpc-methods.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
					if err != nil {
						t.Error(err)
						return
					}
					writeErr := json.NewEncoder(log).Encode(request.Method)
					closeErr := log.Close()
					if writeErr != nil || closeErr != nil {
						t.Error("RPC method audit failed", writeErr, closeErr)
						return
					}
					result := map[string]any{}
					if request.Method == "worktree.ps" {
						result = map[string]any{"worktrees": []any{}, "totalCount": 0, "truncated": false}
					}
					if request.Method == "terminal.show" {
						// Serve the same durable identity as the CLI fixture, including
						// its original response gate for the receipt-before-commit case.
						var created struct{ Terminal native.Terminal }
						raw, err := os.ReadFile(filepath.Join(dir, "worktree-created.json"))
						if err != nil || json.Unmarshal(raw, &created) != nil || request.Params.Terminal != created.Terminal.Handle {
							return
						}
						_ = process.New(time.Second).WriteExecutionFile(filepath.Join(dir, "show-entered"), []byte("entered"))
						for {
							if _, err := os.Stat(filepath.Join(dir, "hold-show-response")); os.IsNotExist(err) {
								break
							}
							time.Sleep(20 * time.Millisecond)
						}
						result = map[string]any{"terminal": created.Terminal}
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
	for _, mode := range []string{"graceful", "kill", "receipt-before-commit", "wire-loss", "owner-crash", "worktree-receipt-without-terminal", "missing-creation-plan"} {
		t.Run(mode, func(t *testing.T) {
			dir, request := setupRun(t)
			state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
			runtime := "creation-runtime-" + filepath.Base(filepath.Dir(state))
			t.Setenv("STEERRUNNER_CREATION_RUNTIME", runtime)
			creationObservationFixture(t, state, runtime)
			t.Cleanup(func() { assertCreationReadOnlyRPC(t, state) })
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
			if mode == "worktree-receipt-without-terminal" {
				_ = os.WriteFile(filepath.Join(state, "omit-create-terminal-identity"), nil, 0600)
			}
			var input contracts.RunRequest
			_ = json.Unmarshal(request.Input, &input)
			// Hold the original show until the service is killed; a two-second
			// query timeout could commit an unknown receipt between inspect and kill,
			// invalidating the fault window's captured revision under disk load.
			settings, _ := json.Marshal(map[string]any{"access_mode": "full_access", "transport": map[string]any{"executable": helperEXE, "args": []string{"--orca-create-fixture"}, "wait_timeout": "15s"}})
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
			plans, err := filepath.Glob(filepath.Join(dir, "orca-initial-terminals", "*.json"))
			if err != nil || len(plans) != 0 {
				t.Fatal("structured preparation created a terminal launch plan", plans, err)
			}
			if before.Run.StopEvidenceScope != "orca_agent_session" || before.Run.Execution.Protocol != string(b.ProtocolOrcaAgentSessionV1) || before.Run.Execution.Agent != "codex" || before.Run.Execution.TurnStarted {
				t.Fatal("fixture did not freeze the original structured contract", before.Run)
			}
			if mode == "missing-creation-plan" {
				if err := os.Remove(filepath.Join(filepath.Dir(ownerPath), orcatransport.CreationOwnerConfigFile)); err != nil {
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
			if mode == "wire-loss" || mode == "owner-crash" || mode == "missing-creation-plan" {
				unprovenOriginal := func(v contracts.RunView) bool {
					// A pending stop has priority over preparation diagnostics. The
					// original creation must still be unknown and unauthorized; its
					// coarse status need not overwrite the durable stop projection.
					return !v.NewInputsAllowed && v.ResumeRequired && len(v.Operations) == 1 &&
						v.Operations[0].ID == before.Run.Operations[0].ID && v.Operations[0].Action == "prepare_workspace" && v.Operations[0].Status == "outcome_unknown" &&
						(v.Status == "reconciling" || v.Phase == "stop_requested" && v.StopStatus == "pending")
				}
				view := waitRun(t, dir, id, unprovenOriginal)
				time.Sleep(1200 * time.Millisecond)
				view = waitRun(t, dir, id, unprovenOriginal)
				if view.StopStatus == "stopped" || view.Execution.Workspace != "" || view.Execution.SessionID != "" || view.Execution.TurnStarted || countNative(t, "orca/terminal/close") != 0 || countNative(t, "orca/worktree/list") != 0 {
					t.Fatal("unproven external creation was adopted, closed or settled", view)
				}
			} else {
				view := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
				if view.Execution.Workspace != effect.Worktree.Path || view.Execution.Branch != effect.Worktree.Branch || view.Execution.SessionID != "" || view.Execution.TurnStarted || view.NewInputsAllowed || view.StopEvidenceScope != "orca_agent_session" || view.Execution.Protocol != string(b.ProtocolOrcaAgentSessionV1) || len(view.Operations) == 0 || view.Operations[0].ID != before.Run.Operations[0].ID || view.Operations[0].Action != "prepare_workspace" || view.Operations[0].Status != "confirmed" || countNative(t, "orca/terminal/close") != 1 {
					t.Fatalf("original creation identity/stop was not recovered: workspace=%q want=%q branch=%q want=%q operation=%+v want=%+v terminal-close=%d view=%+v", view.Execution.Workspace, effect.Worktree.Path, view.Execution.Branch, effect.Worktree.Branch, view.Operations, before.Run.Operations, countNative(t, "orca/terminal/close"), view)
				}
				assertOriginalCreationShellClosed(t, state, effect.Terminal.Handle)
			}
			if countNative(t, "orca/worktree/create") != 1 || countNative(t, "orca/terminal/create") != 0 || countNative(t, "orca/terminal/send") != 0 {
				t.Fatal("recovery repeated resource creation or sent business input")
			}
			if mode != "wire-loss" && mode != "owner-crash" && mode != "missing-creation-plan" {
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("inspect-original-creation", next.InstanceID), 0)
				if err := waitOwnedService(next); err != nil {
					t.Fatal(err)
				}
				store, err := storage.OpenReader(dir)
				if err != nil {
					t.Fatal(err)
				}
				runs, err := store.Runs()
				_ = store.Close()
				if err != nil || len(runs) != 1 {
					t.Fatal("original record unavailable", err)
				}
				snapshot := runs[0].Snapshot
				if snapshot.Ref.External.RuntimeID != runtime || snapshot.Ref.External.WorktreeID != effect.Worktree.ID || snapshot.Ref.External.Kind != "orca_agent_session" || snapshot.Ref.External.Terminal != "" {
					t.Fatal("recovery changed original structured workspace", snapshot.Ref)
				}
				var frozen struct {
					SessionNotCreated bool            `json:"session_not_created"`
					Attempted         bool            `json:"attempted"`
					CloseAttempted    bool            `json:"close_attempted"`
					CloseReceipt      json.RawMessage `json:"close_receipt"`
				}
				found := false
				for _, e := range snapshot.Implementation {
					if e.ImplementationKey == orcatransport.AgentSessionStateKey {
						found = true
						if json.Unmarshal(e.Payload, &frozen) != nil {
							t.Fatal("invalid original Agent state")
						}
					}
				}
				// CloseAttempted records the original stop operation even when no
				// Agent existed; only a native acknowledgement is a close receipt.
				if !found || !frozen.SessionNotCreated || frozen.Attempted || !frozen.CloseAttempted || len(frozen.CloseReceipt) != 0 {
					t.Fatal("undispatched Agent lost its original not-created proof", frozen)
				}
			}
			t.Logf("original=%s runtime=%s handle=%s incarnation=%s create=1 terminal-create=0 business-send=0", effect.Worktree.ID, runtime, effect.Terminal.Handle, effect.Terminal.IncarnationID)
		})
	}
}
