//go:build windows

package integration

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func checkInspection(t *testing.T, r contracts.Response) contracts.CheckInspection {
	t.Helper()
	raw, _ := json.Marshal(r.Data)
	var v contracts.CheckInspection
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestEnvironmentCheckCLI(t *testing.T) {
	testCount.Add(1)
	dir := filepath.Join(t.TempDir(), "state")
	call(t, dir, []string{"environment-check", "inspect", "--request-id", "unknown"}, "", 4)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("inspect started service or created storage")
	}
	call(t, dir, []string{"environment-check", "inspect", "--request-id", "unknown", "--check-id", "guess"}, "", 2)
	call(t, dir, []string{"environment-check", "stop", "--check-id", "guess", "--input", "-"}, `{}`, 2)
	input := contracts.CheckInput{Repository: contracts.Repository{URL: "https://github.com/owner/repo.git", BaseRevision: strings.Repeat("a", 40), Remote: "origin", BaseBranch: "main", LocalPath: t.TempDir()}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"full_access","executable":"missing.exe"}`)}}
	req := contracts.Request{Protocol: contracts.Protocol, RequestID: "start-original", Input: mustJSON(input)}
	raw, _ := json.Marshal(req)
	accepted := call(t, dir, []string{"environment-check", "start", "--input", "-"}, string(raw), 0)
	capability := call(t, dir, []string{"capabilities"}, "", 0)
	fixturePack := capability.Data.(map[string]any)["environment_check_fixtures"].(map[string]any)
	if fixturePack["contract"] != contracts.EnvironmentCheckContract {
		t.Fatal("fixture version differs from CLI contract")
	}
	samples := fixturePack["samples"].(map[string]any)
	assertFixtureShape(t, samples["start_receipt"].(map[string]any)["data"], accepted.Data)
	var receipt contracts.CheckReceipt
	if err := json.Unmarshal(mustJSON(accepted.Data), &receipt); err != nil || receipt.CheckID == nil {
		t.Fatal(accepted, err)
	}
	info := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0))
	cleanupService(t, dir, info)
	id := *receipt.CheckID
	var inspection contracts.CheckInspection
	until := time.Now().Add(15 * time.Second)
	for {
		inspection = checkInspection(t, call(t, dir, []string{"environment-check", "inspect", "--request-id", req.RequestID}, "", 0))
		if inspection.Check.CleanupStatus == "completed" {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("inspection: %s", mustJSON(inspection))
		}
		time.Sleep(30 * time.Millisecond)
	}
	if inspection.Command.Command != "environment-check start" || inspection.Check.Result != "failed" || inspection.Check.InputDigest == "" || inspection.Check.ResourceDigest == "" || inspection.Check.Actual.SessionID != "" {
		t.Fatalf("inspection: %s", mustJSON(inspection))
	}
	var publicInspection any
	_ = json.Unmarshal(mustJSON(inspection), &publicInspection)
	assertFixtureShape(t, samples["preparation_failure"].(map[string]any)["data"].(map[string]any)["check"], publicInspection.(map[string]any)["check"])
	manifest := contracts.CheckManifest(false)
	if len(inspection.Check.Items) != len(manifest) {
		t.Fatal("fixed manifest changed")
	}
	for i, item := range manifest {
		if inspection.Check.Items[i].ID != item.ID || inspection.Check.Items[i].Required != item.Required {
			t.Fatal("fixed manifest identity changed")
		}
	}
	byID := checkInspection(t, call(t, dir, []string{"environment-check", "inspect", "--check-id", id}, "", 0))
	if byID.Command != nil || byID.Check.Revision != inspection.Check.Revision {
		t.Fatal(byID)
	}
	call(t, dir, []string{"environment-check", "start", "--input", "-"}, string(raw), 0)
	bad := req
	bad.Input = json.RawMessage(`{}`)
	call(t, dir, []string{"environment-check", "start", "--input", "-"}, string(mustJSON(bad)), 5)
	stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-original", Input: mustJSON(contracts.CheckStopInput{CheckID: id, ExpectedRevision: inspection.Check.Revision})}
	call(t, dir, []string{"environment-check", "stop", "--input", "-"}, string(mustJSON(stop)), 0)
	for {
		inspection = checkInspection(t, call(t, dir, []string{"environment-check", "inspect", "--request-id", stop.RequestID}, "", 0))
		if inspection.Command.ActionStatus == "completed" {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("inspection: %s", mustJSON(inspection))
		}
		time.Sleep(30 * time.Millisecond)
	}
	if inspection.Command.Command != "environment-check stop" || inspection.Command.CommandID == receipt.CommandID {
		t.Fatal("stop original was replaced", inspection)
	}
	stop.RequestID = "stale-stop"
	call(t, dir, []string{"environment-check", "stop", "--input", "-"}, string(mustJSON(stop)), 5)
	denied := checkInspection(t, call(t, dir, []string{"environment-check", "inspect", "--request-id", stop.RequestID}, "", 0))
	if denied.Check != nil || denied.Command.ReceiptStatus != "rejected" || denied.Command.Error.Code != "stale_revision" {
		t.Fatal(denied)
	}
	input.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "saved"}, Settings: json.RawMessage(`{"access_mode":"full_access"}`)}
	req.RequestID = "unsupported-original"
	req.Input = mustJSON(input)
	call(t, dir, []string{"environment-check", "start", "--input", "-"}, string(mustJSON(req)), 3)
	denied = checkInspection(t, call(t, dir, []string{"environment-check", "inspect", "--request-id", req.RequestID}, "", 0))
	if denied.Check != nil || denied.Command.Error.Code != "unsupported_capability" {
		t.Fatal(denied)
	}
	call(t, dir, []string{"environment-check", "inspect", "--check-id", "missing"}, "", 5)
	call(t, dir, []string{"environment-check", "inspect", "--request-id", "missing"}, "", 5)
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("shutdown-original", info.InstanceID), 0)
	if err := waitOwnedService(info); err != nil {
		t.Fatal(err)
	}
	proof := call(t, dir, []string{"service", "verify-stopped"}, "", 0)
	var stopped contracts.StoppedVerification
	_ = json.Unmarshal(mustJSON(proof.Data), &stopped)
	if !stopped.ResourcesSettled || len(stopped.Checks) != 1 || stopped.Checks[0].CheckID != id {
		t.Fatal(stopped)
	}
	// A normal query cannot read storage offline or perform recovery.
	call(t, dir, []string{"environment-check", "inspect", "--check-id", id}, "", 4)
	newInfo := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, newInfo)
	reloaded := checkInspection(t, call(t, dir, []string{"environment-check", "inspect", "--check-id", id}, "", 0))
	if reloaded.Check.InputDigest != inspection.Check.InputDigest || reloaded.Check.Revision != inspection.Check.Revision {
		t.Fatal("readable frozen report changed after restart", reloaded)
	}
}

func assertFixtureShape(t *testing.T, expected, actual any) {
	t.Helper()
	switch v := expected.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			t.Fatal("public fixture object shape changed")
		}
		for key, child := range v {
			current, ok := a[key]
			if !ok {
				t.Fatal("missing public fixture field", key)
			}
			assertFixtureShape(t, child, current)
		}
	case []any:
		a, ok := actual.([]any)
		if !ok {
			t.Fatal("public fixture manifest shape changed")
		}
		for i, child := range v {
			if i >= len(a) {
				break
			} // allowed actions/evidence vary with report state
			assertFixtureShape(t, child, a[i])
		}
	default:
		if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
			t.Fatalf("public fixture scalar shape changed: expected %T (%v), actual %T (%v)", expected, expected, actual, actual)
		}
	}
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
