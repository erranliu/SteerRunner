//go:build windows

package integration

import (
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSRStabilityLostSubmitResponse(t *testing.T) {
	testCount.Add(1)
	dir, req := setupRun(t)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(req)
	// Deliberately discard the accepted submit response; recover only by public request identity.
	call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	r := call(t, dir, []string{"inspect", "--request-id", req.RequestID}, "", 0)
	body, _ := json.Marshal(r.Data)
	var data struct {
		Run *contracts.RunView `json:"run"`
	}
	_ = json.Unmarshal(body, &data)
	if data.Run == nil {
		t.Fatal("accepted request cannot be recovered")
	}
	replay := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	if replay.Data.(map[string]any)["run_id"] != data.Run.RunID {
		t.Fatal("lost response produced replacement Run")
	}
	waitRun(t, dir, data.Run.RunID, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	if countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 3 {
		t.Fatal("request reconciliation resent native input")
	}
}
func TestSRStabilityRevocationAcrossRestart(t *testing.T) {
	testCount.Add(1)
	dir, req := setupRun(t)
	_ = os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "drop-next-start"), []byte("unknown native receipt"), 0600)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(req)
	r := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	id := r.Data.(map[string]any)["run_id"].(string)
	v := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "reconciling" })
	revokeWithFreshRevision(t, dir, id, v.Revision)
	v = waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" && !v.NewInputsAllowed })
	count := countNative(t, "turn/start")
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("stability-service-stop", info.InstanceID), 0)
	if err := waitOwnedService(info); err != nil {
		t.Fatal(err)
	}
	next := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, next)
	after := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.ResumeRequired })
	if after.NewInputsAllowed {
		t.Fatal("service restart restored revoked authority")
	}
	time.Sleep(1100 * time.Millisecond)
	if countNative(t, "turn/start") != count {
		t.Fatal("revoked input was dispatched after restart")
	}
}

func revokeWithFreshRevision(t *testing.T, dir, id string, revision uint64) {
	t.Helper()
	for attempt := 0; attempt < 5; attempt++ {
		input, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: revision})
		raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: fmt.Sprintf("stability-revoke-%d", attempt), Input: input})
		response, exit, diagnostics, err := invoke(runnerEXE, []string{"stop", "--run-id", id, "--input", "-", "--state-dir", dir, "--json"}, string(raw))
		if err == nil && exit == 0 && response.OK {
			return
		}
		if err != nil || response.OK || response.Error == nil || response.Error.Code != "stale_revision" {
			body, _ := json.Marshal(response)
			t.Fatalf("stop was not explicitly rejected for stale revision; not retried: exit=%d error=%v response=%s stderr=%s", exit, err, body, diagnostics)
		}
		t.Logf("stop explicitly rejected stale_revision at revision %d; refresh before a new request", revision)
		revision = waitRun(t, dir, id, func(contracts.RunView) bool { return true }).Revision
	}
	t.Fatal("stop version conflict did not settle within five explicit rejections")
}
