//go:build windows

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
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
			if restored.Execution.SessionID != original.Execution.SessionID || restored.Execution.TurnID != original.Execution.TurnID || restored.ResumeRequired || contentProcess(t, dir, id) != owner || !bytes.Equal(historyRecoveryEvidence(t, history.Data), historyRecoveryEvidence(t, call(t, dir, []string{"history", "--run-id", id}, "", 0).Data)) {
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
	// Use the public cold reader; the caller never opens Runner private storage.
	v := runView(t, call(t, dir, []string{"inspect", "--offline", "--run-id", id}, "", 0))
	if v.RunID != id || v.Revision == 0 {
		t.Fatal("original cold Run binding missing", v)
	}
	raw, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: v.Revision})
	return contracts.Request{Protocol: contracts.Protocol, RequestID: requestID, Input: raw}
}

func marshalEvidence(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func historyRecoveryEvidence(t *testing.T, value any) []byte {
	t.Helper()
	var page map[string]any
	if err := json.Unmarshal(marshalEvidence(t, value), &page); err != nil {
		t.Fatal(err)
	}
	run, ok := page["run"].(map[string]any)
	if !ok {
		t.Fatal("history response is missing its Run")
	}
	// History includes a live Run projection. Reattachment can advance these
	// metadata fields without changing history, identity or authorization.
	delete(run, "revision")
	delete(run, "updated_at")
	return marshalEvidence(t, page)
}

// Public EXE loop: no caller reads Runner private state or starts workers before
// the original stop is durably accepted. The native peer is controlled.
func TestSRSingleExecutionColdStop(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	native := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	t.Setenv("STEERRUNNER_CONTENT_FIXTURE", "1")
	for name, text := range map[string]string{"hold-turn": "hold", "content-final": `{"completed":true}`} {
		if err := os.WriteFile(filepath.Join(native, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var input contracts.RunRequest
	if err := json.Unmarshal(request.Input, &input); err != nil {
		t.Fatal(err)
	}
	input.Workflow = contracts.Workflow{ID: "execute", Version: 1, PromptOverrides: map[string]string{"execute": "Invoke project detection once"}}
	input.Publication = contracts.Publication{}
	input.ProgressFile = nil
	input.ResultFile = &contracts.ResultFileInput{InitialContentBase64: "e30="}
	request.Input, _ = json.Marshal(input)
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	original := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.TurnID != "" })
	activeReuse := request
	activeInput := input
	activeInput.ReuseWorktreeFromRunID = id
	activeReuse.RequestID = "reuse-active-source"
	activeReuse.Input, _ = json.Marshal(activeInput)
	if denied := call(t, dir, []string{"submit", "--input", "-"}, string(marshalEvidence(t, activeReuse)), 5); denied.Error.Code != "workspace_not_settled" {
		t.Fatal("active writer was reusable", denied)
	}
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("single-detach", service.InstanceID), 0)
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	// Query by the original submit identity, as needed after an unknown receipt.
	found := runView(t, call(t, dir, []string{"inspect", "--offline", "--request-id", request.RequestID}, "", 0))
	if found.RunID != id || found.InputDigest != original.InputDigest || found.Revision < original.Revision {
		t.Fatal(found)
	}
	stop := offlineContentStop(t, dir, id, "single-original-stop")
	start := contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: id, Request: stop}}}
	next := serviceInfo(t, call(t, dir, []string{"service", "start", "--input", "-"}, string(start.JSON()), 0))
	cleanupService(t, dir, next)
	settled := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	if !settled.Closed || countNative(t, "turn/start") != 1 || settled.InputDigest != original.InputDigest {
		t.Fatal("replayed input or unclosed execution", settled)
	}
	receipt := call(t, dir, []string{"inspect", "--request-id", stop.RequestID}, "", 0)
	var view struct {
		Command contracts.CommandView `json:"command"`
	}
	if json.Unmarshal(marshalEvidence(t, receipt.Data), &view) != nil || view.Command.ActionStatus != "completed" {
		t.Fatal("original stop did not settle", receipt)
	}
	result := call(t, dir, []string{"result-file", "--run-id", id}, "", 0)
	var file contracts.ResultFileView
	if json.Unmarshal(marshalEvidence(t, result.Data), &file) != nil || file.CollectionStatus != "closed" {
		t.Fatal("final result was not sealed", result)
	}
	oldResult := marshalEvidence(t, result.Data)
	oldView := marshalEvidence(t, settled)
	root, branch := settled.Execution.Workspace, settled.Execution.Branch
	if settled.WorkspaceIdentity == "" {
		t.Fatal("creation did not retain workspace identity")
	}
	git(t, root, "config", "user.name", "Reuse Fixture")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	for round := 1; round <= 2; round++ {
		if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte(fmt.Sprintf("round %d", round)), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "spec.md")
		git(t, root, "commit", "-m", fmt.Sprintf("retained round %d", round))
		if err := os.WriteFile(filepath.Join(root, "leftover"), []byte("untracked must survive"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte("unfinished must survive"), 0600); err != nil {
			t.Fatal(err)
		}
		head, dirty := git(t, root, "rev-parse", "HEAD"), git(t, root, "status", "--porcelain")
		input.ReuseWorktreeFromRunID = settled.RunID
		input.Workflow.PromptOverrides["execute"] = fmt.Sprintf("Finish retained work round %d", round)
		request.RequestID = fmt.Sprintf("reuse-%d", round)
		request.Input, _ = json.Marshal(input)
		raw, _ = json.Marshal(request)
		var responses [2]contracts.Response
		var exits [2]int
		var failures [2]error
		var submissions [2]contracts.Request
		var racing sync.WaitGroup
		for index := range submissions {
			submissions[index] = request
			submissions[index].RequestID += fmt.Sprintf("-racer-%d", index)
			racing.Add(1)
			go func(index int) {
				defer racing.Done()
				responses[index], exits[index], _, failures[index] = invoke(runnerEXE, []string{"submit", "--state-dir", dir, "--input", "-", "--json"}, string(marshalEvidence(t, submissions[index])))
			}(index)
		}
		racing.Wait()
		winner := -1
		for index := range responses {
			if failures[index] != nil {
				t.Fatal(failures[index])
			}
			if exits[index] == 0 {
				if winner >= 0 {
					t.Fatal("two workspace writers accepted")
				}
				winner = index
			} else if exits[index] != 5 {
				t.Fatal("unexpected reuse failure", responses[index], exits[index])
			}
		}
		if winner < 0 {
			t.Fatal("no reuse request accepted", responses)
		}
		request = submissions[winner]
		raw = marshalEvidence(t, request)
		reused := responses[winner]
		nextID := reused.Data.(map[string]any)["run_id"].(string)
		replay := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
		if replay.Data.(map[string]any)["run_id"] != nextID {
			t.Fatal("duplicate request transferred twice")
		}
		active := waitRun(t, dir, nextID, func(v contracts.RunView) bool { return v.Execution.TurnID != "" })
		if active.Execution.Workspace != root || active.Execution.Branch != branch || active.ReuseWorktreeFromRunID != settled.RunID || active.Execution.SessionID == settled.Execution.SessionID || active.WorkspaceIdentity != settled.WorkspaceIdentity || git(t, root, "rev-parse", "HEAD") != head || git(t, root, "status", "--porcelain") != dirty {
			t.Fatal("reuse lost state or reused a session", active)
		}
		// A different request cannot fork the same ancestor while its successor writes.
		conflict := request
		conflict.RequestID += "-conflict"
		conflictRaw, _ := json.Marshal(conflict)
		call(t, dir, []string{"submit", "--input", "-"}, string(conflictRaw), 5)
		oldStop := contracts.Request{Protocol: contracts.Protocol, RequestID: fmt.Sprintf("old-stop-%d", round), Input: marshalEvidence(t, contracts.RevisionInput{ExpectedRevision: settled.Revision})}
		call(t, dir, []string{"stop", "--run-id", settled.RunID, "--input", "-"}, string(marshalEvidence(t, oldStop)), 5)
		if round == 1 {
			call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("reuse-active-detach", next.InstanceID), 0)
			if err := waitOwnedService(next); err != nil {
				t.Fatal(err)
			}
			next = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, next)
			active = waitRun(t, dir, nextID, func(v contracts.RunView) bool { return v.Execution.TurnID != "" })
		}
		stopNext := contracts.Request{Protocol: contracts.Protocol, RequestID: fmt.Sprintf("reuse-stop-%d", round), Input: marshalEvidence(t, contracts.RevisionInput{ExpectedRevision: active.Revision})}
		call(t, dir, []string{"stop", "--run-id", nextID, "--input", "-"}, string(marshalEvidence(t, stopNext)), 0)
		settled = waitRun(t, dir, nextID, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
		waitResultSealed(t, dir, nextID)
	}
	if err := os.Remove(filepath.Join(native, "hold-turn")); err != nil {
		t.Fatal(err)
	}
	input.ReuseWorktreeFromRunID = settled.RunID
	input.Workflow.PromptOverrides["execute"] = "CONTROLLED_REUSE_FINISH: commit the retained modifications"
	request.RequestID = "reuse-closeout"
	request.Input, _ = json.Marshal(input)
	closedID := call(t, dir, []string{"submit", "--input", "-"}, string(marshalEvidence(t, request)), 0).Data.(map[string]any)["run_id"].(string)
	closed := waitRun(t, dir, closedID, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	if !closed.Closed || closed.Execution.Workspace != root || closed.Execution.Branch != branch || git(t, root, "status", "--porcelain") != "" || git(t, root, "log", "-1", "--format=%s") != "finish retained modifications" {
		t.Fatal("new execute could not finish retained work", closed)
	}
	if countNative(t, "turn/start") != 4 || countNative(t, "thread/start") != 4 {
		t.Fatal("reuse resent original inputs or sessions")
	}
	if got := marshalEvidence(t, call(t, dir, []string{"result-file", "--run-id", id}, "", 0).Data); !bytes.Equal(got, oldResult) {
		t.Fatal("source result changed after reuse")
	}
	if got := marshalEvidence(t, runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))); !bytes.Equal(got, oldView) {
		t.Fatal("source Run changed after transfer")
	}
}

func waitResultSealed(t *testing.T, dir, id string) {
	t.Helper()
	waitRun(t, dir, id, func(v contracts.RunView) bool {
		var file contracts.ResultFileView
		response := call(t, dir, []string{"result-file", "--run-id", id}, "", 0)
		return json.Unmarshal(marshalEvidence(t, response.Data), &file) == nil && file.CollectionStatus == "closed"
	})
}
