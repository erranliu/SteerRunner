//go:build windows

package integration

import (
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func auditedRun(t *testing.T) (string, contracts.Request) {
	t.Helper()
	dir, envelope := setupRun(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var request contracts.RunRequest
	_ = json.Unmarshal(envelope.Input, &request)
	request.Workflow = contracts.Workflow{ID: workflow.AuditedDevelopment, Version: 1, AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}, PromptOverrides: map[string]string{workflow.StepAuditSpec: "Audit the current implementation against frozen Spec and Plan. Frozen prompt marker."}}
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
	if len(capabilities["workflows"].([]any)) != 2 || capabilities["workflow"].(map[string]any)["id"] != "development" {
		t.Fatal("workflow catalog missing")
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

func TestSRAuditedWorkflow(t *testing.T) {
	t.Run("repair_and_feedback", func(t *testing.T) {
		testCount.Add(1)
		dir, envelope := auditedRun(t)
		t.Setenv("STEERRUNNER_AUDIT_FAILURES", "1")
		id := submitAudited(t, dir, envelope)
		ready := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
		if ready.Audit == nil || ready.Audit.RepairCount != 1 || len(ready.Audit.Results) != 2 || ready.Execution.RequestedModel != "executor" || ready.PR == nil {
			t.Fatalf("audit view incomplete: %+v", ready)
		}
		if countNative(t, "turn/start") != 8 || countNative(t, "thread/start") != 1 {
			t.Fatal("unexpected audit/repair chain")
		}
		calls, _ := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
		if !strings.Contains(string(calls), "Frozen prompt marker.") {
			t.Fatal("prompt override lost")
		}
		raw, _ := json.Marshal(envelope)
		retry := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
		if retry.Data.(map[string]any)["run_id"] != id {
			t.Fatal("idempotency lost")
		}
		var changed contracts.RunRequest
		_ = json.Unmarshal(envelope.Input, &changed)
		changed.Workflow.AuditModel.Model = "other-model"
		conflict := envelope
		conflict.Input, _ = json.Marshal(changed)
		raw, _ = json.Marshal(conflict)
		call(t, dir, []string{"submit", "--input", "-"}, string(raw), 5)
		feedback := contracts.Request{Protocol: contracts.Protocol, RequestID: "audited-feedback", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d,"kind":"feedback","text":"update original work"}`, ready.Revision))}
		raw, _ = json.Marshal(feedback)
		call(t, dir, []string{"reply", "--run-id", id, "--input", "-"}, string(raw), 0)
		updated := waitRun(t, dir, id, func(r contracts.RunView) bool {
			return r.Status == "pr_ready" && r.PR != nil && r.PR.HeadRevision != ready.PR.HeadRevision
		})
		if countNative(t, "turn/start") != 13 || updated.Execution.SessionID != ready.Execution.SessionID || updated.PR.URL != ready.PR.URL || updated.Audit.RepairCount != 1 {
			t.Fatal("feedback skipped audits, reset budget or replaced original session/PR")
		}
	})
	for _, scenario := range []string{"budget", "bad_json", "unavailable", "question_restart", "unknown_audit"} {
		t.Run(scenario, func(t *testing.T) {
			testCount.Add(1)
			dir, envelope := auditedRun(t)
			state := os.Getenv("STEERRUNNER_FIXTURE_STATE")
			switch scenario {
			case "budget":
				t.Setenv("STEERRUNNER_AUDIT_FAILURES", "3")
			case "bad_json":
				_ = os.WriteFile(filepath.Join(state, "audit-bad-json"), []byte("true"), 0600)
			case "unavailable":
				_ = os.WriteFile(filepath.Join(state, "audit-unavailable"), []byte("true"), 0600)
			case "question_restart", "unknown_audit":
				_ = os.WriteFile(filepath.Join(state, "audit-question"), []byte("once"), 0600)
			}
			id := submitAudited(t, dir, envelope)
			if scenario == "question_restart" || scenario == "unknown_audit" {
				question := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "waiting_input" && r.Question != nil })
				if question.Step != workflow.StepAuditSpec || question.Execution.RequestedModel != "reviewer" {
					t.Fatal("wrong question stage")
				}
				if scenario == "question_restart" {
					info := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0))
					call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("audit-service-stop", info.InstanceID), 0)
					if err := waitOwnedService(info); err != nil {
						t.Fatal(err)
					}
					newInfo := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
					cleanupService(t, dir, newInfo)
					current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.ResumeRequired })
					if current.NewInputsAllowed || countNative(t, "turn/start") != 3 {
						t.Fatal("restart dispatched audit")
					}
					writeCommand(t, dir, "resume", id, "audit-resume", current.Revision)
					question = waitRun(t, dir, id, func(r contracts.RunView) bool { return !r.ResumeRequired && r.NewInputsAllowed && r.Question != nil })
				} else {
					_ = os.WriteFile(filepath.Join(state, "drop-next-start"), []byte("drop audit receipt"), 0600)
				}
				auditControl(t, dir, id, "reply", map[string]any{"kind": "answer", "question_id": question.Question.ID, "text": "yes"})
				if scenario == "unknown_audit" {
					waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Phase == "operation_outcome_unknown" })
					before := countNative(t, "turn/start")
					time.Sleep(1100 * time.Millisecond)
					if before != 4 || countNative(t, "turn/start") != before {
						t.Fatal("unknown audit replayed")
					}
					auditControl(t, dir, id, "stop", map[string]any{})
					waitRun(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" })
					return
				}
				ready := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
				if ready.Execution.SessionID != question.Execution.SessionID || countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 6 || ready.Audit.RepairCount != 0 {
					t.Fatal("answer/resume left original audit")
				}
				return
			}
			blocked := waitRun(t, dir, id, func(r contracts.RunView) bool {
				return r.Status == "blocked" || scenario == "unavailable" && r.Phase == "operation_outcome_unknown"
			})
			if blocked.PR != nil || countNative(t, "thread/start") != 1 {
				t.Fatal("failed audit published or replaced session")
			}
			if scenario == "budget" && (blocked.Audit.RepairCount != 2 || blocked.Diagnostic != "audit_repair_limit" || countNative(t, "turn/start") != 9) {
				t.Fatal("repair budget not enforced", blocked)
			}
		})
	}
}
