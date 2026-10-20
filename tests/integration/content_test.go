//go:build windows

package integration

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func contentFixture(t *testing.T, workflow string, schema json.RawMessage) (string, string, contracts.ServiceInfo) {
	t.Helper()
	root := t.TempDir()
	native := filepath.Join(root, "native")
	_ = os.MkdirAll(native, 0700)
	t.Setenv("STEERRUNNER_FIXTURE_STATE", native)
	t.Setenv("CODEX_HOME", native)
	t.Setenv("STEERRUNNER_CONTENT_FIXTURE", "1")
	if !existsTest(filepath.Join(native, "content-final")) {
		_ = os.WriteFile(filepath.Join(native, "content-final"), []byte(`{"ok":true}`), 0600)
	}
	if marker := os.Getenv("STEERRUNNER_CONTENT_MARKER"); marker != "" {
		if marker == "bad-json" {
			_ = os.WriteFile(filepath.Join(native, "content-final"), []byte("bad JSON"), 0600)
		} else {
			_ = os.WriteFile(filepath.Join(native, marker), []byte("marker"), 0600)
		}
	}
	dir := filepath.Join(root, "state")
	settings, _ := json.Marshal(contracts.CodexSettings{Executable: helperEXE, AccessMode: "read_only"})
	r := contracts.RunRequest{Title: "Content controlled", Workflow: contracts.Workflow{ID: workflow, Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settings}}
	if workflow == "generate" {
		r.Generate = &contracts.GenerateInput{Instruction: "Return content", Materials: []contracts.Document{}, OutputSchema: schema}
	} else {
		r.Conversation = &contracts.ConversationInput{Instruction: "Discuss", Materials: []contracts.Document{}, Directory: contracts.DirectoryAccess{Path: root, Access: "read_only"}}
	}
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	input, _ := json.Marshal(r)
	raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "content-submit", Input: input})
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	return dir, id, service
}
func existsTest(path string) bool { _, err := os.Stat(path); return err == nil }

func contentProcess(t *testing.T, dir, id string) codex.ProcessReceipt {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "runs", id, "app-server", "process.json"))
	var receipt codex.ProcessReceipt
	if err != nil || json.Unmarshal(raw, &receipt) != nil || process.InspectProcess(&receipt.Identity) != "matching process still exists" {
		t.Fatal("original content process is not verifiably live", err, receipt)
	}
	return receipt
}

func contentCommand(t *testing.T, dir, id, command, request string, body map[string]any, exit int) string {
	t.Helper()
	v := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	body["expected_revision"] = v.Revision
	input, _ := json.Marshal(body)
	raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: request, Input: input})
	call(t, dir, []string{command, "--run-id", id, "--input", "-"}, string(raw), exit)
	return string(raw)
}

func verifyContentTakeover(t *testing.T, dir, id string) {
	t.Helper()
	before := call(t, dir, []string{"inspect", "--run-id", id}, "", 0)
	starts := countNative(t, "turn/start")
	response := call(t, dir, []string{"service", "verify-takeover", "--candidate", runnerEXE}, "", 0)
	raw, _ := json.Marshal(response.Data)
	var report contracts.TakeoverReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if !report.Compatible || len(report.Items) != 1 || report.Items[0].ID != id || report.Items[0].RequestID != "content-submit" || report.Items[0].RequestDigest == "" || report.Items[0].Status != "compatible" {
		t.Fatal("original content takeover rejected or unbound", report)
	}
	after := call(t, dir, []string{"inspect", "--run-id", id}, "", 0)
	beforeRaw, _ := json.Marshal(before.Data)
	afterRaw, _ := json.Marshal(after.Data)
	if string(beforeRaw) != string(afterRaw) || starts != countNative(t, "turn/start") {
		t.Fatal("takeover changed original content or sent input")
	}
}

func TestSRContentGenerate(t *testing.T) {
	testCount.Add(1)
	for _, tc := range []struct {
		name         string
		schema       json.RawMessage
		marker, want string
	}{
		{"text", nil, "", "succeeded"}, {"json", json.RawMessage(`{"type":"object","required":["ok"]}`), "", "succeeded"},
		{"schema_mismatch", json.RawMessage(`{"type":"array"}`), "", "failed"},
		{"bad_json", json.RawMessage(`{"type":"object"}`), "bad-json", "failed"},
		{"failed", nil, "content-failed", "failed"}, {"cancelled", nil, "content-cancelled", "cancelled"}, {"no_final", nil, "missing-final", "failed"}, {"question", nil, "content-question", "failed"},
		{"no_auth", nil, "content-no-auth", "failed"}, {"no_directory", nil, "content-no-directory", "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Set marker before service initialization; each subcase owns its fixture.
			t.Setenv("STEERRUNNER_CONTENT_MARKER", tc.marker)
			dir, id, _ := contentFixture(t, "generate", tc.schema)
			view := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
			expectedTurns := 1
			if tc.name == "no_auth" || tc.name == "no_directory" {
				expectedTurns = 0
			}
			if view.ContentResult == nil || view.ContentResult.Status != tc.want || !view.Closed || view.PR != nil || countNative(t, "turn/start") != expectedTurns || countNative(t, "thread/start") != expectedTurns {
				t.Fatalf("wrong one-shot result: %+v", view)
			}
			if tc.name == "question" && view.ContentResult.Diagnostic != "human_input_required" {
				t.Fatal(view.ContentResult)
			}
			if tc.name == "text" || tc.name == "json" {
				verifyContentTakeover(t, dir, id)
			}
			page := call(t, dir, []string{"history", "--run-id", id}, "", 0)
			if !page.OK {
				t.Fatal(page)
			}
		})
	}
}

