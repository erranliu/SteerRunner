//go:build windows

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRMultipleRunsIsolation(t *testing.T) {
	testCount.Add(1)
	dir, envelope := setupRun(t)
	t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
	state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	hold := filepath.Join(state, "hold-turn")
	if err := os.WriteFile(hold, []byte("hold both original implementation turns"), 0600); err != nil {
		t.Fatal(err)
	}
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	var input contracts.RunRequest
	if err := json.Unmarshal(envelope.Input, &input); err != nil {
		t.Fatal(err)
	}
	input.Publication.Settings, _ = json.Marshal(contracts.CommandProviderSettings{BaseURL: "https://controlled.invalid", Query: contracts.QuerySettings{Executable: helperEXE, Args: []string{"--query", os.Getenv("STEERRUNNER_FIXTURE_PROVIDER"), "{pr_url}"}}})
	var ids [2]string
	var submitted [2]string
	for i := range ids {
		envelope.RequestID = fmt.Sprintf("multi-submit-%d", i)
		input.Title = fmt.Sprintf("peer %d", i)
		envelope.Input, _ = json.Marshal(input)
		raw, _ := json.Marshal(envelope)
		submitted[i] = string(raw)
		ids[i] = call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	}
	if ids[0] == ids[1] {
		t.Fatal("different requests collided")
	}
	var before [2]contracts.RunView
	for i, id := range ids {
		before[i] = waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.SessionID != "" && v.Execution.TurnID != "" })
		retry := call(t, dir, []string{"submit", "--input", "-"}, submitted[i], 0)
		if retry.Data.(map[string]any)["run_id"] != id {
			t.Fatal("retry crossed Run identity")
		}
	}
	if before[0].Execution.SessionID == before[1].Execution.SessionID || before[0].Execution.Workspace == before[1].Execution.Workspace || before[0].Execution.Branch == before[1].Execution.Branch {
		t.Fatal("concurrent native resources collided")
	}
	revokeWithFreshRevision(t, dir, ids[0], before[0].Revision)
	waitRun(t, dir, ids[0], func(v contracts.RunView) bool { return v.StopStatus == "stopped" && !v.NewInputsAllowed })
	peer := waitRun(t, dir, ids[1], func(contracts.RunView) bool { return true })
	if !peer.NewInputsAllowed || peer.StopStatus == "stopped" || peer.Execution.SessionID != before[1].Execution.SessionID {
		t.Fatal("stopping first Run changed peer authority")
	}
	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	ready := waitRun(t, dir, ids[1], func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	if ready.PR == nil || ready.PR.HeadBranch != before[1].Execution.Branch || ready.Execution.SessionID != before[1].Execution.SessionID {
		t.Fatal("peer did not finish original workflow")
	}
	first := waitRun(t, dir, ids[0], func(contracts.RunView) bool { return true })
	if first.PR != nil || first.NewInputsAllowed {
		t.Fatal("stopped Run consumed peer publication or regained authority")
	}
	for _, id := range ids {
		res := call(t, dir, []string{"events", "--run-id", id}, "", 0)
		raw, _ := json.Marshal(res.Data)
		var page contracts.EventPage
		if json.Unmarshal(raw, &page) != nil || len(page.Events) == 0 {
			t.Fatal("events missing")
		}
		for _, event := range page.Events {
			if event.RunID != id {
				t.Fatal("event crossed Run boundary")
			}
		}
	}
	if countNative(t, "thread/start") != 2 || countNative(t, "turn/start") != 4 {
		t.Fatal("retry or stop duplicated native inputs")
	}
	revokeWithFreshRevision(t, dir, ids[1], ready.Revision)
	waitRun(t, dir, ids[1], func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
}
