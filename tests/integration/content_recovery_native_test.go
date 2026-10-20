//go:build windows && live

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// This uses the public CLI and real native process. It owns only new content
// directories, performs no Git/PR writes, and never retries an unknown input.
func TestSRContentNativeGenerationRecovery(t *testing.T) {
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
	candidate := buildCompatibleCandidate(t)
	for _, mode := range []string{"graceful", "kill", "compatible-image", "startup-stop"} {
		t.Run(mode, func(t *testing.T) {
			root, err := os.MkdirTemp("C:/srt", "cw-recovery-")
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "state")
			service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, service)
			nonce := "RECOVERY_" + contracts.NewID()
			settings, _ := json.Marshal(contracts.CodexSettings{Executable: *contentNativeExe, Model: *contentNativeModel, ReasoningEffort: "low", AccessMode: "read_only"})
			request := contracts.RunRequest{Title: "Native generation recovery " + mode, Workflow: contracts.Workflow{ID: "conversation", Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settings}, Conversation: &contracts.ConversationInput{Instruction: fmt.Sprintf("Remember token %s. Use the shell tool to execute exactly: Start-Sleep -Seconds 20; Write-Output '%s'. Then return exactly the token. The command only waits and writes stdout; do not change files or use other tools.", nonce, nonce), Materials: []contracts.Document{}, Directory: contracts.DirectoryAccess{Path: root, Access: "read_only"}}}
			input, _ := json.Marshal(request)
			raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "native-recovery-submit", Input: input})
			id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
			active := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanInterrupt && v.Execution.TurnID != "" }, time.Minute)
			owner := contentProcess(t, dir, id)
			tool := waitNativeRecoveryTool(t, dir, id, nonce, active, owner.Identity)
			if mode == "compatible-image" {
				check := call(t, dir, []string{"service", "verify-takeover", "--candidate", candidate}, "", 0)
				var report contracts.TakeoverReport
				if json.Unmarshal(marshalEvidence(t, check.Data), &report) != nil || !report.Compatible {
					t.Fatal("real original execution cannot be adopted", check)
				}
			}
			if mode == "kill" {
				if err := killOwnedService(service); err != nil {
					t.Fatal(err)
				}
			} else {
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("native-recovery-detach", service.InstanceID), 0)
				if err := waitOwnedService(service); err != nil {
					t.Fatal(err)
				}
			}
			if process.InspectProcess(&owner.Identity) != "matching process still exists" {
				t.Fatal("service exit killed native execution")
			}
			if process.InspectProcess(&tool.Process) != "matching process still exists" {
				t.Fatal("original active tool did not survive service exit")
			}
			if mode == "compatible-image" {
				previous := runnerEXE
				runnerEXE = candidate
				t.Cleanup(func() { runnerEXE = previous })
			}
			args := []string{"service", "start"}
			startInput := ""
			if mode == "startup-stop" {
				startInput = string((contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: id, Request: offlineContentStop(t, dir, id, "native-original-stop")}}}).JSON())
				args = append(args, "--input", "-")
			}
			next := serviceInfo(t, call(t, dir, args, startInput, 0))
			cleanupService(t, dir, next)
			var result contracts.RunView
			if mode == "startup-stop" {
				result = waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" }, time.Minute)
				if !result.Closed {
					t.Fatal("startup stop left content open", result)
				}
				if process.InspectProcess(&tool.Process) != "process absent" {
					t.Fatal("startup stop retained original native tool")
				}
			} else {
				result = waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage || v.Status == "blocked" }, 3*time.Minute)
				if !result.Controls.CanMessage || result.ResumeRequired || result.ContentResult == nil || !strings.Contains(result.ContentResult.Text, nonce) || result.Execution.SessionID != active.Execution.SessionID || result.Execution.TurnID != active.Execution.TurnID || contentProcess(t, dir, id) != owner {
					t.Fatal("native original execution was lost, replaced or replayed", result)
				}
				history := call(t, dir, []string{"history", "--run-id", id}, "", 0)
				var page struct {
					Entries []contracts.HistoryEntry `json:"entries"`
				}
				if err := json.Unmarshal(marshalEvidence(t, history.Data), &page); err != nil {
					t.Fatal(err)
				}
				answers := 0
				for _, entry := range page.Entries {
					if entry.Role == "assistant" {
						answers++
						if entry.SessionID != active.Execution.SessionID || entry.TurnID != active.Execution.TurnID {
							t.Fatal("foreign native completion", entry)
						}
					}
				}
				if answers != 1 {
					t.Fatal("native input replayed", answers)
				}
				contentCommand(t, dir, id, "close", "close-native-recovered", map[string]any{}, 0)
				result = waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" }, time.Minute)
			}
			contentCommand(t, dir, id, "message", "reject-native-closed", map[string]any{"text": "must reject"}, 5)
			if process.InspectProcess(&tool.Process) != "process absent" {
				t.Fatal("closed native Run retained original shell process")
			}
			call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("native-final-stop", next.InstanceID), 0)
			if err := waitOwnedService(next); err != nil {
				t.Fatal(err)
			}
			call(t, dir, []string{"service", "verify-stopped"}, "", 0)
			evidence := map[string]any{"mode": mode, "root": root, "original": active, "process": owner, "active_tool": tool, "settled": result, "instance_before": service.InstanceID, "instance_after": next.InstanceID}
			if err := os.WriteFile(filepath.Join(*evidenceFlag, "native-recovery-"+mode+".json"), marshalEvidence(t, evidence), 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}
