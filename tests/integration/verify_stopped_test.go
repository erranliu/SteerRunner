//go:build windows

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
	for until := time.Now().Add(5 * time.Second); ; {
		response = call(t, dir, []string{"service", "verify-stopped"}, "", 0)
		raw, _ = json.Marshal(response.Data)
		if err = json.Unmarshal(raw, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.ResourcesSettled {
			break
		}
		if time.Now().After(until) {
			t.Fatal("exact original managed writers did not settle", proof)
		}
		time.Sleep(30 * time.Millisecond)
	}
	if len(proof.Runs) != 1 || proof.Runs[0].RunID != id || !proof.Runs[0].ResourcesSettled {
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
