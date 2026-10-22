package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/cost"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestRunUsagePublicExecutable(t *testing.T) {
	if runtime.GOOS != "windows" || testing.Short() {
		t.Skip("Windows executable/pipe integration")
	}
	root := t.TempDir()
	exe := filepath.Join(root, "runner.exe")
	build := exec.Command("go", "build", "-o", exe, "../../cmd/steerrunner")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatal(err, string(raw))
	}
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.InitUsage(); err != nil {
		t.Fatal(err)
	}
	run := &runstate.Record{ID: "fixture-run", Revision: 1, Status: "pr_ready", Snapshot: b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1}}}
	if err = s.Commit(run, 0, nil); err != nil {
		t.Fatal(err)
	}
	model, provider := "fixture", "synthetic"
	when := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	event := api.Event{ID: "fixture-event", RunID: run.ID, SourceRef: "fixture-source", SessionID: "fixture-session", ResponseID: "response", Decision: "counted", Model: &model, Provider: &provider, MetadataBasis: "observed", CallTime: &when, TimeBasis: "request", Tokens: api.Tokens{InputTotal: api.Number(100), InputUncached: api.Number(100), CacheRead: api.Number(0), CacheWrite: api.Number(0), Output: api.Number(20), Total: api.Number(120)}}
	snapshot, err := s.CommitUsage(context.Background(), api.Snapshot{RunID: run.ID, AccessCleanupStatus: "not_required", Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, Finalized: true}}, []api.Event{event})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	bound := ""
	call := func(input string, args ...string) (contracts.Response, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		if bound != "" && contracts.ServiceBindingCommand(args[0]) {
			args = append(args, "--expected-binding", bound)
		}
		cmd := exec.CommandContext(ctx, exe, append(args, "--state-dir", state, "--json")...)
		cmd.Stdin = bytes.NewBufferString(input)
		raw, err := cmd.Output()
		exit := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				exit = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		var response contracts.Response
		if err = json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err, string(raw))
		}
		return response, exit
	}
	input := `{"pending_stops":[],"usage_policy":{"enabled":false,"remote_log_access":{"mode":"disabled","grants":[]}}}`
	started, exit := call(input, "service", "start", "--input", "-")
	if exit != 0 {
		t.Fatal(started)
	}
	raw, _ := json.Marshal(started.Data)
	var info contracts.ServiceInfo
	_ = json.Unmarshal(raw, &info)
	fullBinding, _ := json.Marshal(contracts.ExpectedServiceBinding{Contract: contracts.ServiceBindingContract, Protocol: info.Protocol, InstanceID: info.InstanceID, StateDir: info.StateDir, ExecutableSHA256: info.ExecutableSHA256, Version: info.Version, ResourceDigest: info.ResourceDigest})
	bound = string(fullBinding)
	t.Cleanup(func() {
		body, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-usage-test", Input: json.RawMessage(`{"expected_instance_id":"` + info.InstanceID + `"}`)})
		_, _ = call(string(body), "service", "stop", "--input", "-")
		for i := 0; i < 100; i++ {
			response, _ := call("", "service", "status")
			raw, _ := json.Marshal(response.Data)
			var status contracts.ServiceInfo
			_ = json.Unmarshal(raw, &status)
			if status.Status == "not_running" {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Error("isolated service did not exit")
	})
	response, exit := call("", "run-usage", "--run-id", run.ID, "--snapshot-revision", "1")
	if exit != 0 {
		t.Fatal(response)
	}
	raw, _ = json.Marshal(response.Data)
	var got api.Snapshot
	_ = json.Unmarshal(raw, &got)
	if got.Revision != snapshot.Revision || got.Tokens == nil || *got.Tokens.Total != 120 {
		t.Fatal(got)
	}
	response, exit = call("", "run-usage-events", "--run-id", run.ID, "--snapshot-revision", "1", "--limit", "1")
	if exit != 0 {
		t.Fatal(response)
	}
	response, exit = call("", "run-usage", "--run-id", "missing")
	if exit != 5 || response.Error.Code != "run_not_found" {
		t.Fatal(response, exit)
	}
	response, exit = call("", "run-usage", "--run-id", run.ID, "--snapshot-revision", "999")
	if exit != 5 || response.Error.Code != "snapshot_not_found" {
		t.Fatal(response, exit)
	}
	response, exit = call("", "estimate-run-cost", "--run-id", run.ID, "--usage-revision", "1", "--pricing-policy", filepath.Join(root, "absent.json"))
	if exit != 0 {
		t.Fatal("bad policy blocked usage service", response)
	}
	policy := cost.Policy{Schema: "run-pricing-policy/v1", ID: "test", Version: "1", Source: "synthetic arithmetic fixture", Currency: "USD", Enabled: true, Rules: []cost.Rule{{Provider: provider, Model: model, From: when.Add(-time.Hour), Until: when.Add(time.Hour), Rates: cost.Rates{Input: "2", CacheRead: "0", CacheWrite: "0", Output: "4"}}}}
	raw, _ = json.Marshal(policy)
	file := filepath.Join(root, "prices.json")
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	response, exit = call("", "estimate-run-cost", "--run-id", run.ID, "--usage-revision", "1", "--pricing-policy", file)
	if exit != 0 {
		t.Fatal(response)
	}
	raw, _ = json.Marshal(response.Data)
	var estimate cost.Estimate
	_ = json.Unmarshal(raw, &estimate)
	if estimate.Status != "estimated" || estimate.Total == nil || *estimate.Total != "0.00028000" {
		t.Fatal(estimate)
	}
	response, exit = call(input, "service", "start", "--input", "-")
	if exit != 0 {
		t.Fatal("same policy rejected", response)
	}
	response, exit = call(`{"pending_stops":[]}`, "service", "start", "--input", "-")
	if exit != 0 {
		t.Fatal("omitted policy replaced running policy", response)
	}
	response, exit = call(`{"pending_stops":[],"usage_policy":{"enabled":true,"remote_log_access":{"mode":"disabled","grants":[]}}}`, "service", "start", "--input", "-")
	if exit != 5 || response.Error.Code != "usage_policy_conflict" {
		t.Fatal(response, exit)
	}
}
