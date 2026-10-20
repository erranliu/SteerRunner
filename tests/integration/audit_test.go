//go:build windows

package integration

import (
	"encoding/base64"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSRResultAcquisition(t *testing.T) {
	testCount.Add(1)
	for _, scenario := range []string{"plain-marker", "audit-zero", "audit-positive", "audit-offline", "audit-report", "parser-unsupported", "parser-complete", "parser-restart", "parser-kill", "parser-stop", "parser-tool"} {
		t.Run(scenario, func(t *testing.T) {
			selectedRunner := runnerEXE
			t.Cleanup(func() { runnerEXE = selectedRunner })
			if *baselineFlag != "" && (scenario == "parser-restart" || scenario == "parser-kill") {
				runnerEXE = *baselineFlag
			}
			dir, envelope := setupRun(t)
			hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-parser")
			if scenario == "audit-offline" {
				t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
				hold = filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-audit")
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
				config, _ := json.Marshal(map[string]any{"codex_parser": map[string]any{"enabled": true, "executable": exe}})
				if err := os.WriteFile(filepath.Join(dir, "service", "result-acquisition.json"), config, 0600); err != nil {
					t.Fatal(err)
				}
				if scenario == "parser-restart" || scenario == "parser-kill" || scenario == "parser-stop" {
					if err := os.WriteFile(hold, nil, 0600); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Remove(hold) })
					t.Setenv("STEERRUNNER_PARSER_CASE", "PARSER_WAIT("+base64.RawURLEncoding.EncodeToString([]byte(hold))+")")
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
				until := time.Now().Add(15 * time.Second)
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
			if scenario == "parser-restart" || scenario == "parser-kill" || scenario == "parser-stop" {
				active := waitRunFor(t, dir, id, func(r contracts.RunView) bool {
					return r.Step == workflow.StepAuditParse && r.Phase == "parser_running"
				}, 5*time.Minute)
				if len(active.ResultAcquisition) < 2 || countNative(t, "turn/start") != 2 {
					t.Fatal("parser did not start from the completed original audit")
				}
				reader, err := storage.OpenRecoveryReader(dir)
				if err != nil {
					t.Fatal(err)
				}
				runs, err := reader.Runs()
				reader.Close()
				if err != nil || len(runs) != 1 {
					t.Fatal(err)
				}
				var requestFound bool
				for _, intent := range runs[0].Intents {
					if intent.Request.Input.Parse == nil {
						continue
					}
					requestFound = true
					started, err := parser.ReadStartedReceipt(dir, intent.Request)
					if err != nil || started.Process == nil {
						t.Fatalf("original parser identity: %v", err)
					}
					if scenario == "parser-stop" {
						current := waitRun(t, dir, id, func(r contracts.RunView) bool { return true })
						writeCommand(t, dir, "stop", id, "explicit-parser-stop", current.Revision)
						waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" }, 5*time.Minute)
						if process.InspectProcess(started.Process) != "process absent" || countNative(t, "turn/start") != 2 {
							t.Fatal("explicit stop retained parser or sent successor")
						}
						return
					}
					if scenario == "parser-kill" {
						if err := killOwnedService(info); err != nil {
							t.Fatal(err)
						}
					} else {
						call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("parser-restart-stop", info.InstanceID), 0)
					}
					if err := waitOwnedService(info); err != nil {
						t.Fatal(err)
					}
					if process.InspectProcess(started.Process) != "matching process still exists" {
						t.Fatal("service exit killed original parser")
					}
					if *baselineFlag != "" {
						retainCompatibilityDB(t, dir, "ebr-"+scenario+"-original-v4.db")
					}
					if err := os.Remove(hold); err != nil {
						t.Fatal(err)
					}
					until := time.Now().Add(15 * time.Second)
					for {
						if receipt, err := parser.ReadReceipt(dir, intent.Request); err == nil && receipt.State.Stopped {
							break
						}
						if time.Now().After(until) {
							t.Fatal("original parser did not finish offline")
						}
						time.Sleep(20 * time.Millisecond)
					}
				}
				if !requestFound {
					t.Fatal("original parser intent missing")
				}
				if err := waitOwnedService(info); err != nil {
					t.Fatal(err)
				}
				if *baselineFlag != "" {
					runnerEXE = selectedRunner
				}
				info = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				cleanupService(t, dir, info)
				starts, err := os.ReadFile(hold + ".starts")
				if err != nil || string(starts) != "start\n" {
					t.Fatal("original parser was replayed", err)
				}
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
	expected := map[string]int{"development": 2, "development-audited": 2, "conversation": 1, "generate": 1}
	profiles := capabilities["workflows"].([]any)
	if len(profiles) != len(expected) || capabilities["workflow"].(map[string]any)["id"] != "development" {
		t.Fatal("workflow catalog missing")
	}
	for _, profile := range profiles {
		item := profile.(map[string]any)
		id, _ := item["id"].(string)
		version, _ := item["version"].(float64)
		if want, found := expected[id]; !found || version != float64(want) {
			t.Fatal("missing, duplicate or unsupported workflow", item)
		}
		delete(expected, id)
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
