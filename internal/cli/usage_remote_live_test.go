package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/cost"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage/access"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

// Explicit opt-in, existing completed evidence only. The copied Record has no
// business intents, authorization or controls. Only the isolated usage worker
// may touch Orca; it uses the actual EXE's public policy/query/stop interfaces.
func TestRunUsageRemotePublicLive(t *testing.T) {
	if os.Getenv("SR_USAGE_LIVE_PUBLIC") != "1" {
		t.Skip("explicit remote public acceptance only")
	}
	source, id, root := os.Getenv("SR_USAGE_LIVE_STATE"), os.Getenv("SR_USAGE_LIVE_RUN"), os.Getenv("SR_USAGE_LIVE_EVIDENCE")
	if source == "" || id == "" || root == "" {
		t.Fatal("explicit source, Run and isolated evidence directory required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	reader, err := storage.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := reader.UsageRuns(ctx)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	var original *runstate.Record
	for _, r := range runs {
		if r.ID == id {
			original = r
		}
	}
	if original == nil || original.Status != "pr_ready" {
		t.Fatal("requires original completed Run")
	}
	before, _ := json.Marshal(original)
	binding, p, _, err := orca.UsageBinding(original.Profile, original.Snapshot, orca.UsageInitialization{LaunchCommand: original.LaunchCommand, CreatedAt: original.CreatedAt})
	if err != nil || p.Environment == "" || binding.Agent != "codex" {
		t.Fatal("requires bound remote Codex", err)
	}
	host, err := native.UsageHostID(p.Environment)
	if err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, p.Args...), "status", "--json", "--environment", p.Environment)
	raw, err := native.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, p.Executable, args...).Output()
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := native.ParseEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := native.OpenUsageFiles(ctx, p.Environment, envelope.Meta.RuntimeID, host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { channel.Close() })
	groups, err := channel.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	folders, err := channel.Folders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := channel.Sessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	logRoot := ""
	for _, s := range sessions {
		if s.SessionID == binding.SessionID && s.Agent == binding.Agent && parsers.NormalizePath(s.CWD) == parsers.NormalizePath(binding.CWD) {
			if logRoot != "" {
				t.Fatal("ambiguous source")
			}
			logRoot = filepath.Dir(s.FilePath)
		}
	}
	if logRoot == "" {
		t.Fatal("missing source")
	}
	state := filepath.Join(root, "state")
	if err = os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	copy := *original
	copy.Revision = 1
	if err = store.InitUsage(); err == nil {
		err = store.Commit(&copy, 0, nil)
	}
	store.Close()
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "runner.exe")
	if raw, err = exec.Command("go", "build", "-o", exe, "../../cmd/steerrunner").CombinedOutput(); err != nil {
		t.Fatal(err, string(raw))
	}
	call := func(input string, args ...string) contracts.Response {
		t.Helper()
		bounded, done := context.WithTimeout(context.Background(), 40*time.Second)
		defer done()
		cmd := exec.CommandContext(bounded, exe, append(args, "--state-dir", state, "--json")...)
		cmd.Stdin = bytes.NewBufferString(input)
		body, e := cmd.Output()
		if e != nil {
			t.Fatalf("public command %v: %v", args, e)
		}
		var response contracts.Response
		if e = json.Unmarshal(body, &response); e != nil || !response.OK {
			t.Fatalf("public command %v invalid response: %s", args, body)
		}
		return response
	}
	decode := func(response contracts.Response, dst any) {
		t.Helper()
		body, _ := json.Marshal(response.Data)
		if err := json.Unmarshal(body, dst); err != nil {
			t.Fatal(err)
		}
	}
	policy := api.DefaultPolicy()
	policy.RemoteLogAccess.Mode = "temporary"
	policy.RemoteLogAccess.Grants = []api.Grant{{Environment: p.Environment, HostID: host, SessionRoots: []string{logRoot}}}
	body, _ := json.Marshal(contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{}, UsagePolicy: &policy})
	var info contracts.ServiceInfo
	decode(call(string(body), "service", "start", "--input", "-"), &info)
	t.Cleanup(func() {
		stop, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-remote-usage-live", Input: json.RawMessage(`{"expected_instance_id":"` + info.InstanceID + `"}`)})
		var current contracts.ServiceInfo
		decode(call("", "service", "status"), &current)
		if current.Status != "not_running" {
			call(string(stop), "service", "stop", "--input", "-")
		}
		until := time.Now().Add(30 * time.Second)
		for {
			var status contracts.ServiceInfo
			decode(call("", "service", "status"), &status)
			if status.Status == "not_running" {
				break
			}
			if time.Now().After(until) {
				t.Error("isolated service did not exit")
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		clean, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		afterGroups, e1 := channel.Groups(clean)
		afterFolders, e2 := channel.Folders(clean)
		if e1 != nil || e2 != nil || !reflect.DeepEqual(groups, afterGroups) || !reflect.DeepEqual(folders, afterFolders) {
			t.Errorf("remote baseline changed: %v/%v", e1, e2)
		}
		reader, e := storage.OpenReader(state)
		if e != nil {
			t.Error(e)
			return
		}
		defer reader.Close()
		var permit api.CollectionPermit
		if e = reader.UsageJournal(clean, api.CollectionPermitKey, &permit); e != nil || permit.Active {
			t.Error("stopped isolated service retained active collection permit", e)
		}
		keys, e := reader.UsageJournalKeys(clean, "lease:")
		if e != nil || len(keys) != 1 {
			t.Error("expected one automatic lease", e, keys)
		}
		copiedRuns, e := reader.UsageRuns(clean)
		if e != nil || len(copiedRuns) != 1 || copiedRuns[0].Revision != 1 || copiedRuns[0].Status != "pr_ready" {
			t.Error("usage worker changed copied business Run", e)
		}
		for _, key := range keys {
			var lease access.Lease
			if e = reader.UsageJournal(clean, key, &lease); e != nil {
				t.Error(e)
				continue
			}
			if lease.Phase != "revoked" || !channel.SelectorRevoked(clean, lease.FolderID, lease.File) {
				t.Error("registration not revoked", lease.Phase)
			}
			t.Logf("public cleanup epoch=%d phase=%s group=%s folder=%s", lease.Epoch, lease.Phase, lease.GroupID, lease.FolderID)
		}
		sourceReader, e := storage.OpenReader(source)
		if e != nil {
			t.Error(e)
			return
		}
		defer sourceReader.Close()
		originalRuns, e := sourceReader.UsageRuns(clean)
		if e != nil {
			t.Error(e)
			return
		}
		for _, r := range originalRuns {
			if r.ID == id {
				// The original production service can refresh PR facts while this
				// read-only probe is running. A revision alone is not a binding
				// change; every source identity/input below must remain identical.
				if r.Revision != original.Revision {
					t.Logf("independent source service revision advanced %d -> %d", original.Revision, r.Revision)
				}
				r.Revision = original.Revision
				after, _ := json.Marshal(r)
				if !bytes.Equal(before, after) {
					t.Error("source Run evidence changed")
				}
			}
		}
	})
	var snapshot api.Snapshot
	until := time.Now().Add(45 * time.Second)
	for {
		decode(call("", "run-usage", "--run-id", id), &snapshot)
		if snapshot.Status == "available" && snapshot.Coverage.Finalized && snapshot.AccessCleanupStatus == "revoked" {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("public collector did not settle: %+v", snapshot)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if snapshot.Tokens == nil || snapshot.Tokens.Total == nil || *snapshot.Tokens.Total != 225453 || snapshot.Coverage.CountedResponses != 11 {
		t.Fatal(snapshot)
	}
	revision := strconv.FormatUint(snapshot.Revision, 10)
	events := []api.Event{}
	cursor := ""
	for {
		args := []string{"run-usage-events", "--run-id", id, "--snapshot-revision", revision, "--limit", "3"}
		if cursor != "" {
			args = append(args, "--after", cursor)
		}
		var page api.Page
		decode(call("", args...), &page)
		events = append(events, page.Events...)
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(events) != 11 {
		t.Fatal("wrong public event count", len(events))
	}
	pricing := cost.Policy{Schema: "run-pricing-policy/v1", ID: "live-arithmetic-only", Version: "1", Source: "synthetic rates; not provider prices", Currency: "USD", Enabled: true, AllowCompletionTime: true}
	seen := map[string]bool{}
	initialization := 0
	for _, event := range events {
		if event.Phase == "initialization" {
			initialization++
			if event.CommandID != "" {
				t.Fatal("initialization given business command")
			}
		}
		if event.Provider == nil || event.Model == nil {
			t.Fatal("native metadata unavailable")
		}
		key := *event.Provider + "/" + *event.Model
		if seen[key] {
			continue
		}
		seen[key] = true
		pricing.Rules = append(pricing.Rules, cost.Rule{Provider: *event.Provider, Model: *event.Model, From: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Rates: cost.Rates{Input: "1", CacheRead: "0.1", CacheWrite: "0", Output: "2"}})
	}
	if initialization != 1 {
		t.Fatal("initialization phase count", initialization)
	}
	pricePath := filepath.Join(root, "synthetic-prices.json")
	body, _ = json.MarshalIndent(pricing, "", "  ")
	if err = os.WriteFile(pricePath, body, 0600); err != nil {
		t.Fatal(err)
	}
	var estimate cost.Estimate
	decode(call("", "estimate-run-cost", "--run-id", id, "--usage-revision", revision, "--pricing-policy", pricePath), &estimate)
	if estimate.Status != "estimated" || estimate.Total == nil || *estimate.Total != "0.05327400" || estimate.Priced != 11 {
		t.Fatal(estimate)
	}
	for name, value := range map[string]any{"usage": snapshot, "events": events, "cost": estimate} {
		body, _ = json.MarshalIndent(value, "", "  ")
		if err = os.WriteFile(filepath.Join(root, name+".json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var again api.Snapshot
	decode(call("", "run-usage", "--run-id", id, "--snapshot-revision", revision), &again)
	if !reflect.DeepEqual(snapshot, again) {
		t.Fatal("same revision changed")
	}
	// Withdraw the temporary local permit through the existing stop/start
	// contract too. A disabled replacement serves immutable history without
	// reopening remote directories or changing the business Run.
	stop, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "withdraw-usage-live-policy", Input: json.RawMessage(`{"expected_instance_id":"` + info.InstanceID + `"}`)})
	call(string(stop), "service", "stop", "--input", "-")
	until = time.Now().Add(30 * time.Second)
	for {
		var current contracts.ServiceInfo
		decode(call("", "service", "status"), &current)
		if current.Status == "not_running" {
			break
		}
		if time.Now().After(until) {
			t.Fatal("policy withdrawal stop did not exit")
		}
		time.Sleep(100 * time.Millisecond)
	}
	withdrawn := api.DefaultPolicy()
	withdrawn.Enabled = false
	body, _ = json.Marshal(contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{}, UsagePolicy: &withdrawn})
	decode(call(string(body), "service", "start", "--input", "-"), &info)
	decode(call("", "run-usage", "--run-id", id, "--snapshot-revision", revision), &again)
	if !reflect.DeepEqual(snapshot, again) {
		t.Fatal("withdrawing permit changed historical snapshot")
	}
	body, _ = json.MarshalIndent(withdrawn, "", "  ")
	if err = os.WriteFile(filepath.Join(root, "withdrawn-policy.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("public Run=%s revision=%d responses=%d total=%d synthetic_USD=%s", id, snapshot.Revision, len(events), *snapshot.Tokens.Total, *estimate.Total)
}
