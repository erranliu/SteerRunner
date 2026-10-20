//go:build windows

package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"golang.org/x/sys/windows"
)

func restartEdgeService(t *testing.T, dir string, old contracts.ServiceInfo, index int) contracts.ServiceInfo {
	t.Helper()
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest(fmt.Sprintf("edge-service-stop-%d", index), old.InstanceID), 0)
	if err := waitOwnedService(old); err != nil {
		t.Fatal(err)
	}
	next := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	if next.InstanceID == old.InstanceID {
		t.Fatal("service did not change generation")
	}
	cleanupService(t, dir, next)
	return next
}

// Original final evidence is lost in the protocol fixture, never in a real
// user's native history. Two independent service generations must preserve the
// missing result and its original input guard without recreating business input.
func TestSREdgePermanentEvidenceLoss(t *testing.T) {
	testCount.Add(1)
	dir, envelope := setupRun(t)
	state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	if err := os.WriteFile(filepath.Join(state, "missing-final"), []byte("original final unavailable"), 0600); err != nil {
		t.Fatal(err)
	}
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	raw, _ := json.Marshal(envelope)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	original := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Phase == "final_unavailable" })
	starts := countNative(t, "turn/start")
	service = restartEdgeService(t, dir, service, 0)
	// Remove only the generated fixture's original thread file, with an exact
	// parent check. No production state or authentication file is modified.
	path := filepath.Join(state, original.Execution.SessionID+".json")
	if filepath.Dir(path) != filepath.Clean(state) {
		t.Fatal("thread history escapes owned fixture")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		service = restartEdgeService(t, dir, service, i)
		view := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Phase == "final_unavailable" })
		if view.NewInputsAllowed != original.NewInputsAllowed || view.ResumeRequired != original.ResumeRequired || view.PR != nil || view.Execution.FinalAvailable || view.Execution.SessionID != original.Execution.SessionID || view.InputDigest != original.InputDigest {
			t.Fatal("missing original source became a new result or authority")
		}
		if countNative(t, "turn/start") != starts || countNative(t, "thread/start") != 1 {
			t.Fatal("service generations replayed unknown business input")
		}
	}
	current := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Phase == "final_unavailable" })
	if current.Controls.CanResume {
		// Explicit authorization must also verify the original source, not just
		// leave the Run safe because no caller ever attempted to continue it.
		auditControl(t, dir, id, "resume", map[string]any{})
		current = waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "blocked" && v.Phase == "control_rejected" })
	} else {
		input, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: current.Revision})
		raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "edge-evidence-resume", Input: input})
		response := call(t, dir, []string{"resume", "--run-id", id, "--input", "-"}, string(raw), 5)
		if response.Error == nil || response.Error.Code != "invalid_state" {
			t.Fatal("missing original source gained a new resume authorization")
		}
	}
	if current.NewInputsAllowed != original.NewInputsAllowed || current.ResumeRequired != original.ResumeRequired || current.PR != nil || current.Execution.SessionID != original.Execution.SessionID || countNative(t, "turn/start") != starts || countNative(t, "thread/start") != 1 {
		t.Fatal("explicit resume replaced unavailable original source")
	}
	t.Logf("outcome=original_evidence_unverified original=%s starts=%d; native history remains unavailable", id, starts)
}

// The real managed worker is stopped, but its receipt transaction is rejected.
// A second service generation also fails that transaction; only removal of the
// fixture's SQLite trigger permits the original intent to settle.
func TestSREdgeStopCommitRace(t *testing.T) {
	testCount.Add(1)
	dir, envelope := setupRun(t)
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	raw, _ := json.Marshal(envelope)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	ready := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	compatibilitySQL(t, dir, `CREATE TRIGGER edge_fail_stop BEFORE INSERT ON events WHEN json_extract(NEW.body,'$.type')='stop_stopped' BEGIN SELECT RAISE(ABORT,'edge stop receipt commit failed'); END;`)
	defer compatibilitySQL(t, dir, `DROP TRIGGER IF EXISTS edge_fail_stop`)
	external := compatibilityRecord(t, dir).Snapshot.Ref.External
	revokeWithFreshRevision(t, dir, id, ready.Revision)
	pending := waitRun(t, dir, id, func(v contracts.RunView) bool {
		return v.StopStatus == "pending" && !v.NewInputsAllowed && process.InspectProcess(&external) == "process absent"
	})
	original := compatibilityRecord(t, dir)
	starts := countNative(t, "turn/start")
	service = restartEdgeService(t, dir, service, 0)
	after := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.ResumeRequired })
	if after.NewInputsAllowed || after.StopStatus == "stopped" || countNative(t, "turn/start") != starts || len(after.Operations) != len(pending.Operations) {
		t.Fatal("failed receipt commit fabricated success or replayed an operation")
	}
	compatibilitySQL(t, dir, `DROP TRIGGER edge_fail_stop`)
	restartEdgeService(t, dir, service, 1)
	settled := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" && !v.NewInputsAllowed })
	current := compatibilityRecord(t, dir)
	if len(current.Intents) != len(original.Intents) || settled.Execution.SessionID != ready.Execution.SessionID || countNative(t, "turn/start") != starts {
		t.Fatal("receipt recovery replaced original identities")
	}
	for i := range original.Intents {
		if current.Intents[i].Request.Operation.ID != original.Intents[i].Request.Operation.ID {
			t.Fatal("receipt recovery created a new intent")
		}
	}
	t.Log("outcome=stopped original intent settled after two receipt-commit failures/generations")
}

