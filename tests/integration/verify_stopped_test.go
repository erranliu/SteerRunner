//go:build windows

package integration

import (
	"bytes"
	"encoding/json"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
func testRemoteOfflineScope(t *testing.T) {
	for _, mode := range []string{"absent", "active", "truncated", "legacy", "uncreated", "uncreated-mixed", "invalid-profile"} {
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
			profile := orca.Profile{Executable: exe, Args: []string{"--native-inventory-fixture", inventory}, Environment: "fixture-remote", WaitTimeout: "1s"}
			id, attempt, config := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 64)
			process := b.ProcessIdentity{Kind: "orca_terminal", Executable: exe, ExecutionHost: profile.Environment, RuntimeID: "original", WorktreeID: "repo::C:/original", Terminal: "original-terminal", Incarnation: "original-incarnation"}
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
			if mode == "legacy" {
				version = orca.ProfileVersion
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
			snap.Implementation = []b.ImplementationDataEnvelope{makeEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.StateVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.StateKey, ProjectID: "project", ExecutionID: id, AttemptID: attempt, Generation: snap.ResourceGeneration, ConfigDigest: config}, state)}
			rows := []any{}
			if mode == "active" {
				rows = append(rows, map[string]any{"handle": process.Terminal, "incarnationId": process.Incarnation, "worktreeId": process.WorktreeID, "connected": true})
			}
			raw, _ := json.Marshal(map[string]any{"ok": true, "_meta": map[string]any{"runtimeId": "original"}, "result": map[string]any{"terminals": rows, "totalCount": len(rows), "truncated": mode == "truncated", "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}})
			if err = os.WriteFile(inventory, raw, 0600); err != nil {
				t.Fatal(err)
			}
			record := &runstate.Record{ID: id, Revision: 1, InputDigest: config, StopStatus: "stopped", Snapshot: snap, Profile: []b.ImplementationDataEnvelope{frozen}, RepositoryPath: dir, Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: profile.Environment}}}}
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
			want := mode == "absent" || mode == "uncreated"
			if proof.ResourcesSettled != want || len(proof.Runs) != 1 || proof.Runs[0].ResourcesSettled != want {
				t.Fatal(proof)
			}
			scope := "orca_terminal"
			if mode == "legacy" || mode == "invalid-profile" {
				scope = ""
			}
			if proof.Runs[0].StopEvidenceScope != scope {
				t.Fatal(proof)
			}
			after, _ := os.ReadFile(filepath.Join(dir, "runner.db"))
			if !bytes.Equal(before, after) || bytes.Contains(after, []byte("stop_evidence_scope")) {
				t.Fatal("offline verification modified storage or persisted scope")
			}
			if strings.HasPrefix(mode, "uncreated") || mode == "invalid-profile" {
				if _, err := os.Stat(inventory + ".calls"); !os.IsNotExist(err) {
					t.Fatal("uncreated record caused inventory or helper effect", err)
				}
				return
			}
			calls, err := os.ReadFile(inventory + ".calls")
			if err != nil || !bytes.Contains(calls, []byte("terminal")) || bytes.Contains(calls, []byte("create")) {
				t.Fatal("offline read was not isolated", err, string(calls))
			}
		})
	}
}
