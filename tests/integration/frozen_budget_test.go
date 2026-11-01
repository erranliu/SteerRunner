//go:build windows

package integration

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// Corrupt only this stopped writer's isolated v8 fixture. Original native
// intent payloads retain their bytes; public reads, replay and stop must work.
func TestSRFrozenBudgetRecovery(t *testing.T) {
	testCount.Add(1)
	for _, mode := range []string{"accepted", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			dir, request := setupRun(t)
			marker := "hold-turn"
			if mode == "unknown" {
				marker = "drop-next-start"
			}
			if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), marker), []byte("isolated budget boundary"), 0600); err != nil {
				t.Fatal(err)
			}
			info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, info)
			raw, _ := json.Marshal(request)
			id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
			waitRun(t, dir, id, func(r contracts.RunView) bool {
				if mode == "unknown" {
					return r.Phase == "operation_outcome_unknown"
				}
				return r.Execution.TurnStarted && r.Execution.TurnID != ""
			})
			call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("before-budget-damage", info.InstanceID), 0)
			if err := waitOwnedService(info); err != nil {
				t.Fatal(err)
			}
			r := compatibilityRecordByID(t, dir, id)
			if len(r.Workflow.StageInputs) != 1 || r.Workflow.StageInputs[0].Budget == nil {
				t.Fatal("original production input lacks budget")
			}
			for n := range r.Workflow.StageInputs {
				r.Workflow.StageInputs[n].Budget = nil
			}
			if r.Snapshot.PendingTurn != nil {
				r.Snapshot.PendingTurn.Budget = nil
			}
			body, _ := json.Marshal(r)
			db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`UPDATE runs SET body=? WHERE id=?`, body, id)
			_ = db.Close()
			if err != nil {
				t.Fatal(err)
			}
			info = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, info)
			current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Revision > 0 })
			call(t, dir, []string{"events", "--run-id", id}, "", 0)
			if replay := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0); replay.Data.(map[string]any)["run_id"] != id {
				t.Fatal("missing budget changed request replay", replay)
			}
			call(t, dir, []string{"inspect", "--request-id", request.RequestID}, "", 0)
			current = waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Revision > 0 })
			writeCommand(t, dir, "stop", id, "stop-missing-budget", current.Revision)
			waitRun(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" })
			time.Sleep(300 * time.Millisecond)
			if countNative(t, "turn/start") != 1 {
				t.Fatal("missing budget replay/restart/stop resent business input")
			}
			after := compatibilityRecordByID(t, dir, id)
			if after.Workflow.StageInputs[0].Budget != nil {
				t.Fatal("query or stop reconstructed budget")
			}
			for _, original := range r.Intents {
				found := false
				for _, saved := range after.Intents {
					if saved.Request.Operation.ID == original.Request.Operation.ID {
						found = true
						if string(saved.Request.Operation.Request) != string(original.Request.Operation.Request) || saved.Request.Operation.PayloadDigest != original.Request.Operation.PayloadDigest {
							t.Fatal("recovery rewrote original intent")
						}
					}
				}
				if !found {
					t.Fatal("original intent lost")
				}
			}
		})
	}
}
