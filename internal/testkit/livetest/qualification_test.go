package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestCoreSmokeRequiresQualifiedFeedbackForExactBuild(t *testing.T) {
	root := t.TempDir()
	write := func(name string, value any) string {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name)
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	scenario := map[string]any{"passed": true, "run_stopped": true, "service_stopped": true, "feedback_head": "updated", "merge_commit": "merged"}
	write("scenario.json", scenario)
	build := strings.Repeat("a", 64)
	record := map[string]any{"scenario": "local-codex.feedback", "case_id": "default", "status": "passed", "evidence": root, "identity": map[string]any{"runner_sha256": build, "driver_sha256": strings.Repeat("b", 64)}}
	execution := contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"model":"executor","reasoning_effort":"high"}`)}
	manifest := write("manifest.json", Manifest{Driver: "development-feedback", Scenario: "local-codex.feedback", Level: "L3", Execution: execution})
	manifestBody, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestSum := sha256.Sum256(manifestBody)
	record["manifest"] = manifest
	record["identity"].(map[string]any)["manifest_sha256"] = hex.EncodeToString(manifestSum[:])
	summary := map[string]any{"status": "passed", "results": []any{map[string]any{"scenario": "local-codex.feedback", "case_id": "default", "status": "passed", "evidence": root}}}
	write("summary.json", summary)
	path := write("case.json", record)
	proof := func() Tool {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		return Tool{Path: path, SHA256: hex.EncodeToString(sum[:])}
	}
	frozen := proof()
	if err := ValidateFeedbackQualification(frozen, build); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFeedbackExecution(frozen, execution); err != nil {
		t.Fatal(err)
	}
	changed := execution
	changed.Settings = json.RawMessage(`{"model":"another","reasoning_effort":"high"}`)
	if ValidateFeedbackExecution(frozen, changed) == nil {
		t.Fatal("different native model reused feedback qualification")
	}
	changed = execution
	changed.Settings = json.RawMessage(`{"reasoning_effort":"high","model":"executor"}`)
	if err := ValidateFeedbackExecution(frozen, changed); err != nil {
		t.Fatal("JSON key order changed execution identity", err)
	}
	summary["status"] = "failed"
	write("summary.json", summary)
	if ValidateFeedbackQualification(frozen, build) == nil {
		t.Fatal("worker pass overrode coordinator failure")
	}
	summary["status"] = "passed"
	write("summary.json", summary)
	if ValidateFeedbackQualification(frozen, strings.Repeat("c", 64)) == nil {
		t.Fatal("foreign build removed L2 coverage")
	}
	record["status"] = "failed"
	write("case.json", record)
	if ValidateFeedbackQualification(frozen, build) == nil || ValidateFeedbackQualification(proof(), build) == nil {
		t.Fatal("changed or failed feedback proof accepted")
	}
	record["status"] = "passed"
	record["scenario"] = "local-codex.smoke"
	write("case.json", record)
	if ValidateFeedbackQualification(proof(), build) == nil {
		t.Fatal("ordinary smoke substituted for feedback qualification")
	}
	record["scenario"] = "local-codex.feedback"
	write("case.json", record)
	for _, field := range []string{"passed", "run_stopped", "service_stopped", "feedback_head", "merge_commit"} {
		original := scenario[field]
		delete(scenario, field)
		write("scenario.json", scenario)
		if ValidateFeedbackQualification(proof(), build) == nil {
			t.Fatal("missing required qualification fact accepted", field)
		}
		scenario[field] = original
	}
}
