package orca

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

func TestCheckoutIdleInspectsLiveTerminal(t *testing.T) {
	const idleShell = `{"foregroundProcess":"pwsh.exe","hasChildProcesses":false,"childProcessEvidence":"no-children"}`
	cases := []struct {
		name, process, screen, tabIncarnation, diagnostic string
	}{
		{name: "idle shell is allowed", process: idleShell, tabIncarnation: "original"},
		{name: "running child blocks even with shell foreground", process: `{"foregroundProcess":"pwsh.exe","hasChildProcesses":true,"childProcessEvidence":"children"}`, diagnostic: "running Orca terminal command"},
		{name: "non-shell foreground blocks", process: `{"foregroundProcess":"git.exe","hasChildProcesses":false,"childProcessEvidence":"no-children"}`, diagnostic: "running Orca terminal command"},
		{name: "unverified process state blocks", process: `{"foregroundProcess":"pwsh.exe","hasChildProcesses":false,"childProcessEvidence":"unverifiable"}`, diagnostic: "activity could not be verified"},
		{name: "legacy boolean alone blocks", process: `{"foregroundProcess":"pwsh.exe","hasChildProcesses":false}`, diagnostic: "activity could not be verified"},
		{name: "unknown foreground blocks", process: `{"foregroundProcess":null,"hasChildProcesses":false,"childProcessEvidence":"no-children"}`, diagnostic: "activity could not be verified"},
		{name: "running shell command with false no-children blocks", process: idleShell, screen: "fixture command running", diagnostic: "not at an idle shell prompt"},
		{name: "prompt with typed command blocks", process: idleShell, screen: `PS D:\checkout> git status`, diagnostic: "not at an idle shell prompt"},
		{name: "changed tab incarnation blocks", process: idleShell, tabIncarnation: "replacement", diagnostic: "tab terminal identity changed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "terminal.list":
					var params struct {
						Worktree                string
						RequireFreshPtyLiveness bool
					}
					if err := json.Unmarshal(req.Params, &params); err != nil || params.Worktree != "id:checkout" || !params.RequireFreshPtyLiveness {
						t.Errorf("unexpected inventory parameters: %s", req.Params)
					}
					ipcReply(conn, req, "runtime", json.RawMessage(`{
						"terminals":[{"handle":"shell","incarnationId":"original","worktreeId":"checkout"}],
						"totalCount":1,"truncated":false,"hostScope":{"hostIds":["local"],"omittedHostIds":[]}
					}`))
				case "terminal.inspectProcess":
					var params struct {
						Terminal, ExpectedIncarnationID string
						ScanChildProcesses              bool
					}
					if err := json.Unmarshal(req.Params, &params); err != nil || params.Terminal != "shell" || params.ExpectedIncarnationID != "original" || !params.ScanChildProcesses {
						t.Errorf("unexpected process inspection parameters: %s", req.Params)
					}
					ipcReply(conn, req, "runtime", map[string]any{"process": json.RawMessage(test.process)})
				case "terminal.read":
					var params struct {
						Terminal, ExpectedIncarnationID string
						Screen                          bool
						Limit                           int
					}
					if err := json.Unmarshal(req.Params, &params); err != nil || params.Terminal != "shell" || params.ExpectedIncarnationID != "original" || !params.Screen || params.Limit != 1 {
						t.Errorf("unexpected screen parameters: %s", req.Params)
					}
					screen := test.screen
					if screen == "" {
						screen = `PS D:\checkout>`
					}
					ipcReply(conn, req, "runtime", map[string]any{"terminal": map[string]any{
						"handle": "shell", "status": "running", "source": "screen", "tail": []string{screen},
						"tailRows": []any{map[string]any{"wrapped": false, "trailingSpaces": 1}},
					}})
				case "session.tabs.list":
					ipcReply(conn, req, "runtime", map[string]any{
						"worktree": "checkout",
						"tabs":     []any{map[string]any{"terminal": "shell", "incarnationId": test.tabIncarnation}},
					})
				default:
					t.Errorf("unexpected Orca request %q", req.Method)
					ipcReply(conn, req, "runtime", nil)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := (EnvironmentTerminalClient{Runtime: "runtime"}).CheckoutIdle(ctx, "checkout")
			if test.diagnostic == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.diagnostic) {
				t.Fatalf("expected %q, got %v", test.diagnostic, err)
			}
		})
	}
}
