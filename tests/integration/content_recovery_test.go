//go:build windows

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestSRContentGenerationRecovery(t *testing.T) {
	testCount.Add(1)
	for _, mode := range []string{"generate-graceful", "conversation-kill", "question-reply", "startup-stop"} {
		t.Run(mode, func(t *testing.T) {
			marker := "hold-turn"
			if mode == "question-reply" {
				marker = "content-question"
			}
			t.Setenv("STEERRUNNER_CONTENT_MARKER", marker)
			t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
			flow := "conversation"
			if mode == "generate-graceful" {
				flow = "generate"
			}
			dir, id, service := contentFixture(t, flow, nil)
			original := waitRun(t, dir, id, func(v contracts.RunView) bool {
				if mode == "question-reply" {
					return v.Question != nil
				}
				return v.Execution.TurnID != "" && (flow == "generate" || v.Controls.CanInterrupt)
			})
			owner := contentProcess(t, dir, id)
			history := call(t, dir, []string{"history", "--run-id", id}, "", 0)
			if mode == "conversation-kill" {
				if err := killOwnedService(service); err != nil {
					t.Fatal(err)
				}
			} else {
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("content-detach", service.InstanceID), 0)
				if err := waitOwnedService(service); err != nil {
					t.Fatal(err)
				}
			}
			if process.InspectProcess(&owner.Identity) != "matching process still exists" || countNative(t, "turn/start") != 1 {
				t.Fatal("service exit stopped or replaced original execution")
			}
			input := ""
			if mode == "startup-stop" {
				stop := offlineContentStop(t, dir, id, "original-content-stop")
				input = string((contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: id, Request: stop}}}).JSON())
			}
			args := []string{"service", "start"}
			if input != "" {
				args = append(args, "--input", "-")
			}
			next := serviceInfo(t, call(t, dir, args, input, 0))
			cleanupService(t, dir, next)
			if mode == "startup-stop" {
				stopped := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
				if !stopped.Closed || countNative(t, "turn/start") != 1 {
					t.Fatal("startup stop permitted a successor input", stopped)
				}
				contentCommand(t, dir, id, "message", "closed-reject", map[string]any{"text": "must reject"}, 5)
				return
			}
			restored := waitRun(t, dir, id, func(v contracts.RunView) bool {
				if mode == "question-reply" {
					return v.Question != nil
				}
				return (flow == "generate" || v.Controls.CanInterrupt) && v.Execution.TurnID != ""
			})
			if restored.Execution.SessionID != original.Execution.SessionID || restored.Execution.TurnID != original.Execution.TurnID || restored.ResumeRequired || contentProcess(t, dir, id) != owner || !bytes.Equal(marshalEvidence(t, history.Data), marshalEvidence(t, call(t, dir, []string{"history", "--run-id", id}, "", 0).Data)) {
				t.Fatal("generation changed original session, turn, process, history or authorization", restored)
			}
			if mode == "question-reply" {
				if restored.Question.ID != original.Question.ID {
					t.Fatal("question identity changed")
				}
				raw := contentCommand(t, dir, id, "reply", "original-answer", map[string]any{"kind": "answer", "question_id": original.Question.ID, "text": "one"}, 0)
				call(t, dir, []string{"reply", "--run-id", id, "--input", "-"}, raw, 0)
				waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage })
				if countNative(t, "turn/steer") != 1 {
					t.Fatal("replayed original answer")
				}
			} else {
				if err := os.Remove(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")); err != nil {
					t.Fatal(err)
				}
				waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanMessage || v.StopStatus == "stopped" })
			}
			if countNative(t, "turn/start") != 1 || countNative(t, "thread/start") != 1 {
				t.Fatal("generation replayed original input")
			}
			if flow == "conversation" {
				contentCommand(t, dir, id, "close", "close-recovered", map[string]any{}, 0)
				waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
			}
		})
	}
}

func offlineContentStop(t *testing.T, dir, id, requestID string) contracts.Request {
	t.Helper()
	reader, err := storage.OpenRecoveryReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := reader.Runs()
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.ID == id {
			raw, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: r.Revision})
			return contracts.Request{Protocol: contracts.Protocol, RequestID: requestID, Input: raw}
		}
	}
	t.Fatal("original offline content Run missing")
	return contracts.Request{}
}

func marshalEvidence(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
