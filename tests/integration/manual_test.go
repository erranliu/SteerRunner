//go:build windows

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// The native input is accepted before its response is visible. Kill the service
// while its independent owner is pending: the new service has no observer.
// All controls cross CLI/IPC; no database or owner receipt is fabricated.
func TestSRManualReconciliation(t *testing.T) {
	testCount.Add(1)
	t.Setenv("STEERRUNNER_CONTENT_MARKER", "hold-turn-response")
	dir, id, service := contentFixture(t, "conversation", nil)
	hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn-response")
	t.Cleanup(func() { _ = os.Remove(hold) })
	until := time.Now().Add(time.Minute)
	for !existsTest(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl")) || countNative(t, "turn/start") != 1 {
		if time.Now().After(until) {
			t.Fatal("original native input not accepted")
		}
		time.Sleep(20 * time.Millisecond)
	}
	owner := contentProcess(t, dir, id)
	if err := killOwnedService(service); err != nil {
		t.Fatal(err)
	}
	next := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, next)
	paused := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.Phase == "manual_intervention_required" }, 45*time.Second)
	if !paused.Controls.CanResume || !paused.ResumeRequired || paused.NewInputsAllowed {
		t.Fatal("unknown operation lacks public manual exit", paused)
	}
	failed := contentCommand(t, dir, id, "resume", "manual-unresolved", map[string]any{}, 0)
	waitRunFor(t, dir, id, func(v contracts.RunView) bool {
		return v.Controls.CanResume && v.Phase == "manual_intervention_required"
	}, 40*time.Second)
	replay := call(t, dir, []string{"resume", "--run-id", id, "--input", "-"}, failed, 0)
	if replay.Data.(map[string]any)["action_status"] != "failed" {
		t.Fatal("original failed resume not replayed", replay)
	}
	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	// The original owner finishes the same RPC; the next explicit resume queries
	// that receipt. Polling while paused may not grant input authority.
	time.Sleep(300 * time.Millisecond)
	if v := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0)); v.NewInputsAllowed || !v.ResumeRequired {
		t.Fatal("late receipt resumed automatically", v)
	}
	succeeded := contentCommand(t, dir, id, "resume", "manual-resolved", map[string]any{}, 0)
	ready := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage }, time.Minute)
	if ready.ResumeRequired || contentProcess(t, dir, id) != owner || countNative(t, "turn/start") != 1 || countNative(t, "thread/start") != 1 || ready.ContentResult == nil || !ready.ContentResult.Source.Verified {
		t.Fatal("manual reconciliation replaced or resent original input", ready)
	}
	replay = call(t, dir, []string{"resume", "--run-id", id, "--input", "-"}, succeeded, 0)
	if replay.Data.(map[string]any)["action_status"] != "completed" {
		t.Fatal("completed manual resume did not replay", replay)
	}
	contentCommand(t, dir, id, "message", "manual-next", map[string]any{"text": "next unique input"}, 0)
	waitRunFor(t, dir, id, func(v contracts.RunView) bool {
		return v.Controls.CanMessage && v.ContentResult != nil && v.ContentResult.Source.CommandID != ready.ContentResult.Source.CommandID
	}, time.Minute)
	if countNative(t, "turn/start") != 2 || countNative(t, "thread/start") != 1 {
		t.Fatal("successor was duplicated")
	}
	contentCommand(t, dir, id, "close", "manual-close", map[string]any{}, 0)
	waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" }, time.Minute)
}
