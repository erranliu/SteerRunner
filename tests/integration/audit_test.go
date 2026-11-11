//go:build windows

package integration

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSRResultAcquisition(t *testing.T) {
	testCount.Add(1)
	for _, scenario := range selectedControlledCases("TestSRResultAcquisition") {
		runControlledCase(t, "TestSRResultAcquisition", scenario, func(t *testing.T) {
			selectedRunner := runnerEXE
			t.Cleanup(func() { runnerEXE = selectedRunner })
			dir, envelope := setupRun(t)
			hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-audit")
			if scenario == "audit-offline" {
				t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
				if err := os.WriteFile(hold, nil, 0600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(hold) })
			}
			t.Setenv("STEERRUNNER_SUCCESS_OUTPUT", "1")
			var req contracts.RunRequest
			_ = json.Unmarshal(envelope.Input, &req)
			if scenario == "audit-offline" {
				req.Publication.Settings, _ = json.Marshal(contracts.CommandProviderSettings{BaseURL: "https://controlled.invalid", Query: contracts.QuerySettings{Executable: helperEXE, Args: []string{"--query", os.Getenv("STEERRUNNER_FIXTURE_PROVIDER"), "{pr_url}"}}})
			}
			audited := scenario != "plain-marker"
			if audited {
				req.Workflow = contracts.Workflow{ID: workflow.AuditedDevelopment, Version: 2, AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}}
				var settings contracts.CodexSettings
				_ = json.Unmarshal(req.Execution.Settings, &settings)
				settings.Model = "executor"
				settings.ReasoningEffort = "medium"
				req.Execution.Settings, _ = json.Marshal(settings)
				t.Setenv("STEERRUNNER_AUDIT_EXPECT_MODELS", "1")
			}
			if scenario == "audit-positive" {
				t.Setenv("STEERRUNNER_AUDIT_FIX", "1")
			}
			if scenario == "audit-report" || strings.HasPrefix(scenario, "parser-") {
				t.Setenv("STEERRUNNER_AUDIT_PLAIN", "1")
			}
			if strings.HasPrefix(scenario, "parser-") {
				if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
					t.Fatal(err)
				}
				var settings contracts.CodexSettings
				_ = json.Unmarshal(req.Execution.Settings, &settings)
				exe := settings.Executable
				if scenario == "parser-unsupported" {
					exe = "missing-parser.exe"
				}
				config, _ := json.Marshal(map[string]any{"codex_parser": map[string]any{"enabled": true, "executable": exe, "model": "internal-parser", "reasoning_effort": "low"}})
				if err := os.WriteFile(filepath.Join(dir, "service", "result-acquisition.json"), config, 0600); err != nil {
					t.Fatal(err)
				}

				if scenario == "parser-tool" {
					t.Setenv("STEERRUNNER_PARSER_CASE", "PARSER_TOOL")
				}
			}
			envelope.Input, _ = json.Marshal(req)
			info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, info)
			raw, _ := json.Marshal(envelope)
			accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			id := accepted.Data.(map[string]any)["run_id"].(string)
			if scenario == "audit-offline" {
				active := waitRunFor(t, dir, id, func(r contracts.RunView) bool {
					return r.Step == workflow.StepAuditCode && r.Execution.TurnStarted && countNative(t, "turn/start") == 2
				}, 5*time.Minute)
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("audit-offline", info.InstanceID), 0)
				if err := waitOwnedService(info); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(hold); err != nil {
					t.Fatal(err)
				}
				// The original owner's Git checkpoint alone allows 30 seconds.
				until := time.Now().Add(time.Minute)
				for {
					files, _ := filepath.Glob(filepath.Join(dir, "runs", id, "app-server", "owner", "checkpoints", "*.json"))
					if len(files) >= 2 {
						break
					}
					if time.Now().After(until) {
						t.Fatalf("offline audit checkpoint not collected: %+v", active)
					}
					time.Sleep(20 * time.Millisecond)
				}
				if countNative(t, "turn/start") != 2 {
					t.Fatal("successor sent while service offline")
				}
				info = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				cleanupService(t, dir, info)
			}
			// This is the whole multi-turn flow, including original owner Git
			// checkpoints and optional parsing. Keep the per-operation deadlines;
			// allow the complete sequence to finish on a busy Windows test host.
			ready := waitRunFor(t, dir, id, func(r contracts.RunView) bool {
				return r.Status == "pr_ready" || r.Status == "blocked" || r.Status == "waiting_input"
			}, 5*time.Minute)
			if ready.Status != "pr_ready" {
				t.Fatalf("%s: %s %s", scenario, ready.Step, ready.Diagnostic)
			}
			wantTurns := 2
			if audited {
				wantTurns = 3
			}
			if scenario == "audit-report" || scenario == "parser-unsupported" || scenario == "parser-tool" {
				wantTurns = 4
			}
			if countNative(t, "turn/start") != wantTurns || countNative(t, "thread/start") != 1 {
				t.Fatalf("wrong native budget: %d", countNative(t, "turn/start"))
			}
			if audited {
				if ready.Audit == nil || len(ready.Audit.Results) != 1 {
					t.Fatal("missing audit provenance")
				}
				a := ready.Audit.Results[0]
				if a.SourceResultID == "" || a.PostHead != ready.PR.HeadRevision || a.CommandID == "" || a.TurnID == "" {
					t.Fatal(a)
				}
				if scenario == "audit-positive" && (a.PreHead == a.PostHead || a.FixedCount == nil || *a.FixedCount != 1) {
					t.Fatal(a)
				}
				if scenario == "parser-unsupported" {
					found := false
					for _, r := range ready.ResultAcquisition {
						for _, attempt := range r.Attempts {
							if attempt.Method == "llm_parser" && attempt.Status == "failed" {
								found = true
							}
						}
					}
					if !found {
						t.Fatal("unsupported qualification was hidden")
					}
				}
				if scenario == "parser-complete" {
					found := false
					for _, result := range ready.ResultAcquisition {
						if result.Method == "llm_parser" && result.Status == "satisfied" {
							found = true
						}
					}
					if !found {
						t.Fatal("independent result not accepted")
					}
				}
				if scenario == "parser-complete" || scenario == "parser-tool" {
					var usage api.Snapshot
					if err := json.Unmarshal(marshalEvidence(t, call(t, dir, []string{"run-usage", "--run-id", id}, "", 0).Data), &usage); err != nil {
						t.Fatal(err)
					}
					if usage.Scope != "main_session" || usage.ModelCallCoverage == nil || usage.ModelCallCoverage.InternalRecorded != 1 {
						t.Fatal("internal call coverage missing", usage)
					}
					var page api.Page
					if err := json.Unmarshal(marshalEvidence(t, call(t, dir, []string{"run-usage-events", "--run-id", id, "--snapshot-revision", strconv.FormatUint(usage.Revision, 10)}, "", 0).Data), &page); err != nil {
						t.Fatal(err)
					}
					internal := 0
					for _, event := range page.Events {
						if event.Scope != "runner_internal" {
							continue
						}
						internal++
						if event.CallID == "" || event.CallSource != "llm_parser" || event.CommandID != a.CommandID || event.SourceResultID == "" || event.StepID != workflow.StepAuditParse || event.ConfiguredModel != "internal-parser" || event.ConfiguredReasoningEffort != "low" || event.SessionID == ready.Execution.SessionID {
							t.Fatal("internal call lost original source or model", event)
						}
						if scenario == "parser-complete" && (event.Decision != "counted" || event.CoverageStatus != "covered" || event.Tokens.Total == nil || *event.Tokens.Total != 2) {
							t.Fatal("parser usage lost or reattributed", event)
						}
						if scenario == "parser-tool" && (event.Decision != "uncovered" || event.CoverageStatus != "uncovered" || event.Tokens.Total != nil) {
							t.Fatal("uncovered model call became zero", event)
						}
					}
					if internal != 1 {
						t.Fatal("internal call missing or duplicated", page)
					}
				}
			}
		})
	}
}

