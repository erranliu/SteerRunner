//go:build windows

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRManagedConversationWorkspaceWrite(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	t.Setenv("STEERRUNNER_CONTENT_FIXTURE", "1")
	t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
	t.Setenv("STEERRUNNER_WORKSPACE_WRITE_FIXTURE", "1")
	native := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	write := func(dir, name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(native, "hold-turn", "hold")
	write(native, "content-final", "original native final")
	var input contracts.RunRequest
	if err := json.Unmarshal(request.Input, &input); err != nil {
		t.Fatal(err)
	}
	input.Workflow = contracts.Workflow{ID: "conversation", Version: 2}
	input.Conversation = &contracts.ConversationInput{Instruction: "Write in the original worktree", Materials: []contracts.Document{}}
	input.Documents, input.Publication = contracts.Documents{}, contracts.Publication{}
	request.Input = marshalEvidence(t, input)
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(marshalEvidence(t, request)), 0).Data.(map[string]any)["run_id"].(string)
	inspect := func() contracts.RunView {
		return runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	}
	wire := func(requestID string, body any) string {
		return string(marshalEvidence(t, contracts.Request{Protocol: contracts.Protocol, RequestID: requestID, Input: marshalEvidence(t, body)}))
	}
	args := func(command string) []string {
		return append(strings.Fields(command), "--run-id", id, "--input", "-")
	}
	active := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Controls.CanInterrupt })
	if got := call(t, dir, args(contracts.WorkspaceAcquire), wire("active", contracts.RevisionInput{ExpectedRevision: active.Revision}), 5); got.Error.Code != "workspace_not_idle" {
		t.Fatal(got)
	}
	if err := os.Remove(filepath.Join(native, "hold-turn")); err != nil {
		t.Fatal(err)
	}
	idle := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.WorkspaceWrite != nil && v.WorkspaceWrite.CanAcquire })
	owner := contentProcess(t, dir, id)
	worktree := idle.Execution.Workspace
	if idle.WorkspaceOriginRunID != id || idle.WorkspaceIdentity == "" || idle.WorkspaceWrite.Owner != "runner" ||
		idle.ContentResult == nil || idle.ContentResult.Status != "succeeded" {
		t.Fatal("managed worktree binding missing", idle)
	}
	firstWrite, err := os.ReadFile(filepath.Join(worktree, "native-write-1"))
	if err != nil || string(firstWrite) != idle.Execution.SessionID {
		t.Fatal("first native writer did not use the original worktree", err)
	}

	// This fixture owns SteerLine's side of the lease and returns it even if a
	// later assertion fails, before normal original-session cleanup runs.
	defer func() {
		query, _, _, err := invoke(runnerEXE, []string{"inspect", "--run-id", id, "--state-dir", dir, "--json"}, "")
		offline := err != nil || !query.OK
		if offline {
			query, _, _, err = invoke(runnerEXE, []string{"inspect", "--offline", "--run-id", id, "--state-dir", dir, "--json"}, "")
		}
		if err != nil || !query.OK {
			return
		}
		v := runView(t, query)
		if v.WorkspaceWrite != nil && v.WorkspaceWrite.LeaseID != "" {
			if offline {
				restored := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				cleanupService(t, dir, restored)
			}
			call(t, dir, args(contracts.WorkspaceRelease), wire("cleanup-lease", contracts.WorkspaceReleaseInput{ExpectedRevision: v.Revision, LeaseID: v.WorkspaceWrite.LeaseID}), 0)
		}
	}()

	var outcomes [2]contracts.Response
	var exits [2]int
	var failures [2]error
	inputs := []string{
		wire("acquire-a", contracts.RevisionInput{ExpectedRevision: idle.Revision}),
		wire("acquire-b", contracts.RevisionInput{ExpectedRevision: idle.Revision}),
	}
	var race sync.WaitGroup
	for i, raw := range inputs {
		race.Add(1)
		go func() {
			defer race.Done()
			outcomes[i], exits[i], _, failures[i] = invoke(runnerEXE, append(args(contracts.WorkspaceAcquire), "--state-dir", dir, "--json"), raw)
		}()
	}
	race.Wait()
	winner := -1
	for i, result := range outcomes {
		if failures[i] == nil && exits[i] == 0 && result.OK && winner == -1 {
			winner = i
		} else if failures[i] != nil || exits[i] != 5 || result.Error == nil || result.Error.Code != "workspace_write_held" {
			t.Fatal("unexpected acquire race result", outcomes, exits, failures)
		}
	}
	if winner < 0 {
		t.Fatal("no write lease acquired", outcomes)
	}
	receipt := outcomes[winner].Data.(map[string]any)
	lease := receipt["command_id"].(string)
	held := inspect()
	if held.WorkspaceWrite.Owner != "steerline" || held.WorkspaceWrite.LeaseID != lease || held.NewInputsAllowed || receipt["action_status"] != "completed" {
		t.Fatal("lease was not committed with its receipt", held, receipt)
	}
	sendWhileHeld := func(requestID string, view contracts.RunView) {
		t.Helper()
		got := call(t, dir, args("message"), wire(requestID, contracts.MessageInput{ExpectedRevision: view.Revision, Text: "must not dispatch"}), 5)
		if got.Error.Code != "workspace_write_held" || countNative(t, "turn/start") != 1 {
			t.Fatal("Agent input escaped the write gate", got)
		}
	}
	sendWhileHeld("blocked-message-before-restart", held)
	if got := call(t, dir, args(contracts.WorkspaceRelease), wire("wrong-lease", contracts.WorkspaceReleaseInput{ExpectedRevision: held.Revision, LeaseID: "other-holder"}), 5); got.Error.Code != "workspace_write_lease_mismatch" {
		t.Fatal(got)
	}

	// The caller prepares diverging commits and leaves rebase stopped. Runner
	// receives neither a Git command nor a conflict-resolution instruction.
	source := input.Repository.LocalPath
	write(source, "spec.md", "upstream edit\n")
	git(t, source, "add", "spec.md")
	git(t, source, "-c", "user.name=SteerLine", "-c", "user.email=fixture@example.invalid", "commit", "-m", "upstream")
	git(t, source, "push", "origin", "main")
	write(worktree, "spec.md", "worktree edit\n")
	git(t, worktree, "add", "spec.md")
	git(t, worktree, "-c", "user.name=SteerLine", "-c", "user.email=fixture@example.invalid", "commit", "-m", "worktree")
	git(t, worktree, "fetch", "origin", "main")
	rebase := exec.Command("git", "-C", worktree, "rebase", "FETCH_HEAD")
	if output, err := rebase.CombinedOutput(); err == nil || !strings.Contains(string(output), "CONFLICT") {
		t.Fatalf("expected a real rebase conflict: %v %s", err, output)
	}
	conflictIndex := git(t, worktree, "ls-files", "--unmerged")
	if conflictIndex == "" {
		t.Fatal("rebase did not leave unmerged entries")
	}

	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("handoff-detach", service.InstanceID), 0)
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	cold := runView(t, call(t, dir, []string{"inspect", "--offline", "--command-id", lease}, "", 0))
	if cold.WorkspaceWrite == nil || *cold.WorkspaceWrite != *held.WorkspaceWrite || cold.Execution.SessionID != idle.Execution.SessionID {
		t.Fatal("offline inspection lost the original lease or session", cold)
	}
	next := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, next)
	if next.InstanceID == service.InstanceID {
		t.Fatal("expected replacement service")
	}
	restored := inspect()
	sendWhileHeld("blocked-message-after-restart", restored)
	if restored.WorkspaceWrite == nil || *restored.WorkspaceWrite != *held.WorkspaceWrite || contentProcess(t, dir, id) != owner {
		t.Fatal("service restart changed the original lease or native process", restored)
	}
	replayed := call(t, dir, args(contracts.WorkspaceAcquire), inputs[winner], 0)
	if replayed.Data.(map[string]any)["command_id"] != lease {
		t.Fatal("replay did not return the original lease receipt", replayed)
	}
	release := wire("release", contracts.WorkspaceReleaseInput{ExpectedRevision: restored.Revision, LeaseID: lease})
	call(t, dir, args(contracts.WorkspaceRelease), release, 0)
	call(t, dir, args(contracts.WorkspaceRelease), release, 0)
	returned := inspect()
	if returned.WorkspaceWrite.Owner != "runner" || returned.WorkspaceWrite.LeaseID != "" || !returned.Controls.CanMessage {
		t.Fatal("original session did not regain write authorization", returned)
	}
	contentCommand(t, dir, id, "message", "original-follow-up", map[string]any{"text": "Read the conflict and write a marker"}, 0)
	continued := waitRun(t, dir, id, func(v contracts.RunView) bool {
		return v.Controls.CanMessage && v.Execution.TurnID != idle.Execution.TurnID
	})
	secondWrite, err := os.ReadFile(filepath.Join(worktree, "native-write-2"))
	if err != nil || string(secondWrite) != idle.Execution.SessionID ||
		continued.RunID != id || continued.Execution.SessionID != idle.Execution.SessionID ||
		continued.Execution.AttemptID != idle.Execution.AttemptID || continued.Execution.Workspace != worktree ||
		continued.WorkspaceIdentity != idle.WorkspaceIdentity || countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 2 ||
		git(t, worktree, "ls-files", "--unmerged") != conflictIndex {
		t.Fatal("follow-up changed original binding, input count or conflict state", continued, err)
	}
	contentCommand(t, dir, id, "close", "close-original", map[string]any{}, 0)
	waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
}
