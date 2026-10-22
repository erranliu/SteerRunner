package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

// Explicit existing-evidence opt-in. The actual service discovers its source
// from the original Run; no path or response IDs are supplied to the collector.
func TestRunUsageLocalCodexPublicLive(t *testing.T) { testRunUsageLocalPublicLive(t, "codex") }
func TestRunUsageLocalPiPublicLive(t *testing.T)    { testRunUsageLocalPublicLive(t, "pi") }
func TestRunUsageLocalDSHPublicLive(t *testing.T)   { testRunUsageLocalPublicLive(t, "dsh") }

func testRunUsageLocalPublicLive(t *testing.T, agent string) {
	if os.Getenv("SR_USAGE_LOCAL_LIVE") != "1" {
		t.Skip("explicit existing local Orca evidence only")
	}
	source, id, root := os.Getenv("SR_USAGE_LOCAL_STATE"), os.Getenv("SR_USAGE_LOCAL_RUN"), os.Getenv("SR_USAGE_LOCAL_EVIDENCE")
	if source == "" || id == "" || !filepath.IsAbs(root) {
		t.Fatal("absolute new evidence directory, source and Run required")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("evidence directory must be new")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	readOriginal := func() *runstate.Record {
		t.Helper()
		s, err := storage.OpenReader(source)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		// A current stopped Run needs its full frozen execution identity for
		// reconstruction. Historical idle Runs use the read-only projection.
		var runs []*runstate.Record
		if agent == "codex" {
			runs, err = s.Runs()
		} else {
			// Historical business fields are deliberately not migrated or copied.
			// UsageRuns reads only committed native identity and receipt evidence.
			runs, err = s.UsageRuns(ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			if r.ID == id {
				return r
			}
		}
		t.Fatal("original Run missing")
		return nil
	}
	original := readOriginal()
	if original.Status != "stopped" && original.Status != "pr_ready" && original.Status != "completed" {
		t.Fatal("requires settled original Run", original.Status)
	}
	binding, p, _, err := orca.UsageBinding(original.Profile, original.Snapshot, orca.UsageInitialization{LaunchCommand: original.LaunchCommand, CreatedAt: original.CreatedAt})
	if err != nil || p.Environment != "" || binding.Agent != agent {
		t.Fatal("requires original local Orca binding", err)
	}
	cliRaw, err := native.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, p.Executable, append(p.Args, "status", "--json")...).Output()
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := native.ParseEnvelope(cliRaw)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := native.OpenUsageFiles(ctx, "", envelope.Meta.RuntimeID, "local")
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	groups, err := channel.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	folders, err := channel.Folders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var locator any
	file := ""
	if agent == "codex" {
		sessions, err := channel.Sessions(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range sessions {
			if s.Agent == "codex" && s.SessionID == binding.SessionID && parsers.NormalizePath(s.CWD) == parsers.NormalizePath(binding.CWD) {
				if file != "" {
					t.Fatal("ambiguous original source")
				}
				file = s.FilePath
				locator = s
			}
		}
		if file == "" {
			t.Fatal("original session not located")
		}
	} else {
		if original.Status != "pr_ready" || p.Terminal == nil {
			t.Fatal("historical evidence requires an idle pr_ready Run with frozen terminal config")
		}
		pattern := filepath.Join(p.Terminal.StateDirectory, "sessions", "*"+binding.SessionID+".jsonl")
		if agent == "dsh" {
			pattern = filepath.Join(p.Terminal.StateDirectory, "home", ".dsh", "sessions", "*", binding.SessionID, "session.v4.jsonl.zstd")
		}
		files, e := filepath.Glob(pattern)
		if e != nil || len(files) != 1 {
			t.Fatal("original native source must be unique", len(files), e)
		}
		file = files[0]
		locator = map[string]any{"agent": agent, "session_id": binding.SessionID, "cwd": binding.CWD, "path": file, "commands": binding.Commands}
	}
	readControl := func() localNativeControl {
		if agent == "codex" {
			return readLocalCodexControl(t, file, binding)
		}
		return readLocalTerminalControl(t, file, binding)
	}
	control := readControl()
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	save := func(name string, v any) {
		t.Helper()
		body, e := json.MarshalIndent(v, "", "  ")
		if e == nil {
			e = os.WriteFile(filepath.Join(root, name+".json"), body, 0600)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	save("control", control)
	save("locator", locator)
	state := filepath.Join(root, "state")
	if err = os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	copy := *original
	copy.Revision = 1
	if copy.Allowed {
		t.Fatal("source still authorizes business execution")
	}
	if err = s.InitUsage(); err == nil {
		err = s.Commit(&copy, 0, nil)
	}
	s.Close()
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "runner.exe")
	if raw, e := exec.Command("go", "build", "-o", exe, "../../cmd/steerrunner").CombinedOutput(); e != nil {
		t.Fatal(e, string(raw))
	}
	call := func(input any, args ...string) contracts.Response {
		t.Helper()
		bounded, done := context.WithTimeout(context.Background(), 40*time.Second)
		defer done()
		cmd := exec.CommandContext(bounded, exe, append(args, "--state-dir", state, "--json")...)
		if input != nil {
			raw, _ := json.Marshal(input)
			cmd.Stdin = bytes.NewReader(raw)
		}
		raw, e := cmd.Output()
		if e != nil {
			t.Fatal("public call failed", args, e)
		}
		var response contracts.Response
		if json.Unmarshal(raw, &response) != nil || !response.OK {
			t.Fatalf("public %v rejected: %s", args, raw)
		}
		return response
	}
	decode := func(response contracts.Response, dst any) {
		t.Helper()
		raw, _ := json.Marshal(response.Data)
		if e := json.Unmarshal(raw, dst); e != nil {
			t.Fatal(e)
		}
	}
	var info contracts.ServiceInfo
	serial := 0
	start := func(enabled bool) {
		t.Helper()
		policy := api.DefaultPolicy()
		policy.Enabled = enabled
		decode(call(contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{}, UsagePolicy: &policy}, "service", "start", "--input", "-"), &info)
	}
	stop := func() {
		t.Helper()
		var status contracts.ServiceInfo
		decode(call(nil, "service", "status"), &status)
		if status.Status == "not_running" {
			return
		}
		serial++
		body, _ := json.Marshal(contracts.ServiceStopInput{ExpectedInstanceID: info.InstanceID})
		call(contracts.Request{Protocol: contracts.Protocol, RequestID: fmt.Sprintf("local-usage-stop-%d", serial), Input: body}, "service", "stop", "--input", "-")
		until := time.Now().Add(30 * time.Second)
		for {
			decode(call(nil, "service", "status"), &status)
			if status.Status == "not_running" {
				break
			}
			if time.Now().After(until) {
				t.Fatal("isolated service did not stop")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	t.Cleanup(func() { stop() })
	get := func() api.Snapshot { var v api.Snapshot; decode(call(nil, "run-usage", "--run-id", id), &v); return v }
	readPages := func(revision uint64) []api.Event {
		t.Helper()
		cursor := ""
		events := []api.Event{}
		for {
			args := []string{"run-usage-events", "--run-id", id, "--snapshot-revision", strconv.FormatUint(revision, 10), "--limit", "2"}
			if cursor != "" {
				args = append(args, "--after", cursor)
			}
			var page api.Page
			decode(call(nil, args...), &page)
			if page.Revision != revision {
				t.Fatal("mixed revision")
			}
			events = append(events, page.Events...)
			cursor = page.NextCursor
			if cursor == "" {
				return events
			}
		}
	}
	start(true)
	until := time.Now().Add(40 * time.Second)
	var snapshot api.Snapshot
	for {
		snapshot = get()
		if snapshot.Status == "available" && snapshot.Coverage.Finalized {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("collector did not settle: %+v", snapshot)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if snapshot.Coverage.ExpectedSources == nil || *snapshot.Coverage.ExpectedSources != 1 || snapshot.Coverage.VerifiedSources != 1 || snapshot.Coverage.MissingUsageResponses != 0 || len(snapshot.Issues) != 0 || snapshot.AccessCleanupStatus != "not_required" || snapshot.Tokens == nil || !reflect.DeepEqual(*snapshot.Tokens, control.Tokens) || snapshot.Coverage.CountedResponses != int64(len(control.Responses)) {
		t.Fatal("control sum mismatch", snapshot, control)
	}
	events := readPages(snapshot.Revision)
	nativeIDs := map[string]bool{}
	eventIDs := map[string]bool{}
	for _, event := range events {
		if event.RunID != id || event.Agent != agent || event.AttemptID != binding.AttemptID || event.Generation != binding.Generation || event.SessionID != binding.SessionID || event.Decision != "counted" || nativeIDs[event.ResponseID] || eventIDs[event.ID] {
			t.Fatal("foreign/duplicate event")
		}
		expected, ok := control.Responses[event.ResponseID]
		if !ok || !reflect.DeepEqual(expected, event.Tokens) {
			t.Fatal("native response mismatch", event.ResponseID)
		}
		if control.Commands != nil && (event.CommandID != control.Commands[event.ResponseID] || event.Phase != "command") {
			t.Fatal("native command ownership mismatch", event.ResponseID)
		}
		nativeIDs[event.ResponseID] = true
		eventIDs[event.ID] = true
	}
	if len(nativeIDs) != len(control.Responses) {
		t.Fatal("native responses omitted")
	}
	save("usage", snapshot)
	save("events", events)
	policy := cost.Policy{Schema: "run-pricing-policy/v1", ID: "local-native-arithmetic-only", Version: "1", Source: "synthetic rates, not supplier prices", Currency: "USD", Enabled: true, AllowCompletionTime: true}
	models := map[string]bool{}
	for _, e := range events {
		if e.Model == nil || e.Provider == nil {
			t.Fatal("native pricing metadata unavailable")
		}
		key := *e.Provider + "/" + *e.Model
		if !models[key] {
			models[key] = true
			policy.Rules = append(policy.Rules, cost.Rule{Provider: *e.Provider, Model: *e.Model, From: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Rates: cost.Rates{Input: "1", CacheRead: "0.1", CacheWrite: "0", Output: "2"}})
		}
	}
	save("synthetic-prices", policy)
	pricePath := filepath.Join(root, "synthetic-prices.json")
	var estimate cost.Estimate
	decode(call(nil, "estimate-run-cost", "--run-id", id, "--usage-revision", strconv.FormatUint(snapshot.Revision, 10), "--pricing-policy", pricePath), &estimate)
	numerator := *control.Tokens.InputUncached*10 + *control.Tokens.CacheRead + *control.Tokens.Output*20
	expectedAmount := fmt.Sprintf("%d.%08d", numerator/10000000, numerator%10000000*10)
	if estimate.Status != "estimated" || estimate.Total == nil || *estimate.Total != expectedAmount || estimate.Priced != int64(len(events)) {
		t.Fatal("independent arithmetic mismatch", estimate, expectedAmount)
	}
	save("cost", estimate)
	// A bad or disabled independent price policy cannot alter base usage.
	var bad cost.Estimate
	decode(call(nil, "estimate-run-cost", "--run-id", id, "--usage-revision", strconv.FormatUint(snapshot.Revision, 10), "--pricing-policy", filepath.Join(root, "missing.json")), &bad)
	if bad.Status != "unavailable" || bad.Total != nil || !reflect.DeepEqual(get(), snapshot) {
		t.Fatal("bad price policy changed usage")
	}
	policy.Enabled = false
	save("disabled-prices", policy)
	decode(call(nil, "estimate-run-cost", "--run-id", id, "--usage-revision", strconv.FormatUint(snapshot.Revision, 10), "--pricing-policy", filepath.Join(root, "disabled-prices.json")), &bad)
	if bad.Status != "unavailable" || !reflect.DeepEqual(get(), snapshot) {
		t.Fatal("disabled costs changed usage")
	}
	stop()
	start(true)
	time.Sleep(2500 * time.Millisecond)
	if !reflect.DeepEqual(get(), snapshot) || !reflect.DeepEqual(readPages(snapshot.Revision), events) {
		t.Fatal("graceful restart duplicated/rewrote usage")
	}
	stop()
	// Simulate loss of only the observation-finished cache in this isolated DB.
	// Preserve binding/prefix and ledger, then prove a real reread is idempotent.
	s, err = storage.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	var checkpoint map[string]any
	if err = s.UsageJournal(ctx, keys[0], &checkpoint); err != nil {
		t.Fatal(err)
	}
	checkpoint["Finalized"] = false
	if err = s.PutUsageJournal(ctx, keys[0], checkpoint); err != nil {
		t.Fatal(err)
	}
	s.Close()
	start(true)
	until = time.Now().Add(40 * time.Second)
	for {
		reader, e := storage.OpenReader(state)
		if e != nil {
			t.Fatal(e)
		}
		var cp struct {
			Instance  string
			Finalized bool
		}
		e = reader.UsageJournal(ctx, keys[0], &cp)
		reader.Close()
		if e != nil {
			t.Fatal(e)
		}
		if cp.Instance == info.InstanceID && cp.Finalized {
			break
		}
		if time.Now().After(until) {
			t.Fatal("restart never reread original source")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !reflect.DeepEqual(get(), snapshot) || !reflect.DeepEqual(readPages(snapshot.Revision), events) {
		t.Fatal("native reread duplicated usage")
	}
	var pricedAgain cost.Estimate
	decode(call(nil, "estimate-run-cost", "--run-id", id, "--usage-revision", strconv.FormatUint(snapshot.Revision, 10), "--pricing-policy", pricePath), &pricedAgain)
	if !reflect.DeepEqual(estimate, pricedAgain) {
		t.Fatal("reread changed fixed estimate")
	}
	stop()
	start(false)
	time.Sleep(2200 * time.Millisecond)
	if !reflect.DeepEqual(get(), snapshot) {
		t.Fatal("disabled replacement degraded finalized history")
	}
	stop()
	reader, err := storage.OpenReader(state)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	leases, err := reader.UsageJournalKeys(ctx, "lease:")
	if err != nil || len(leases) != 0 {
		t.Fatal("local path created directory grants", leases, err)
	}
	var permit api.CollectionPermit
	if err = reader.UsageJournal(ctx, api.CollectionPermitKey, &permit); err != nil || permit.Active {
		t.Fatal("stopped permit active", err)
	}
	copied, err := reader.UsageRuns(ctx)
	if err != nil || len(copied) != 1 || copied[0].Revision != 1 || copied[0].Allowed || (agent != "codex" && len(copied[0].Intents) != 0) {
		t.Fatal("usage changed business Run", err)
	}
	afterOriginal := readOriginal()
	originalRevision, afterRevision := original.Revision, afterOriginal.Revision
	// The independent original service can refresh PR state. Its monotonic
	// business revision is not part of the immutable usage evidence projection.
	if agent != "codex" {
		afterOriginal.Revision = original.Revision
	}
	if !reflect.DeepEqual(original, afterOriginal) {
		t.Fatal("original Run evidence changed")
	}
	if again := readControl(); !reflect.DeepEqual(again, control) {
		t.Fatal("original log changed")
	}
	// The initial native inventory socket can idle out during service cycling.
	// Reauthenticate on the same runtime for the final baseline assertion.
	check, err := native.OpenUsageFiles(ctx, "", envelope.Meta.RuntimeID, "local")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	afterGroups, e1 := check.Groups(ctx)
	afterFolders, e2 := check.Folders(ctx)
	if e1 != nil || e2 != nil || !reflect.DeepEqual(groups, afterGroups) || !reflect.DeepEqual(folders, afterFolders) {
		t.Fatal("local directory baseline changed", e1, e2)
	}
	save("restart-proof", map[string]any{"run_id": id, "snapshot_revision": snapshot.Revision, "responses": len(events), "graceful_restart": true, "forced_source_reread": true, "disabled_history": true, "lease_count": len(leases), "permit_active": permit.Active, "business_revision": copied[0].Revision, "original_revision_before": originalRevision, "original_revision_after": afterRevision, "agent": agent})
	t.Logf("Run=%s session=%s responses=%d total=%d synthetic_USD=%s revision=%d restart/reread dedup passed", id, binding.SessionID, len(events), *snapshot.Tokens.Total, expectedAmount, snapshot.Revision)
}

type localNativeControl struct {
	Commands  map[string]string     `json:"commands,omitempty"`
	SHA256    string                `json:"sha256"`
	Bytes     int                   `json:"bytes"`
	Tokens    api.Tokens            `json:"tokens"`
	Responses map[string]api.Tokens `json:"responses"`
}

// Independent raw authority check: intentionally does not call the production
// parser, ledger, normalization or cost calculation. This dedicated session
// must be wholly owned by the public Run, including bootstrap responses.
func readLocalCodexControl(t *testing.T, file string, b parsers.Binding) localNativeControl {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil || len(raw) > parsers.MaxSource {
		t.Fatal("source read", err)
	}
	hash := sha256.Sum256(raw)
	control := localNativeControl{SHA256: hex.EncodeToString(hash[:]), Bytes: len(raw), Responses: map[string]api.Tokens{}}
	var input, uncached, read, write, output, reason, total int64
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 65536), parsers.MaxRecord)
	for scanner.Scan() {
		var r struct {
			Type    string `json:"type"`
			Payload struct {
				ID         string           `json:"id"`
				CWD        string           `json:"cwd"`
				SessionID  string           `json:"session_id"`
				ResponseID string           `json:"response_id"`
				Usage      map[string]int64 `json:"usage"`
			} `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			t.Fatal("invalid original JSONL")
		}
		if r.Type == "session_meta" && (r.Payload.ID != b.SessionID || parsers.NormalizePath(r.Payload.CWD) != parsers.NormalizePath(b.CWD)) {
			t.Fatal("foreign original header")
		}
		if r.Type != "token_usage_record" {
			continue
		}
		p := r.Payload
		u := p.Usage
		if p.ResponseID == "" || p.SessionID != "" && p.SessionID != b.SessionID {
			t.Fatal("foreign native usage")
		}
		for _, name := range []string{"input_tokens", "cached_input_tokens", "cache_write_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens"} {
			v, ok := u[name]
			if !ok || v < 0 {
				t.Fatal("native counter unavailable", name)
			}
		}
		if u["input_tokens"]+u["output_tokens"] != u["total_tokens"] || u["cache_write_input_tokens"] != 0 {
			t.Fatal("unqualified native token shape")
		}
		x := api.Tokens{InputTotal: api.Number(u["input_tokens"]), InputUncached: api.Number(u["input_tokens"] - u["cached_input_tokens"]), CacheRead: api.Number(u["cached_input_tokens"]), CacheWrite: api.Number(0), Output: api.Number(u["output_tokens"]), Reasoning: api.Number(u["reasoning_output_tokens"]), Total: api.Number(u["total_tokens"])}
		if old, ok := control.Responses[p.ResponseID]; ok {
			if !reflect.DeepEqual(old, x) {
				t.Fatal("conflicting native response")
			}
			continue
		}
		control.Responses[p.ResponseID] = x
		input += *x.InputTotal
		uncached += *x.InputUncached
		read += *x.CacheRead
		write += *x.CacheWrite
		output += *x.Output
		reason += *x.Reasoning
		total += *x.Total
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(control.Responses) == 0 {
		t.Fatal("no authoritative native responses")
	}
	control.Tokens = api.Tokens{InputTotal: &input, InputUncached: &uncached, CacheRead: &read, CacheWrite: &write, Output: &output, Reasoning: &reason, Total: &total}
	return control
}
