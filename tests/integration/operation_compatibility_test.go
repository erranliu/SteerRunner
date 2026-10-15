//go:build windows

package integration

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
	_ "modernc.org/sqlite"
)

// Exercise the public commands against a real previous EXE, then continue the
// same isolated state directory with the current EXE. No business SQL is edited.
func TestSROperationV2Compatibility(t *testing.T) {
	if *baselineFlag == "" {
		t.Skip("requires an explicit previous EXE via -baseline-exe")
	}
	if _, err := os.Stat(*baselineFlag); err != nil {
		t.Fatal(err)
	}
	testCount.Add(1)
	currentEXE := runnerEXE
	for _, state := range []string{"pr_ready", "stopped", "unknown", "stop_pending"} {
		t.Run(state, func(t *testing.T) {
			runnerEXE = *baselineFlag
			t.Cleanup(func() { runnerEXE = currentEXE })
			dir, request := setupRun(t)
			if state == "unknown" {
				if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "drop-next-start"), []byte("drop original receipt"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			originalService := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, originalService)
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			id := accepted.Data.(map[string]any)["run_id"].(string)
			before := waitRun(t, dir, id, func(r contracts.RunView) bool {
				if state == "unknown" {
					return r.Phase == "operation_outcome_unknown"
				}
				return r.Status == "pr_ready"
			})
			if state == "stopped" || state == "stop_pending" {
				if state == "stop_pending" {
					compatibilitySQL(t, dir, `CREATE TRIGGER fail_stop_receipt BEFORE INSERT ON events WHEN json_extract(NEW.body,'$.type')='stop_stopped' BEGIN SELECT RAISE(ABORT,'stop receipt commit failed'); END;`)
				}
				identity := compatibilityRecord(t, dir).Snapshot.Ref.External
				writeCommand(t, dir, "stop", id, "stop-before-upgrade", before.Revision)
				if state == "stop_pending" {
					before = waitRun(t, dir, id, func(r contracts.RunView) bool {
						return process.InspectProcess(&identity) == "process absent" && r.StopStatus == "pending" && len(r.Operations) > len(before.Operations)
					})
				} else {
					before = waitRun(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" })
				}
			}
			page := call(t, dir, []string{"events", "--run-id", id, "--limit", "1"}, "", 0)
			starts, threads := countNative(t, "turn/start"), countNative(t, "thread/start")
			call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("stop-baseline", originalService.InstanceID), 0)
			if err := waitOwnedService(originalService); err != nil {
				t.Fatal(err)
			}
			retainCompatibilityDB(t, dir, state+"-baseline-v2.db")
			original := compatibilityRecord(t, dir)
			// The previous worker may reconcile between the first inspection and
			// shutdown. Compare the final durable baseline after its owner exited,
			// rather than attributing that legitimate update to the new EXE.
			before = original.View()
			expectedOperations := append([]contracts.OperationView(nil), before.Operations...)
			if state == "stop_pending" {
				if original.PendingStop == "" || len(original.Intents) != len(before.Operations) || original.Intents[len(original.Intents)-1].Request.Operation.Status != b.OperationInFlight {
					t.Fatal("baseline did not retain its uncommitted stop outcome")
				}
				compatibilitySQL(t, dir, `DROP TRIGGER fail_stop_receipt`)
				expectedOperations[len(expectedOperations)-1].Status = string(b.OperationConfirmed)
			}
			runnerEXE = currentEXE
			// A second current-version start verifies that loading and writing the
			// old aggregate did not lose its original associations or authorization.
			for _, restart := range []string{"first", "second"} {
				service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				cleanupService(t, dir, service)
				after := waitRun(t, dir, id, func(r contracts.RunView) bool {
					return r.ResumeRequired && (state != "stop_pending" || r.StopStatus == "stopped" && reflect.DeepEqual(r.Operations, expectedOperations))
				})
				if after.NewInputsAllowed || after.RunID != before.RunID || after.InputDigest != before.InputDigest || after.ResourceDigest != before.ResourceDigest || !reflect.DeepEqual(after.Materials, before.Materials) || !reflect.DeepEqual(after.PR, before.PR) || !reflect.DeepEqual(after.Final, before.Final) {
					t.Fatalf("%s: old v2 facts or input revocation changed", restart)
				}
				if after.Execution.AttemptID != before.Execution.AttemptID || after.Execution.Generation != before.Execution.Generation || after.Execution.SessionID != before.Execution.SessionID || after.Execution.TurnID != before.Execution.TurnID || after.Execution.Branch != before.Execution.Branch || after.Execution.Workspace != before.Execution.Workspace || !reflect.DeepEqual(after.Operations, expectedOperations) {
					t.Fatalf("%s: original operation or execution binding changed", restart)
				}
				retry := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
				if retry.Data.(map[string]any)["run_id"] != id {
					t.Fatal("old request receipt created another Run")
				}
				currentPage := call(t, dir, []string{"events", "--run-id", id, "--limit", "1"}, "", 0)
				if !reflect.DeepEqual(currentPage.Data, page.Data) {
					t.Fatal("existing event or cursor changed after upgrade")
				}
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("stop-current-"+restart, service.InstanceID), 0)
				if err := waitOwnedService(service); err != nil {
					t.Fatal(err)
				}
			}
			if countNative(t, "turn/start") != starts || countNative(t, "thread/start") != threads {
				t.Fatal("upgrade replayed an original input or created a replacement session")
			}
			retainCompatibilityDB(t, dir, state+"-current-v2.db")
			if state == "stop_pending" {
				settled := compatibilityRecord(t, dir)
				index := len(original.Intents) - 1
				// Compare the original durable request, including input, digest, key
				// and full binding. Only the outcome is allowed to change.
				want := original.Intents[index].Request
				want.Operation.Status = b.OperationConfirmed
				if len(settled.Intents) != len(original.Intents) || !reflect.DeepEqual(settled.Intents[index].Request, want) || settled.PendingStop != "" || settled.Intents[index].Receipt == nil || !settled.Intents[index].Receipt.State.WritersAbsent {
					t.Fatal("upgraded stop replaced its original request or failed to prove writer absence")
				}
			}
		})
	}
}

func compatibilitySQL(t *testing.T, dir, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(statement); err != nil {
		t.Fatal(err)
	}
}

func compatibilityRecord(t *testing.T, dir string) runstate.Record {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var body []byte
	if err := db.QueryRow(`SELECT body FROM runs`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var record runstate.Record
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func retainCompatibilityDB(t *testing.T, dir, name string) {
	t.Helper()
	if *evidenceFlag == "" {
		return
	}
	// Both the previous and current owners have been proven exited before copy.
	body, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*evidenceFlag, name), body, 0600); err != nil {
		t.Fatal(err)
	}
}
