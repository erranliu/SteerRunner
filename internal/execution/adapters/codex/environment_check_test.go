package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentCheckCompletionNotification(t *testing.T) {
	// Native turn/completed is a notification and therefore has no RPC ID.
	call, err := decodeEnvironmentCompletion(json.RawMessage(`{"threadId":"original-thread","turn":{"id":"original-turn","status":"completed"}}`), "original-thread", "original-turn")
	if err != nil || !call.Completed || len(call.RequestID) != 0 {
		t.Fatal("valid native notification was rejected", call, err)
	}
	for _, raw := range []string{`{"threadId":"other","turn":{"id":"original-turn","status":"completed"}}`, `{"threadId":"original-thread","turn":{"id":"other","status":"completed"}}`, `{"threadId":"original-thread","turn":{"id":"original-turn","status":"inProgress"}}`, `{"threadId":"original-thread","turn":{"id":"original-turn","status":"failed"}}`} {
		if _, err := decodeEnvironmentCompletion(json.RawMessage(raw), "original-thread", "original-turn"); err == nil {
			t.Fatal("foreign or unsettled turn passed", raw)
		}
	}
}

func TestEnvironmentCheckWriteGate(t *testing.T) {
	g := &checkWriteGate{base: "steerline-check/check/base", head: "steerline-check/check/head"}
	zero, sha := strings.Repeat("0", 40), strings.Repeat("a", 40)
	ref := "refs/heads/" + g.base
	packet := func(ref, old, new string) string {
		line := old + " " + new + " " + ref + "\x00 report-status\n"
		return fmt.Sprintf("%04x%s0000", len(line)+4, line)
	}
	request := func(body string) bool {
		return g.permit(httptest.NewRequest("POST", "https://github.com/owner/repo.git/git-receive-pack", strings.NewReader(body)), "owner/repo")
	}
	if request(packet(ref, zero, sha)) {
		t.Fatal("unarmed write passed")
	}
	g.arm("base_push", ref, zero, sha, g.base, g.head, 0)
	if request(packet("refs/heads/main", zero, sha)) {
		t.Fatal("business branch passed")
	}
	if request(packet(ref, sha, sha)) {
		t.Fatal("existing branch overwrite passed")
	}
	if !request(packet(ref, zero, sha)) {
		t.Fatal("authorized creation denied")
	}
	if request(packet(ref, zero, sha)) {
		t.Fatal("duplicate or late write passed")
	}
	g.arm("delete", ref, sha, zero, g.base, g.head, 0)
	if request(packet(ref, strings.Repeat("b", 40), zero)) {
		t.Fatal("changed SHA deleted")
	}
	if !request(packet(ref, sha, zero)) {
		t.Fatal("owned deletion rejected")
	}
	g.arm("pr_create", "", "", "", g.base, g.head, 0)
	api := func(method, path, body string) bool {
		return g.permit(httptest.NewRequest(method, "https://api.github.com/repos/owner/repo/"+path, strings.NewReader(body)), "owner/repo")
	}
	if api("POST", "pulls", `{"base":"main","head":"`+g.head+`"}`) {
		t.Fatal("business PR passed")
	}
	if !api("POST", "pulls", `{"base":"`+g.base+`","head":"`+g.head+`","maintainer_can_modify":false}`) {
		t.Fatal("check PR denied")
	}
	if api("POST", "pulls", `{"base":"`+g.base+`","head":"`+g.head+`"}`) {
		t.Fatal("duplicate PR passed")
	}
	g.arm("pr_close", "", "", "", g.base, g.head, 17)
	if api("PATCH", "pulls/18", `{"state":"closed"}`) || api("PUT", "pulls/17/merge", `{}`) {
		t.Fatal("foreign PR or merge passed")
	}
	if !api("PATCH", "pulls/17", `{"state":"closed"}`) {
		t.Fatal("original close denied")
	}
}

func TestEnvironmentCheckPermissionModes(t *testing.T) {
	for _, mode := range []string{"sandbox", "full_access"} {
		p, err := NativePermissions("windows", "local", "codex", mode)
		if err != nil {
			t.Fatal(err)
		}
		if p.ApprovalPolicy != "never" || p.ApprovalsReviewer != "user" {
			t.Fatal(p)
		}
		thread, err := ThreadStartParams("C:/work", p.Sandbox, "", nil)
		if err != nil || thread["approvalPolicy"] != "never" {
			t.Fatal(thread, err)
		}
		turn, err := TurnStartParams("C:/work", "thread", "command", "input", p.Sandbox, "", "", []string{"C:/git"})
		if err != nil || turn["approvalPolicy"] != "never" {
			t.Fatal(turn, err)
		}
		if err = validateNativeLaunchArgs(p.Args()); err != nil {
			t.Fatal(err)
		}
		if mode == "full_access" && turn["sandboxPolicy"].(map[string]any)["type"] != "dangerFullAccess" {
			t.Fatal(turn)
		}
	}
	if _, err := NativePermissions("windows", "local", "codex", "unknown"); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func TestEnvironmentExpiredPreflightRejectsBeforeNativeTurn(t *testing.T) {
	auth := testAuthentication(t)
	auth.expires = time.Now().Add(-time.Minute)
	dir := t.TempDir()
	// This client has no connection. No native RPC can be reached by preflight.
	s := &EnvironmentSession{setup: EnvironmentSetup{Workspace: dir}, input: b.TurnInput{LinkedWorktreeArgs: []string{"--add-dir", dir + "/1", "--add-dir", dir + "/2", "--add-dir", dir + "/3", "--add-dir", dir + "/4"}}, backend: Backend{Config: Config{Authentication: auth, Workspace: dir}}, handle: &SessionHandle{client: new(codexapp.Client)}}
	turn, err := s.Start(context.Background())
	var rejected *EnvironmentInputRejectedError
	if turn != "" || !errors.As(err, &rejected) || rejected.Reason != "authentication_expired_or_unavailable" {
		t.Fatal("preflight was reported as uncertain native input", turn, err)
	}
}
