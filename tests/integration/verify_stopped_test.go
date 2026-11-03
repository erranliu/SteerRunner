//go:build windows

package integration

import (
	"bytes"
	"encoding/json"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func testOfflineVerificationAfterWriterExitWithoutShutdownReceipt(t *testing.T) {
	dir, req := setupRun(t)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(req)
	response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	id := response.Data.(map[string]any)["run_id"].(string)
	waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	if v := call(t, dir, []string{"service", "verify-stopped"}, "", 5); v.Error.Code != "directory_conflict" {
		t.Fatal(v)
	}
	nativeCount := countNative(t, "turn/start")
	if err := killOwnedService(info); err != nil {
		t.Fatal(err)
	}
	if err := waitOwnedService(info); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	var proof contracts.StoppedVerification
	for i := 0; i < 2; i++ {
		response = call(t, dir, []string{"service", "verify-stopped"}, "", 0)
		raw, _ = json.Marshal(response.Data)
		if err = json.Unmarshal(raw, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.ResourcesSettled {
			t.Fatal("service exit was misreported as native tree settlement", proof)
		}
	}
	if len(proof.Runs) != 1 || proof.Runs[0].RunID != id || proof.Runs[0].ResourcesSettled {
		t.Fatal(proof)
	}
	after, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("offline verification changed execution storage", err)
	}
	if countNative(t, "turn/start") != nativeCount {
		t.Fatal("offline verification generated native input")
	}
	if v := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0)); v.Status != "not_running" || v.LastShutdown != nil {
		t.Fatal("verification restarted service or rewrote shutdown", v)
	}
}

// Persist only the original identity through the real SQLite store, then invoke
// the public offline EXE. The fixture permits inventory reads and no mutation.
func testRemoteOfflineScope(t *testing.T) { testOrcaOfflineScope(t, "fixture-remote", false) }

func testNativeCandidateOfflineScope(t *testing.T) {
	for _, environment := range []string{"", "fixture-remote"} {
		t.Run(environment, func(t *testing.T) { testOrcaOfflineScope(t, environment, true) })
	}
}

