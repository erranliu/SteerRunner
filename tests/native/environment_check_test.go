//go:build windows && live

package native

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func environmentCheck(t *testing.T, m livetest.Manifest) {
	dir := filepath.Join(m.Root, "s")
	result := scenarioResult{Scenario: m.Scenario, Level: m.Level, Stage: "accepted"}
	t.Cleanup(func() {
		result.Passed = !t.Failed() && result.RunStopped && result.ServiceStopped
		save(t, "scenario.json", result)
	})
	service := invoke(t, m, dir, []string{"service", "start"}, nil).Data.(map[string]any)
	instance := service["instance_id"].(string)
	t.Cleanup(func() {
		req := request("service-stop", map[string]any{"expected_instance_id": instance})
		_, err := command(m, dir, []string{"service", "stop", "--input", "-"}, &req)
		if err != nil {
			t.Error(err)
			return
		}
		for until := time.Now().Add(30 * time.Second); time.Now().Before(until); time.Sleep(time.Second) {
			response, err := command(m, dir, []string{"service", "status"}, nil)
			if err == nil && response.Data.(map[string]any)["status"] == "not_running" {
				result.ServiceStopped = true
				break
			}
		}
		proof, err := command(m, dir, []string{"service", "verify-stopped"}, nil)
		if err != nil {
			t.Error(err)
			return
		}
		save(t, "check-offline-stop.json", proof)
		if proof.Data.(map[string]any)["resources_settled"] != true {
			t.Error("check writers/resources were not settled")
		}
	})
	source, err := m.ExecutionRepositoryPath()
	if err != nil {
		t.Fatal(err)
	}
	input := contracts.CheckInput{Repository: contracts.Repository{URL: livetest.RemoteURL, BaseRevision: m.BaseRevision, Remote: "origin", BaseBranch: m.BaseBranch, LocalPath: source}, Execution: m.Execution, Publication: &m.Publication}
	req := request("environment-check-start", input)
	response := invoke(t, m, dir, []string{"environment-check", "start", "--input", "-"}, &req)
	save(t, "check-start.json", response)
	raw, _ := json.Marshal(response.Data)
	var receipt contracts.CheckReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil || receipt.CheckID == nil {
		t.Fatal(response, err)
	}
	id := *receipt.CheckID
	result.RunID = id
	var previous *contracts.EnvironmentCheck
	for until := time.Now().Add(12 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		response := invoke(t, m, dir, []string{"environment-check", "inspect", "--check-id", id}, nil)
		raw, _ := json.Marshal(response.Data)
		var inspection contracts.CheckInspection
		if err := json.Unmarshal(raw, &inspection); err != nil || inspection.Check == nil {
			t.Fatal(err)
		}
		check := inspection.Check
		if previous != nil {
			if f := contracts.ValidateCheckUpdate(*previous, *check); f != nil {
				t.Fatal(f)
			}
		}
		previous = check
		result.Stage = check.Status + ":" + check.CleanupStatus
		save(t, "scenario.json", result)
		save(t, "check-latest.json", response)
		if check.CleanupStatus == "pending" {
			continue
		}
		if check.Result != "passed" || check.CleanupStatus != "completed" || contracts.CheckResult(*check) != "passed" {
			t.Fatalf("environment check did not pass: %s / %s / %s", check.Result, check.CleanupStatus, check.Diagnostic)
		}
		if check.Actual.ApprovalPolicy != "never" || check.Actual.SessionID == "" || check.Actual.TurnID == "" || check.Actual.Account == "" || check.Actual.AuthenticationSource != "host-gh-repository-broker" {
			t.Fatal("missing native context", check.Actual)
		}
		if m.Execution.Adapter == "orca-terminal" {
			runtime, err := m.OrcaRuntimeID()
			if err != nil || check.Actual.RuntimeID != runtime || check.Actual.Terminal == "" || check.Actual.TerminalIncarnation == "" || check.Actual.WorktreeID == "" {
				t.Fatal("missing original Orca execution-host identity", check.Actual, err)
			}
		}
		if check.Resources.PRNumber <= 0 || !check.Resources.PRClosed || !check.Resources.BaseDeleted || !check.Resources.HeadDeleted || !check.Resources.WriterStopped || !check.Resources.LocalRemoved {
			t.Fatal("owned objects not settled", check.Resources)
		}
		if !strings.HasSuffix(check.Resources.BaseRef, "/"+id+"/base") || !strings.HasSuffix(check.Resources.HeadRef, "/"+id+"/head") {
			t.Fatal("foreign reference namespace")
		}
		// Driver only reads Worker-owned references and PR; it never writes them.
		if len(strings.Fields(string(git(t, m, "ls-remote", "origin", check.Resources.BaseRef, check.Resources.HeadRef)))) != 0 {
			t.Fatal("owned references remain")
		}
		base := strings.Fields(string(git(t, m, "ls-remote", "origin", "refs/heads/"+m.BaseBranch)))
		if len(base) != 2 || base[0] != m.BaseRevision {
			t.Fatal("project source base changed during environment check")
		}
		result.OriginalHead = check.Resources.HeadSHA
		result.PRURL = check.Resources.PRURL
		result.RunStopped = true
		result.Stage = "check_verified"
		return
	}
	t.Fatal("original check deadline exhausted")
}
