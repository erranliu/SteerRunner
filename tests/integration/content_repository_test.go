//go:build windows

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func fixedContentRepository(t *testing.T) *contracts.GenerateRepository {
	t.Helper()
	path := t.TempDir()
	git(t, path, "init")
	git(t, path, "config", "user.name", "Runner Test")
	git(t, path, "config", "user.email", "runner@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "code.txt"), []byte("FIXED_SOURCE_47\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, path, "add", "--", "code.txt")
	git(t, path, "-c", "core.hooksPath=", "commit", "-m", "fixed source")
	commit := git(t, path, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(path, "code.txt"), []byte("DIRTY_SOURCE_99\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "dirty-only.txt"), []byte("NOT_COMMITTED"), 0600); err != nil {
		t.Fatal(err)
	}
	return &contracts.GenerateRepository{LocalPath: path, Commit: commit}
}

func TestSRContentRepository(t *testing.T) {
	testCount.Add(1)
	for _, mode := range []string{"complete", "active-restart-stop", "unknown-restart-stop"} {
		t.Run(mode, func(t *testing.T) {
			repo := fixedContentRepository(t)
			final := `{"documents":[{"path":"R7/spec.md","content":"full spec"},{"path":"R7/plan.md","content":"full plan"}]}`
			t.Setenv("STEERRUNNER_CONTENT_FINAL", final)
			if mode == "active-restart-stop" {
				t.Setenv("STEERRUNNER_CONTENT_MARKER", "hold-turn")
			}
			if mode == "unknown-restart-stop" {
				t.Setenv("STEERRUNNER_CONTENT_MARKER", "drop-next-start")
			}
			schema, _ := json.Marshal(map[string]any{"const": json.RawMessage(final)})
			dir, id, service := contentFixture(t, "generate", schema, repo)
			var view contracts.RunView
			if mode == "complete" {
				view = waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
			} else {
				view = waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.TurnStarted || v.Status == "reconciling" })
			}
			if view.RepositorySource == nil || view.RepositorySource.Commit != repo.Commit || len(view.RepositorySource.Files) != 1 {
				t.Fatal("missing fixed source", view)
			}
			data, err := os.ReadFile(filepath.Join(view.Execution.Workspace, "code.txt"))
			if err != nil || string(data) != "FIXED_SOURCE_47\n" {
				t.Fatal("dirty content leaked", err, string(data))
			}
			settings, _ := json.Marshal(contracts.CodexSettings{Executable: helperEXE, AccessMode: "read_only"})
			input, _ := json.Marshal(contracts.RunRequest{Title: "Content controlled", Workflow: contracts.Workflow{ID: "generate", Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settings}, Generate: &contracts.GenerateInput{Repository: repo, Instruction: "Return content", Materials: []contracts.Document{}, OutputSchema: schema}})
			raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "content-submit", Input: input})
			if replay := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0); replay.Data.(map[string]any)["run_id"] != id {
				t.Fatal("replay created run")
			}
			before, _ := json.Marshal(view.RepositorySource)
			service = restartEdgeService(t, dir, service, 81)
			time.Sleep(time.Second)
			view = runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
			after, _ := json.Marshal(view.RepositorySource)
			if string(before) != string(after) || countNative(t, "turn/start") != 1 {
				t.Fatal("restart changed source or repeated input")
			}
			call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			if mode == "complete" {
				var got, expected any
				if view.ContentResult != nil {
					_ = json.Unmarshal(view.ContentResult.JSON, &got)
				}
				_ = json.Unmarshal([]byte(final), &expected)
				gotJSON, _ := json.Marshal(got)
				expectedJSON, _ := json.Marshal(expected)
				if view.ContentResult == nil || string(gotJSON) != string(expectedJSON) || view.ContentResult.RepositorySource == nil || view.Status != "completed" {
					t.Fatal("full result lost", view)
				}
				verifyContentTakeover(t, dir, id)
				call(t, dir, []string{"history", "--run-id", id}, "", 0)
			}
			contentCommand(t, dir, id, "stop", "stop-fixed-repository", map[string]any{}, 0)
			waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
			service = restartEdgeService(t, dir, service, 82)
			if countNative(t, "turn/start") != 1 {
				t.Fatal("stop recovery repeated input")
			}
			_ = service
		})
	}
}
