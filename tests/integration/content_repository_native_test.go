//go:build windows && live

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRContentRepositoryNative(t *testing.T) {
	if *contentNativeExe == "" {
		t.Skip("explicit native opt-in required")
	}
	testCount.Add(1)
	if *evidenceFlag == "" {
		t.Fatal("evidence directory required")
	}
	repo := fixedContentRepository(t)
	root, err := os.MkdirTemp("C:/srt", "gr-")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "state")
	save := func(name string, value any) {
		t.Helper()
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(*evidenceFlag, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save("repository-native-isolation", map[string]any{"root": root, "repository": repo, "native": *contentNativeExe, "runner": runnerEXE, "model": *contentNativeModel})
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	settings, _ := json.Marshal(contracts.CodexSettings{Executable: *contentNativeExe, AccessMode: "read_only", Model: *contentNativeModel, ReasoningEffort: "low"})
	r := contracts.RunRequest{Title: "Fixed repository code investigation", Workflow: contracts.Workflow{ID: "generate", Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settings}, Generate: &contracts.GenerateInput{Repository: repo, Instruction: "Use a shell read-only tool to read code.txt in the current snapshot. Return two documents at R7/spec.md and R7/plan.md. Include the entire exact contents of code.txt and the fixed commit in each document body. Use Roadmap internal code R7. Do not write files. Return only JSON matching output_schema.", Materials: []contracts.Document{}, OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["documents"],"properties":{"documents":{"type":"array","minItems":2,"maxItems":2,"prefixItems":[{"type":"object","required":["path","content"],"additionalProperties":false,"properties":{"path":{"const":"R7/spec.md"},"content":{"type":"string","minLength":1}}},{"type":"object","required":["path","content"],"additionalProperties":false,"properties":{"path":{"const":"R7/plan.md"},"content":{"type":"string","minLength":1}}}]}}}`)}}
	input, _ := json.Marshal(r)
	envelope := contracts.Request{Protocol: contracts.Protocol, RequestID: "native-fixed-repository", Input: input}
	raw, _ := json.Marshal(envelope)
	save("repository-native-request", envelope)
	response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	save("repository-native-submit", response)
	id := response.Data.(map[string]any)["run_id"].(string)
	view := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" || v.Status == "blocked" }, 4*time.Minute)
	save("repository-native-result", call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	if view.ContentResult == nil || view.ContentResult.Status != "succeeded" || view.Status != "completed" || view.RepositorySource == nil || view.RepositorySource.Commit != repo.Commit || len(view.RepositorySource.Files) != 1 {
		t.Fatal("native fixed repository result failed", view)
	}
	var output struct {
		Documents []struct{ Path, Content string } `json:"documents"`
	}
	if json.Unmarshal(view.ContentResult.JSON, &output) != nil || len(output.Documents) != 2 {
		t.Fatal("native documents missing")
	}
	for _, doc := range output.Documents {
		if !strings.Contains(doc.Content, "FIXED_SOURCE_47") || !strings.Contains(doc.Content, repo.Commit) || strings.Contains(doc.Content, "DIRTY_SOURCE_99") {
			t.Fatal("native did not investigate exact source", doc)
		}
	}
	logs, err := filepath.Glob(filepath.Join(dir, "runs", id, "turns", "*", "appserver.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	toolObserved := false
	for _, path := range logs {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		toolObserved = toolObserved || strings.Contains(string(body), `"commandExecution"`)
	}
	save("repository-native-tool-proof", map[string]any{"command_execution_observed": toolObserved, "original_logs": logs})
	if !toolObserved {
		t.Fatal("native code read tool not observed")
	}
	save("repository-native-history", call(t, dir, []string{"history", "--run-id", id}, "", 0))
	service = restartEdgeService(t, dir, service, 87)
	replayed := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	if replayed.Data.(map[string]any)["run_id"] != id {
		t.Fatal("replay created a run")
	}
	restored := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	if restored.Execution.SessionID != view.Execution.SessionID || restored.Execution.TurnID != view.Execution.TurnID || restored.ContentResult == nil {
		t.Fatal("restart changed native result")
	}
	save("repository-native-replayed", replayed)
	contentCommand(t, dir, id, "stop", "native-fixed-stop", map[string]any{}, 0)
	save("repository-native-stopped", call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	save("repository-native-service-stop", call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("native-fixed-service-stop", service.InstanceID), 0))
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	save("repository-native-verify-stopped", call(t, dir, []string{"service", "verify-stopped"}, "", 0))
}
