//go:build windows && live

package integration

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var contentNativeExe = flag.String("content-native-exe", "", "Explicit existing native Codex 0.159.2; never installed by this driver")
var contentNativeModel = flag.String("content-native-model", "", "Optional model available to the existing native account")

// Focused public-CLI proof for final-only takeover, independent of development
// configuration and of the multi-turn native acceptance scenario below.
func TestSRContentGenerateTakeoverNative(t *testing.T) {
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
	root, err := os.MkdirTemp("C:/srt", "gt-")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "state")
	save := func(name string, value any) {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(*evidenceFlag, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save("isolation", map[string]string{"root": root, "runner": runnerEXE, "native": *contentNativeExe, "model": *contentNativeModel})
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	settings := contracts.CodexSettings{Executable: *contentNativeExe, Model: *contentNativeModel, ReasoningEffort: "low", AccessMode: "read_only"}
	settingsRaw, _ := json.Marshal(settings)
	material := "fixed-takeover-20261020"
	request := contracts.RunRequest{Title: "Generate takeover proof", Workflow: contracts.Workflow{ID: "generate", Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settingsRaw}, Generate: &contracts.GenerateInput{Instruction: `Return exactly {"ok":true,"token":"fixed-takeover-20261020"}. Do not use tools.`, Materials: []contracts.Document{{Kind: "inline", Path: "token.txt", Content: &material}}, OutputSchema: json.RawMessage(`{"type":"object","required":["ok","token"],"properties":{"ok":{"const":true},"token":{"const":"fixed-takeover-20261020"}},"additionalProperties":false}`)}}
	input, _ := json.Marshal(request)
	envelope := contracts.Request{Protocol: contracts.Protocol, RequestID: "native-generate-takeover", Input: input}
	save("submit-request", envelope)
	raw, _ := json.Marshal(envelope)
	submitted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	save("submit-response", submitted)
	id := submitted.Data.(map[string]any)["run_id"].(string)
	generated := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.ContentResult != nil && v.StopStatus == "stopped" }, 3*time.Minute)
	if generated.Status != "completed" || generated.ContentResult.Status != "succeeded" || !generated.ContentResult.Source.Verified || len(generated.ContentResult.JSON) == 0 {
		t.Fatal("real generation failed", generated)
	}
	save("content-result-response", call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	stopRaw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "native-generate-explicit-stop", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, generated.Revision))})
	save("explicit-stop-response", call(t, dir, []string{"stop", "--run-id", id, "--input", "-"}, string(stopRaw), 0))
	stopped := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" }, time.Minute)
	if stopped.ContentResult == nil || !bytes.Equal(mustJSON(generated.ContentResult), mustJSON(stopped.ContentResult)) {
		t.Fatal("stop replaced original content result")
	}
	save("stopped-inspect-response", call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	save("service-stop-response", call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("native-generate-service-stop", service.InstanceID), 0))
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	proof := call(t, dir, []string{"service", "verify-stopped"}, "", 0)
	save("verify-stopped-response", proof)
	var settlement contracts.StoppedVerification
	proofRaw, _ := json.Marshal(proof.Data)
	if err := json.Unmarshal(proofRaw, &settlement); err != nil || !settlement.ResourcesSettled {
		t.Fatal("resource settlement unverified", proof)
	}
	before, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	response := call(t, dir, []string{"service", "verify-takeover", "--candidate", runnerEXE}, "", 0)
	save("verify-takeover-response", response)
	var report contracts.TakeoverReport
	reportRaw, _ := json.Marshal(response.Data)
	if err := json.Unmarshal(reportRaw, &report); err != nil || !report.Compatible || len(report.Items) != 1 {
		t.Fatal(response)
	}
	item := report.Items[0]
	if item.ID != id || item.InputDigest != generated.InputDigest || item.RequestID != envelope.RequestID || item.RequestDigest != storage.RequestDigest("submit", "", envelope) || item.Status != "compatible" || item.Revision != stopped.Revision {
		t.Fatal("original bindings changed", item)
	}
	after, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("offline takeover changed original database", err)
	}
	save("database-unchanged", map[string]any{"unchanged": true, "sha256": resultacquisition.Digest(before), "run_id": id, "input_digest": generated.InputDigest})
	t.Logf("native=true root=%s run=%s result=passed settlement=passed takeover=compatible original_database=unchanged", root, id)
}

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
	originalProcess := contentProcess(t, dir, conversation)
	service = restartEdgeService(t, dir, service, 71)
	restarted := waitRunFor(t, dir, conversation, func(v contracts.RunView) bool { return v.Controls.CanMessage }, time.Minute)
	saveNative("native-restarted", restarted)
	afterHistory := call(t, dir, []string{"history", "--run-id", conversation}, "", 0)
	beforeJSON, _ := json.Marshal(beforeHistory.Data)
	afterJSON, _ := json.Marshal(afterHistory.Data)
	if string(beforeJSON) != string(afterJSON) || restarted.Execution.SessionID != original || restarted.ResumeRequired || contentProcess(t, dir, conversation) != originalProcess {
		t.Fatal("restart changed history, process or original authorization")
	}
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