func TestSREdgeStorageFailure(t *testing.T) {
	testCount.Add(1)
	dir, envelope := setupRun(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	compatibilitySQL(t, dir, `CREATE TRIGGER edge_fail_accept BEFORE INSERT ON commands BEGIN SELECT RAISE(ABORT,'edge durable acceptance unavailable'); END;`)
	defer compatibilitySQL(t, dir, `DROP TRIGGER IF EXISTS edge_fail_accept`)
	raw, _ := json.Marshal(envelope)
	response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 6)
	if response.OK || response.Error == nil || response.Error.Code != "request_outcome_unknown" {
		t.Fatal("failed transaction advertised durable acceptance")
	}
	lookup := call(t, dir, []string{"inspect", "--request-id", envelope.RequestID}, "", 5)
	if lookup.Error == nil || lookup.Error.Code != "request_not_found" || countNative(t, "thread/start") != 0 || countNative(t, "turn/start") != 0 {
		t.Fatal("rolled-back submission created receipt or business work")
	}
	compatibilitySQL(t, dir, `DROP TRIGGER edge_fail_accept`)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	ready := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	if countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 3 {
		t.Fatal("confirmed-not-accepted retry duplicated business work")
	}
	revokeWithFreshRevision(t, dir, id, ready.Revision)
	waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	t.Log("outcome=rejected acceptance rollback verified; original-ID retry after recovery executed once")
}

// A frozen, bounded load qualifies resource growth without exhausting the user's
// machine. It is scheduled exclusively by L4. Each native process must actually
// exit, rather than relying on task/Run counts as a proxy for released handles.
func TestSREdgeResourcePressure(t *testing.T) {
	testCount.Add(1)
	const rounds = 20
	const handleGrowth = 128
	dir, envelope := setupRun(t)
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(service.PID))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	getHandles := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	measure := func() uint32 {
		var count uint32
		ok, _, err := getHandles.Call(uintptr(handle), uintptr(unsafe.Pointer(&count)))
		if ok == 0 {
			t.Fatal(err)
		}
		return count
	}
	baseline := uint32(0)
	for i := 0; i < rounds; i++ {
		envelope.RequestID = fmt.Sprintf("pressure-%d", i)
		raw, _ := json.Marshal(envelope)
		accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
		id := accepted.Data.(map[string]any)["run_id"].(string)
		ready := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
		replay := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
		if replay.Data.(map[string]any)["run_id"] != id {
			t.Fatal("pressure replay created duplicate Run")
		}
		external := compatibilityRecordByID(t, dir, id).Snapshot.Ref.External
		revokeWithFreshRevision(t, dir, id, ready.Revision)
		waitRun(t, dir, id, func(v contracts.RunView) bool {
			return v.StopStatus == "stopped" && !v.NewInputsAllowed && process.InspectProcess(&external) == "process absent"
		})
		current := measure()
		if i == 0 {
			baseline = current
		}
		if current > baseline+handleGrowth {
			t.Fatalf("service handles grew beyond frozen allowance: baseline=%d current=%d limit=%d", baseline, current, handleGrowth)
		}
		t.Logf("round=%d handles=%d baseline=%d", i+1, current, baseline)
	}
	if countNative(t, "thread/start") != rounds || countNative(t, "turn/start") != rounds*3 {
		t.Fatal("load duplicated or skipped native work")
	}
	t.Logf("outcome=stopped rounds=%d handle_growth_limit=%d", rounds, handleGrowth)
}

// Existing compatibility helper reads a one-Run fixture. Load pressure records
// by the public Run ID while keeping fault-injection SQL inside test code.
func compatibilityRecordByID(t *testing.T, dir, id string) runstate.Record {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var body []byte
	if err = db.QueryRow(`SELECT body FROM runs WHERE id=?`, id).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var record runstate.Record
	if err = json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return record
}