func testOrcaOfflineScope(t *testing.T, environment string, candidate bool) {
	modes := []string{"absent", "active", "truncated", "legacy", "uncreated", "uncreated-mixed", "invalid-profile", "receipt-restarted", "receipt-removed", "receipt-offline", "receipt-missing", "receipt-accepted", "receipt-unknown", "receipt-not-absent", "receipt-foreign", "receipt-incarnation", "receipt-generation", "receipt-input", "receipt-pending-resume", "receipt-pending-stop", "receipt-unknown-create", "receipt-corrupt", "receipt-scope-mismatch"}
	if candidate {
		modes = append(modes, "missing-scope", "old-state")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "service", "writer.lock"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			inventory := filepath.Join(dir, "inventory.json")
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			profile := orca.Profile{NativeTerminal: candidate, Executable: exe, Args: []string{"--native-inventory-fixture", inventory}, Environment: environment, WaitTimeout: "1s"}
			id, attempt, config := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 64)
			process := b.ProcessIdentity{Kind: "orca_terminal", Executable: exe, ExecutionHost: profile.Target().ExecutionHost, RuntimeID: "original", WorktreeID: "repo::C:/original", Terminal: "original-terminal", Incarnation: "original-incarnation"}
			snap := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: id, AttemptID: attempt, External: process}, Session: b.SessionContext{ProjectID: "project", ExecutionID: id, WorkflowRunID: id, AttemptID: attempt}, ConfigurationDigest: config, ResourceGeneration: "original-generation"}
			makeEnvelope := func(meta b.ImplementationDataEnvelope, value any) b.ImplementationDataEnvelope {
				raw, _ := json.Marshal(value)
				e, err := b.NewImplementationDataEnvelope(meta, raw)
				if err != nil {
					t.Fatal(err)
				}
				return e
			}
			version := orca.NativeRemoteProfileVersion
			if candidate {
				version = orca.NativeProfileVersion
			}
			if mode == "legacy" {
				version = 3 // Retired profile, rejected before native inventory.
			}
			frozen := makeEnvelope(b.ImplementationDataEnvelope{CodecVersion: version, Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.ProfileKey, ProjectID: "project", ProfileID: id, AcceptedConfigVersion: config}, profile)
			if mode == "invalid-profile" {
				raw, _ := json.Marshal(profile)
				var fields map[string]any
				_ = json.Unmarshal(raw, &fields)
				fields["runtime_executable"] = ""
				frozen = makeEnvelope(frozen, fields)
			}
			state := orca.State{Process: process}
			if strings.HasPrefix(mode, "uncreated") {
				snap.Ref.External = b.ProcessIdentity{}
				state.Process = b.ProcessIdentity{}
				if mode == "uncreated-mixed" {
					state.Control = &orca.ControlTerminal{}
				}
			}
			stateVersion := orca.StateVersion
			if candidate {
				stateVersion = orca.NativeStateVersion
			}
			if mode == "old-state" {
				stateVersion = orca.LocalStateVersion
			}
			snap.Implementation = []b.ImplementationDataEnvelope{makeEnvelope(b.ImplementationDataEnvelope{CodecVersion: stateVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.StateKey, ProjectID: "project", ExecutionID: id, AttemptID: attempt, Generation: snap.ResourceGeneration, ConfigDigest: config}, state)}
			rows := []any{}
			if mode == "active" {
				rows = append(rows, map[string]any{"handle": process.Terminal, "incarnationId": process.Incarnation, "worktreeId": process.WorktreeID, "connected": true})
			}
			raw, _ := json.Marshal(map[string]any{"ok": true, "_meta": map[string]any{"runtimeId": "original"}, "result": map[string]any{"terminals": rows, "totalCount": len(rows), "truncated": mode == "truncated", "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}})
			if err = os.WriteFile(inventory, raw, 0600); err != nil {
				t.Fatal(err)
			}
			record := &runstate.Record{ID: id, Revision: 1, InputDigest: config, StopStatus: "stopped", Snapshot: snap, Profile: []b.ImplementationDataEnvelope{frozen}, RepositoryPath: dir, Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: profile.Environment}}}}
			if environment == "" {
				record.Request.Execution.Target = contracts.Target{Kind: "local"}
			}
			if candidate && mode != "missing-scope" {
				record.StopEvidenceScope = "orca_terminal"
			}
			confirmed := mode == "receipt-restarted" || mode == "receipt-removed" || mode == "receipt-offline"
			if strings.HasPrefix(mode, "receipt-") {
				intent, e := operation.New(id, snap, b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{}}}, "original-stop", "original-stop-command", time.Now())
				if e != nil {
					t.Fatal(e)
				}
				receipt := b.LifecycleReceipt{Operation: intent.Request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}}
				receipt.Operation.Receipt = []byte(`{"version":1,"phase":"confirmed"}`)
				switch mode {
				case "receipt-accepted":
					receipt.Disposition = b.OperationAccepted
				case "receipt-unknown":
					receipt.Disposition = b.OperationUnknown
				case "receipt-not-absent":
					receipt.State.WorkerAbsent = false
				case "receipt-foreign":
					receipt.Operation.ID = "foreign-stop"
				}
				intent.SettleStop(record.Snapshot.Operations, receipt, receipt.Disposition)
				record.Intents = []operation.Intent{intent}
				switch mode {
				case "receipt-missing":
					record.Intents = nil
				case "receipt-corrupt":
					record.Intents[0].Request.Operation.Receipt = []byte(`{`)
				case "receipt-incarnation":
					record.Snapshot.Ref.External.Incarnation = "replacement"
				case "receipt-generation":
					record.Snapshot.ResourceGeneration = "new-generation"
				case "receipt-input":
					record.InputDigest = "different-input"
				case "receipt-pending-resume":
					record.PendingResume = "new-resume"
				case "receipt-pending-stop":
					record.PendingStop = "unresolved-stop"
				case "receipt-unknown-create":
					pending := intent
					pending.Request = pending.Request.Clone()
					pending.Request.Operation.Action = string(b.LifecycleCreateResource)
					pending.Request.Operation.Status = b.OperationUnknown
					record.Intents = append(record.Intents, pending)
				case "receipt-scope-mismatch":
					record.StopEvidenceScope = orca.LocalCodexStopScope
				}
				// Simulate a restarted host, removed workspace, or unreachable host.
				// Successful durable proof must not consult any of these inventories.
				var unavailable any = map[string]any{"ok": false, "error": map[string]any{"code": "selector_not_found"}, "_meta": map[string]any{"runtimeId": "original"}}
				if mode == "receipt-restarted" {
					unavailable = map[string]any{"ok": true, "_meta": map[string]any{"runtimeId": "replacement-runtime"}, "result": map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}}
				}
				if mode == "receipt-offline" {
					unavailable = map[string]any{"ok": false, "error": map[string]any{"code": "remote_runtime_unavailable"}}
				}
				wire, _ := json.Marshal(unavailable)
				if err = os.WriteFile(inventory, wire, 0600); err != nil {
					t.Fatal(err)
				}
			}
			store, err := storage.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Commit(record, 0, nil); err != nil {
				t.Fatal(err)
			}
			_ = store.Close()
			before, err := os.ReadFile(filepath.Join(dir, "runner.db"))
			if err != nil {
				t.Fatal(err)
			}
			response := call(t, dir, []string{"service", "verify-stopped"}, "", 0)
			raw, _ = json.Marshal(response.Data)
			var proof contracts.StoppedVerification
			if err = json.Unmarshal(raw, &proof); err != nil {
				t.Fatal(err)
			}
			want := mode == "absent" || mode == "uncreated" || confirmed
			if proof.ResourcesSettled != want || len(proof.Runs) != 1 || proof.Runs[0].ResourcesSettled != want {
				t.Fatal(proof)
			}
			scope := "orca_terminal"
			if mode == "legacy" || mode == "invalid-profile" || mode == "receipt-input" {
				scope = ""
			}
			if proof.Runs[0].StopEvidenceScope != scope {
				t.Fatal(proof)
			}
			after, _ := os.ReadFile(filepath.Join(dir, "runner.db"))
			if !bytes.Equal(before, after) || !candidate && mode != "receipt-scope-mismatch" && bytes.Contains(after, []byte("stop_evidence_scope")) {
				t.Fatal("offline verification modified storage or persisted scope")
			}
			if confirmed || mode == "receipt-incarnation" || mode == "receipt-input" || mode == "receipt-generation" || mode == "receipt-scope-mismatch" || strings.HasPrefix(mode, "uncreated") || mode == "invalid-profile" || mode == "legacy" || mode == "missing-scope" || mode == "old-state" {
				if _, err := os.Stat(inventory + ".calls"); !os.IsNotExist(err) {
					t.Fatal("uncreated record caused inventory or helper effect", err)
				}
				return
			}
			calls, err := os.ReadFile(inventory + ".calls")
			if err != nil || !bytes.Contains(calls, []byte("terminal")) || bytes.Contains(calls, []byte("create")) || bytes.Contains(calls, []byte("close")) || bytes.Contains(calls, []byte("send")) {
				t.Fatal("offline read was not isolated", err, string(calls))
			}
		})
	}
}
