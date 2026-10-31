package codexworker

import (
	"os"
	"reflect"
	"testing"
)

func TestAppServerLaunchPreservesConfiguredArgs(t *testing.T) {
	configured := []string{"--config", "example=true"}
	launch, err := AppServerLaunch(os.Args[0], configured, "fixed", "ws://127.0.0.1:1234")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--config", "example=true", "--profile", "fixed", "app-server", "--listen", "ws://127.0.0.1:1234"}
	if !reflect.DeepEqual(launch.Args, want) {
		t.Fatalf("app-server launch changed fixed arguments: %v", launch.Args)
	}
	configured[0] = "changed"
	if launch.Args[0] != "--config" {
		t.Fatal("app-server launch shares mutable command arguments")
	}
}

func TestTurnStartParamsKeepApprovalAndWritableRoots(t *testing.T) {
	roots, err := WritableRoots([]string{"--add-dir", "C:/worktree", "--add-dir", "C:/shared"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WritableRoots([]string{"--add-dir"}); err == nil {
		t.Fatal("incomplete Codex writable root was accepted")
	}
	params, err := TurnStartParams("C:/worktree", "thread", "command", "continue", "danger-full-access", "model", "high", roots)
	if err != nil {
		t.Fatal(err)
	}
	policy, ok := params["sandboxPolicy"].(map[string]any)
	if !ok || policy["type"] != "dangerFullAccess" || params["approvalPolicy"] != "never" || params["approvalsReviewer"] != "user" || params["effort"] != "high" {
		t.Fatalf("native turn policy changed: %+v", params)
	}
	if policy["writableRoots"] != nil || policy["networkAccess"] != nil {
		t.Fatalf("full_access contains workspace grants: %+v", policy)
	}
	if _, err := TurnStartParams("C:/worktree", "thread", "command", "continue", "unsupported", "", "", nil); err == nil {
		t.Fatal("unsupported Codex sandbox was accepted")
	}
}