func TestSRContentConversation(t *testing.T) {
	testCount.Add(1)
	dir, id, service := contentFixture(t, "conversation", nil)
	first := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
	verifyContentTakeover(t, dir, id)
	raw := contentCommand(t, dir, id, "message", "message-two", map[string]any{"text": "second"}, 0)
	call(t, dir, []string{"message", "--run-id", id, "--input", "-"}, raw, 0)
	second := waitRun(t, dir, id, func(v contracts.RunView) bool {
		return v.Controls.CanMessage && v.Execution.TurnID != first.Execution.TurnID
	})
	if second.Execution.SessionID != first.Execution.SessionID || countNative(t, "turn/start") != 2 || countNative(t, "thread/start") != 1 {
		t.Fatal("not original two-turn session")
	}
	state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	_ = os.WriteFile(filepath.Join(state, "hold-turn"), []byte("hold"), 0600)
	contentCommand(t, dir, id, "message", "message-interrupt", map[string]any{"text": "wait"}, 0)
	active := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanInterrupt })
	contentCommand(t, dir, id, "message", "concurrent-message", map[string]any{"text": "no"}, 5)
	contentCommand(t, dir, id, "interrupt", "interrupt-turn", map[string]any{}, 0)
	interrupted := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
	if interrupted.Execution.SessionID != active.Execution.SessionID || interrupted.ContentResult.Status != "cancelled" || interrupted.Closed || countNative(t, "turn/interrupt") != 1 {
		t.Fatal(interrupted)
	}
	_ = os.Remove(filepath.Join(state, "hold-turn"))
	contentCommand(t, dir, id, "message", "message-after-interrupt", map[string]any{"text": "continue"}, 0)
	idle := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
	originalProcess := contentProcess(t, dir, id)
	service = restartEdgeService(t, dir, service, 41)
	restarted := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
	if restarted.ResumeRequired || countNative(t, "turn/start") != 4 || restarted.Execution.SessionID != idle.Execution.SessionID || contentProcess(t, dir, id) != originalProcess {
		t.Fatal("restart replayed input")
	}
	contentCommand(t, dir, id, "resume", "unneeded-resume", map[string]any{}, 5)
	contentCommand(t, dir, id, "message", "after-restart", map[string]any{"text": "continue same session"}, 0)
	continued := waitRun(t, dir, id, func(v contracts.RunView) bool {
		return v.Controls.CanMessage && v.Execution.TurnID != idle.Execution.TurnID
	})
	if continued.Execution.SessionID != first.Execution.SessionID || countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 5 {
		t.Fatal(continued)
	}
	contentCommand(t, dir, id, "close", "close-original", map[string]any{}, 0)
	waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	contentCommand(t, dir, id, "message", "after-close", map[string]any{"text": "no"}, 5)
	_ = service
}

func TestSRContentQuestionAndUnknown(t *testing.T) {
	testCount.Add(1)
	t.Run("unknown-interrupt", func(t *testing.T) {
		t.Setenv("STEERRUNNER_CONTENT_MARKER", "hold-turn")
		dir, id, _ := contentFixture(t, "conversation", nil)
		waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanInterrupt })
		state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
		_ = os.WriteFile(filepath.Join(state, "drop-next-interrupt"), []byte("drop"), 0600)
		raw := contentCommand(t, dir, id, "interrupt", "unknown-interrupt", map[string]any{}, 0)
		v := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
		call(t, dir, []string{"interrupt", "--run-id", id, "--input", "-"}, raw, 0)
		if v.ContentResult == nil || v.ContentResult.Status != "cancelled" || countNative(t, "turn/interrupt") != 1 || countNative(t, "turn/start") != 1 {
			t.Fatal("unknown interrupt replayed", v)
		}
		contentCommand(t, dir, id, "close", "close-unknown-interrupt", map[string]any{}, 0)
	})
	t.Run("question", func(t *testing.T) {
		t.Setenv("STEERRUNNER_CONTENT_MARKER", "content-question")
		dir, id, _ := contentFixture(t, "conversation", nil)
		q := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Question != nil })
		raw := contentCommand(t, dir, id, "reply", "answer-once", map[string]any{"kind": "answer", "question_id": q.Question.ID, "text": "one"}, 0)
		call(t, dir, []string{"reply", "--run-id", id, "--input", "-"}, raw, 0)
		waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
		contentCommand(t, dir, id, "reply", "stale-answer", map[string]any{"kind": "answer", "question_id": q.Question.ID, "text": "two"}, 5)
		if countNative(t, "turn/steer") != 1 || countNative(t, "turn/start") != 1 {
			t.Fatal("answer replayed or created second turn")
		}
		contentCommand(t, dir, id, "close", "close-question", map[string]any{}, 0)
	})
	t.Run("unknown", func(t *testing.T) {
		t.Setenv("STEERRUNNER_CONTENT_MARKER", "drop-next-start")
		dir, id, service := contentFixture(t, "conversation", nil)
		waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "reconciling" })
		service = restartEdgeService(t, dir, service, 42)
		time.Sleep(2 * time.Second)
		view := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
		if countNative(t, "turn/start") != 1 || countNative(t, "thread/start") != 1 || view.Controls.CanMessage || view.Execution.SessionID == "" {
			t.Fatal("unknown input replayed", view)
		}
		contentCommand(t, dir, id, "close", "close-unknown", map[string]any{}, 0)
		waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
		_ = service
	})
}

func runView(t *testing.T, response contracts.Response) contracts.RunView {
	t.Helper()
	raw, _ := json.Marshal(response.Data)
	var wrapper struct {
		Run contracts.RunView `json:"run"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatal(err)
	}
	return wrapper.Run
}
