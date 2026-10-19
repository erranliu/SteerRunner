//go:build windows && live

package integration

import (
	"encoding/json"
	"flag"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var contentNativeExe = flag.String("content-native-exe", "", "Explicit existing native Codex 0.159.2; never installed by this driver")
var contentNativeModel = flag.String("content-native-model", "", "Optional model available to the existing native account")

func TestSRContentNative(t *testing.T) {
	if *contentNativeExe == "" {
		t.Skip("explicit native opt-in required")
	}
	testCount.Add(1)
	if !filepath.IsAbs(*contentNativeExe) || *evidenceFlag == "" {
		t.Fatal("explicit executable/evidence identity required")
	}
	if err := os.MkdirAll("C:/srt", 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("C:/srt", "cw-")
	if err != nil {
		t.Fatal(err)
	}
	saveNative := func(name string, v any) {
		raw, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(filepath.Join(*evidenceFlag, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	saveNative("content-isolation", map[string]any{"root": root, "native": *contentNativeExe, "model": *contentNativeModel, "runner": runnerEXE, "state_isolation": "new Runner-owned directory; no repository, SteerLine, publication or installation"})
	dir := filepath.Join(root, "state")
	directory := filepath.Join(root, "materials")
	_ = os.MkdirAll(directory, 0700)
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	settings, _ := json.Marshal(contracts.CodexSettings{Executable: *contentNativeExe, Model: *contentNativeModel, ReasoningEffort: "low", AccessMode: "read_only"})
	submit := func(requestID string, r contracts.RunRequest) string {
		r.Execution = contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settings}
		input, _ := json.Marshal(r)
		raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: requestID, Input: input})
		return call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	}
	// Full instructions and schema are fixed materials. No native output-schema
	// retry agent participates: the real final is parsed once by Runner.
	gen := submit("native-generate", contracts.RunRequest{Title: "Native JSON generation", Workflow: contracts.Workflow{ID: "generate", Version: 1}, Generate: &contracts.GenerateInput{Instruction: `Return exactly {"ok":true,"count":3}. Do not use tools.`, Materials: []contracts.Document{}, OutputSchema: json.RawMessage(`{"type":"object","required":["ok","count"],"properties":{"ok":{"const":true},"count":{"const":3}},"additionalProperties":false}`)}})
	generated := waitRunFor(t, dir, gen, func(v contracts.RunView) bool { return v.StopStatus == "stopped" }, 3*time.Minute)
	saveNative("native-generation", generated)
	if generated.ContentResult == nil || generated.ContentResult.Status != "succeeded" || len(generated.ContentResult.JSON) == 0 || generated.Status != "completed" || !generated.ContentResult.Source.Verified {
		t.Fatal("real structured generation failed", generated)
	}
	call(t, dir, []string{"history", "--run-id", gen}, "", 0)
	nonce := "cw-" + contracts.NewID()
	conversation := submit("native-conversation", contracts.RunRequest{Title: "Native continued conversation", Workflow: contracts.Workflow{ID: "conversation", Version: 1}, Conversation: &contracts.ConversationInput{Instruction: "Remember the token " + nonce + " for this session. Reply with this exact token only. Do not use tools.", Materials: []contracts.Document{}, Directory: contracts.DirectoryAccess{Path: directory, Access: "read_only"}}})
	idle := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.Controls.CanMessage || v.Status == "blocked" }, 3*time.Minute)
	saveNative("native-conversation-first", idle)
	if !idle.Controls.CanMessage || idle.ContentResult == nil || !strings.Contains(idle.ContentResult.Text, nonce) {
		t.Fatal("real initial conversation failed", idle)
	}
	original := idle.Execution.SessionID
	next := func(request, text string) contracts.RunView {
		before := runView(t, call(t, dir, []string{"inspect", "--run-id", conversation}, "", 0))
		contentCommand(t, dir, conversation, "message", request, map[string]any{"text": text}, 0)
		v := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool {
			return v.Controls.CanMessage && v.Execution.TurnID != before.Execution.TurnID || v.Status == "blocked"
		}, 3*time.Minute)
		if !v.Controls.CanMessage || v.Execution.SessionID != original {
			t.Fatal("continued session identity/state failed", v)
		}
		saveNative(request, v)
		return v
	}
	second := next("native-second", "Repeat the token from my first message exactly, with no tools.")
	if second.ContentResult == nil || !strings.Contains(second.ContentResult.Text, nonce) {
		t.Fatal("native context lost")
	}
	contentCommand(t, dir, conversation, "message", "native-long-turn", map[string]any{"text": "Use a shell tool to wait 45 seconds, then reply LONG_DONE. The command only waits and writes to standard output; do not change any files."}, 0)
	active := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.Controls.CanInterrupt }, time.Minute)
	saveNative("native-before-interrupt", active)
	contentCommand(t, dir, conversation, "interrupt", "native-interrupt", map[string]any{}, 0)
	interrupted := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.Controls.CanMessage }, time.Minute)
	saveNative("native-interrupted", interrupted)
	if interrupted.Closed || interrupted.ContentResult == nil || interrupted.ContentResult.Status != "cancelled" || interrupted.Execution.SessionID != original {
		t.Fatal("native interrupt did not retain original session", interrupted)
	}
	idle = next("native-after-interrupt", "Repeat the token from my first message exactly. Do not use tools.")
	if !strings.Contains(idle.ContentResult.Text, nonce) {
		t.Fatal("context lost after interrupt")
	}
	beforeHistory := call(t, dir, []string{"history", "--run-id", conversation}, "", 0)
	saveNative("native-history-before-restart", beforeHistory.Data)
	service = restartEdgeService(t, dir, service, 71)
	restarted := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.ResumeRequired }, time.Minute)
	saveNative("native-restarted", restarted)
	afterHistory := call(t, dir, []string{"history", "--run-id", conversation}, "", 0)
	beforeJSON, _ := json.Marshal(beforeHistory.Data)
	afterJSON, _ := json.Marshal(afterHistory.Data)
	if string(beforeJSON) != string(afterJSON) || restarted.Execution.SessionID != original || restarted.Controls.CanMessage {
		t.Fatal("restart changed history or auto-authorized input")
	}
	contentCommand(t, dir, conversation, "resume", "native-resume", map[string]any{}, 0)
	waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.Controls.CanMessage || v.Phase == "control_rejected" }, time.Minute)
	continued := next("native-after-restart", "Repeat the token from my first message exactly. Do not use tools.")
	if !strings.Contains(continued.ContentResult.Text, nonce) {
		t.Fatal("context lost across restart")
	}
	contentCommand(t, dir, conversation, "close", "native-close", map[string]any{}, 0)
	closed := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.StopStatus == "stopped" }, time.Minute)
	saveNative("native-closed", closed)
	contentCommand(t, dir, conversation, "message", "native-reject-closed", map[string]any{"text": "must reject"}, 5)
	finalHistory := call(t, dir, []string{"history", "--run-id", conversation}, "", 0)
	saveNative("native-history-final", finalHistory.Data)
	data, _ := json.Marshal(finalHistory.Data)
	var history struct {
		Entries []contracts.HistoryEntry `json:"entries"`
	}
	_ = json.Unmarshal(data, &history)
	turns := 0
	for _, entry := range history.Entries {
		if entry.Role == "assistant" {
			turns++
			if entry.SessionID != original {
				t.Fatal("mixed native source")
			}
		}
	}
	if turns != 5 || !closed.Closed || closed.Status != "closed" {
		t.Fatal("unexpected native turn budget or cleanup", turns, closed)
	}
	t.Logf("native=true generate=passed conversation=passed interrupt=passed restart=passed session=%s root=%s", original, root)
}