func auditedRun(t *testing.T) (string, contracts.Request) {
	t.Helper()
	dir, envelope := setupRun(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var request contracts.RunRequest
	_ = json.Unmarshal(envelope.Input, &request)
	request.Workflow = contracts.Workflow{ID: workflow.AuditedDevelopment, Version: 2, AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}, PromptOverrides: map[string]string{"audit.code": "审计当前代码，直接修复 P0–P2 并验证、提交。报告实际修复数量及内容，遗留问题如实说明。"}}
	var settings contracts.CodexSettings
	_ = json.Unmarshal(request.Execution.Settings, &settings)
	settings.Model, settings.ReasoningEffort = "executor", "medium"
	request.Execution.Settings, _ = json.Marshal(settings)
	envelope.Input, _ = json.Marshal(request)
	t.Setenv("STEERRUNNER_AUDIT_EXPECT_MODELS", "1")
	return dir, envelope
}

func submitAudited(t *testing.T, dir string, envelope contracts.Request) string {
	t.Helper()
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(envelope)
	return call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
}

// Only a confirmed stale-revision rejection permits a fresh control request.
func auditControl(t *testing.T, dir, id, command string, fields map[string]any) {
	t.Helper()
	for attempt := 0; attempt < 5; attempt++ {
		current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Revision > 0 })
		fields["expected_revision"] = current.Revision
		input, _ := json.Marshal(fields)
		raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "audit-control-" + contracts.NewID(), Input: input})
		response, code, stderr, err := invoke(runnerEXE, []string{command, "--run-id", id, "--input", "-", "--state-dir", dir, "--json"}, string(raw))
		if err == nil && code == 0 {
			return
		}
		if err != nil || response.Error == nil || response.Error.Code != "stale_revision" {
			t.Fatalf("audit control failed: code=%d err=%v response=%+v stderr=%s", code, err, response, stderr)
		}
	}
	t.Fatal("audit control revision did not settle")
}

func TestSRWorkflowDiscoveryAndSelection(t *testing.T) {
	testCount.Add(1)
	dir, envelope := auditedRun(t)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	capabilities := call(t, dir, []string{"capabilities"}, "", 0).Data.(map[string]any)
	type selection struct {
		id      string
		version float64
	}
	expected := map[selection]bool{
		{"development", 2}: true, {"development-audited", 2}: true,
		{"conversation", 1}: true, {"conversation", 2}: true,
		{"generate", 1}: true, {"execute", 1}: true,
	}
	profiles := capabilities["workflows"].([]any)
	if len(profiles) != len(expected) || capabilities["workflow"].(map[string]any)["id"] != "development" {
		t.Fatal("workflow catalog missing")
	}
	for _, profile := range profiles {
		item := profile.(map[string]any)
		id, _ := item["id"].(string)
		version, _ := item["version"].(float64)
		key := selection{id, version}
		if !expected[key] {
			t.Fatal("missing, duplicate or unsupported workflow", item)
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		t.Fatal("workflow catalog omitted an expected workflow", expected)
	}
	var request contracts.RunRequest
	_ = json.Unmarshal(envelope.Input, &request)
	request.Workflow.ID = "unknown"
	invalid := envelope
	invalid.Input, _ = json.Marshal(request)
	raw, _ := json.Marshal(invalid)
	call(t, dir, []string{"submit", "--input", "-"}, string(raw), 3)
	if countNative(t, "thread/start") != 0 {
		t.Fatal("invalid selection started agent")
	}
}
