//go:build windows

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Explicit supplemental controlled case: requires the real pre-change EXE.
// It does not deploy an installation or create a real PR.
func TestSRAuditPromptDefaultUpgrade(t *testing.T) {
	if *baselineFlag == "" {
		t.Skip("explicit pre-change Runner required")
	}
	testCount.Add(1)
	candidate := runnerEXE
	defer func() { runnerEXE = candidate }()
	read := func(dir string) *runstate.Record {
		t.Helper()
		r, err := storage.OpenReader(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		runs, err := r.Runs()
		if err != nil || len(runs) != 1 {
			t.Fatal("one original Run required", err)
		}
		return runs[0]
	}
	for _, mode := range []string{"original-frozen", "new-default"} {
		t.Run(mode, func(t *testing.T) {
			runnerEXE = candidate
			if mode == "original-frozen" {
				runnerEXE = *baselineFlag
			}
			dir, envelope := auditedRun(t)
			t.Setenv("STEERRUNNER_SUCCESS_OUTPUT", "1")
			t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
			var req contracts.RunRequest
			json.Unmarshal(envelope.Input, &req)
			req.Workflow.PromptOverrides = nil
			req.Publication.Settings, _ = json.Marshal(contracts.CommandProviderSettings{BaseURL: "https://controlled.invalid", Query: contracts.QuerySettings{Executable: helperEXE, Args: []string{"--query", os.Getenv("STEERRUNNER_FIXTURE_PROVIDER"), "{pr_url}"}}})
			envelope.Input, _ = json.Marshal(req)
			hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-audit")
			if mode == "original-frozen" {
				if err := os.WriteFile(hold, nil, 0600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(hold) })
			}
			info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, info)
			raw, _ := json.Marshal(envelope)
			id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
			var original *runstate.Record
			if mode == "original-frozen" {
				waitRunFor(t, dir, id, func(v contracts.RunView) bool {
					return v.Step == workflow.StepAuditCode && v.Execution.TurnStarted && countNative(t, "turn/start") == 2
				}, 2*time.Minute)
				original = read(dir)
				if strings.Contains(original.PromptBodies["audit.code"], "单一 JSON") {
					t.Fatal("baseline did not contain the original prompt")
				}
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("prompt-upgrade-stop", info.InstanceID), 0)
				if err := waitOwnedService(info); err != nil {
					t.Fatal(err)
				}
				runnerEXE = candidate
				preflight := call(t, dir, []string{"service", "verify-takeover", "--candidate", candidate}, "", 0)
				wire, _ := json.Marshal(preflight.Data)
				var report contracts.TakeoverReport
				if json.Unmarshal(wire, &report) != nil || !report.Compatible {
					t.Fatal("old frozen audit prompt rejected", preflight)
				}
				info = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				cleanupService(t, dir, info)
				if err := os.Remove(hold); err != nil {
					t.Fatal(err)
				}
			}
			ready := waitRunFor(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" || v.Status == "blocked" }, 2*time.Minute)
			after := read(dir)
			if ready.Status != "pr_ready" || ready.ResumeRequired || countNative(t, "turn/start") != 3 {
				t.Fatal("prompt change retried business or required extra acquisition", ready)
			}
			if original != nil {
				if original.InputDigest != after.InputDigest || original.PromptBodies["audit.code"] != after.PromptBodies["audit.code"] || original.Workflow.Config.States[workflow.StepAuditCode].Prompt != after.Workflow.Config.States[workflow.StepAuditCode].Prompt {
					t.Fatal("candidate rewrote original frozen source")
				}
			} else if !strings.Contains(after.PromptBodies["audit.code"], "单一 JSON") {
				t.Fatal("new Run did not freeze the new default")
			}
			if err := os.WriteFile(filepath.Join(*evidenceFlag, "audit-prompt-"+mode+".json"), marshalEvidence(t, map[string]any{"run_id": id, "input_digest": after.InputDigest, "frozen_audit_prompt": after.PromptBodies["audit.code"], "business_turns": 3, "same_original_source": original != nil, "pr_ready": true}), 0600); err != nil {
				t.Fatal(err)
			}
			auditControl(t, dir, id, "stop", map[string]any{})
		})
	}
}
