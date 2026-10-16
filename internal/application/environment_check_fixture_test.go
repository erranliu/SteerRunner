package application

import (
	"bytes"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures are emitted by the production persistence/query path. Only volatile
// IDs, dates and build digests are normalized; they are examples, not readiness
// evidence. Updating this embedded asset changes the resource digest.
func TestEnvironmentCheckPublicFixtures(t *testing.T) {
	e := environmentEngine(t)
	input := contracts.CheckInput{Repository: contracts.Repository{URL: "https://github.com/owner/repo.git", BaseRevision: strings.Repeat("a", 40), Remote: "origin", BaseBranch: "main"}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"sandbox"}`)}}
	start := contracts.Request{Protocol: contracts.Protocol, RequestID: "example-start", Input: canonical(input)}
	checkID := strings.Repeat("1", 32)
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	c := storage.NewCheckCommand("environment-check start", checkID, start)
	r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: checkID, Contract: contracts.EnvironmentCheckContract, Input: input, InputDigest: strings.Repeat("a", 64), ResourceDigest: strings.Repeat("b", 64), CreatedAt: now, ExecutionDeadline: now.Add(10 * time.Minute), CleanupDeadline: now.Add(12 * time.Minute), Status: "queued", Result: "unverified", CleanupStatus: "pending", Actual: contracts.CheckContext{AttemptID: strings.Repeat("4", 32), Generation: strings.Repeat("5", 32)}, Items: contracts.CheckManifest(false), AllowedActions: []string{"stop"}, Excluded: []string{"build", "tests", "business_resources"}, Resources: contracts.CheckResources{BaseRef: "refs/heads/steerline-check/" + checkID + "/base", HeadRef: "refs/heads/steerline-check/" + checkID + "/head", BaseSHA: strings.Repeat("a", 40)}}, StartCommand: c.ID, CommandStates: map[string]string{c.ID: "pending"}}
	e.mu.Lock()
	err := e.commitCheckLocked(r, &c)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	samples := map[string]any{"start_request": start, "start_receipt": c.Response, "inspect_by_check_id": e.HandleEnvironmentCheck("environment-check inspect", checkID, "", nil), "inspect_by_start_request_id": e.HandleEnvironmentCheck("environment-check inspect", "", start.RequestID, nil)}
	if err = e.updateCheck(checkID, func(r *runstate.CheckRecord) {
		r.View.Status = "failed"
		r.View.Items[0].Status = "failed"
		r.View.Items[0].Stage = "native_preparation_failed"
		r.View.Result = "failed"
		r.CommandStates[c.ID] = "failed"
	}); err != nil {
		t.Fatal(err)
	}
	samples["preparation_failure"] = e.HandleEnvironmentCheck("environment-check inspect", checkID, "", nil)
	stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "example-stop", Input: canonical(contracts.CheckStopInput{CheckID: checkID, ExpectedRevision: 2})}
	stopCommand := storage.NewCheckCommand("environment-check stop", checkID, stop)
	e.mu.Lock()
	next := e.checks[checkID].Clone()
	next.CommandStates[stopCommand.ID] = "pending"
	err = e.commitCheckLocked(next, &stopCommand)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	samples["stop_request"] = stop
	samples["stop_receipt"] = stopCommand.Response
	samples["inspect_by_stop_request_id"] = e.HandleEnvironmentCheck("environment-check inspect", "", stop.RequestID, nil)
	stale := contracts.Request{Protocol: contracts.Protocol, RequestID: "example-stale-stop", Input: stop.Input}
	samples["stale_revision"] = e.HandleEnvironmentCheck("environment-check stop", "", "", &stale)
	samples["rejection_without_check"] = e.HandleEnvironmentCheck("environment-check inspect", "", stale.RequestID, nil)
	samples["unknown_request"] = e.HandleEnvironmentCheck("environment-check inspect", "", "example-unknown", nil)
	samples["unknown_write_response"] = contracts.Rejected(start.RequestID, contracts.Fail(6, "request_outcome_unknown", "Original response is unknown; inspect the original request_id."))
	if err = e.updateCheck(checkID, func(r *runstate.CheckRecord) {
		r.View.Status = "unverified"
		r.View.Result = "unverified"
		r.View.Items[0].Status = "unverified"
		r.CommandStates[c.ID] = "unknown"
	}); err != nil {
		t.Fatal(err)
	}
	samples["unverified_original"] = e.HandleEnvironmentCheck("environment-check inspect", "", start.RequestID, nil)
	raw, _ := json.Marshal(samples)
	raw = bytes.ReplaceAll(raw, []byte(c.ID), []byte(strings.Repeat("2", 32)))
	raw = bytes.ReplaceAll(raw, []byte(stopCommand.ID), []byte(strings.Repeat("3", 32)))
	// The rejected command has its own identity too.
	rejected, _ := e.store.CheckCommand(stale.RequestID)
	raw = bytes.ReplaceAll(raw, []byte(rejected.CommandID), []byte(strings.Repeat("6", 32)))
	var normalized map[string]any
	_ = json.Unmarshal(raw, &normalized)
	var dates func(any)
	dates = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for k, child := range v {
				if k == "updated_at" {
					v[k] = now.Format(time.RFC3339)
				} else {
					dates(child)
				}
			}
		case []any:
			for _, child := range v {
				dates(child)
			}
		}
	}
	dates(normalized)
	pack := map[string]any{"contract": contracts.EnvironmentCheckContract, "normalization": "Stable example identities/timestamps/digests; never evidence of environment readiness. Reports without Publication do not include PR readiness.", "samples": normalized}
	want, _ := json.MarshalIndent(pack, "", "  ")
	want = append(want, '\n')
	path := filepath.Join("..", "resources", "assets", "environment-check.fixtures.json")
	if os.Getenv("STEERRUNNER_UPDATE_CHECK_FIXTURES") == "1" {
		if err := os.WriteFile(path, want, 0600); err != nil {
			t.Fatal(err)
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatal("embedded public examples differ from the production query/receipt path; regenerate with STEERRUNNER_UPDATE_CHECK_FIXTURES=1")
	}
	if os.Getenv("STEERRUNNER_UPDATE_CHECK_FIXTURES") != "1" && !bytes.Equal(resources.Read("environment-check.fixtures.json"), want) {
		t.Fatal("fixtures are not shipped with this build")
	}
}
