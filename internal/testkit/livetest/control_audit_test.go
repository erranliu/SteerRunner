package livetest

import (
	"encoding/base64"
	"encoding/json"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"testing"
	"time"
)

func TestFixedControlNativeAudit(t *testing.T) {
	for _, mode := range []string{"valid", "lost-send-receipt", "lost-close-inventory", "wrong-helper", "title-only", "late-create", "extra-create-flag", "wrong-workspace", "wrong-ready", "wrong-incarnation", "wrong-session", "wrong-input", "wrong-response", "changed-final", "bad-auth", "missing-close", "unconfirmed-close", "unknown-create", "unresolved-send", "extra-send-flag", "initial-valid", "initial-no-empty", "initial-extra-command"} {
		t.Run(mode, func(t *testing.T) {
			p := NativeReceiptAuditPolicy{Executable: "C:/runtime/helper.exe", SHA256: strings.Repeat("b", 64), ConfigurationDigest: strings.Repeat("a", 64), AccessMode: "sandbox", Environment: "paired"}
			ready := codex.RemoteReadiness{Verified: true, NativeTransport: codex.RemoteNativeTransport, Contract: codex.PolicyContract, Workspace: "C:/worker", Repository: RemoteURL, RuntimeSHA256: p.SHA256, ConfigurationDigest: p.ConfigurationDigest, AccessMode: p.AccessMode, SessionID: "01a1453f-0327-7f21-b0b9-ee79dad66108", Process: b.ProcessIdentity{PID: 7, Created: "native-creation"}, CheckedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour), AuthenticationControl: true}
			setup := codex.RemoteControlSetup{Binding: strings.Repeat("a", 64), Workspace: ready.Workspace, RuntimeSHA256: p.SHA256}
			encoded := func(v any) string { raw, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(raw) }
			wt := "repo::C:/worker"
			calls := []nativeAuditCall{}
			add := func(id string, args []string, result any, failed bool) {
				response, _ := json.Marshal(map[string]any{"ok": !failed, "result": result, "_meta": map[string]any{"runtimeId": "runtime"}})
				calls = append(calls, nativeAuditCall{Kind: "intent", ID: id, Args: args}, nativeAuditCall{Kind: "receipt", ID: id, Args: args, Response: response, Failed: failed})
			}
			term := func(h, inc string) map[string]any {
				return map[string]any{"handle": h, "incarnationId": inc, "worktreeId": wt, "connected": true, "status": "running"}
			}
			inventory := func(ts []any) any {
				return map[string]any{"terminals": ts, "totalCount": len(ts), "truncated": false, "hostScope": map[string]any{"hostIds": []string{"host"}, "omittedHostIds": []string{}}}
			}
			add("workspace", []string{"worktree", "create"}, map[string]any{"worktree": map[string]any{"id": wt, "path": ready.Workspace, "displayName": "test"}}, false)
			if strings.HasPrefix(mode, "initial-") {
				if mode != "initial-no-empty" {
					add("empty", []string{"terminal", "list", "--worktree", "id:" + wt}, inventory([]any{}), false)
				}
				args := []string{"terminal", "create", "--worktree", "id:" + wt, "--title", "SteerRunner initial execution test", "--json", "--environment", p.Environment}
				if mode == "initial-extra-command" {
					args = append(args, "--command", "codex")
				}
				add("initial", args, map[string]any{"terminal": term("business", "business-inc")}, false)
				add("initial-show", []string{"terminal", "show", "--terminal", "business"}, map[string]any{"terminal": term("business", "business-inc")}, false)
				add("initial-list", []string{"terminal", "list", "--worktree", "id:" + wt}, inventory([]any{term("business", "business-inc")}), false)
			}
			if mode == "wrong-workspace" {
				setup.Workspace = "C:/foreign"
			}
			command := "& '" + p.Executable + "' '--native-control' '" + encoded(setup) + "'"
			if mode == "wrong-helper" {
				command = strings.ReplaceAll(command, p.Executable, "C:/foreign/helper.exe")
			}
			if mode == "title-only" {
				command = "codex"
			}
			create := []string{"terminal", "create", "--worktree", "id:" + wt, "--title", "SteerRunner native control", "--command", command, "--json", "--environment", p.Environment}
			if mode == "extra-create-flag" {
				create = append(create, "--agent", "codex")
			}
			if mode == "late-create" {
				add("early-start", []string{"terminal", "send", "--terminal", "business", "--text", "startup"}, map[string]any{"send": map[string]any{"handle": "business", "accepted": true}}, false)
			}
			add("control", create, map[string]any{"terminal": term("control", "control-inc")}, mode == "unknown-create")
			inc := "control-inc"
			if mode == "wrong-incarnation" {
				inc = "other"
			}
			show := func(id string) {
				add(id, []string{"terminal", "show", "--terminal", "control"}, map[string]any{"terminal": term("control", inc)}, false)
			}
			show("control-show")
			readSetup := setup
			if mode == "wrong-ready" {
				readSetup.RuntimeSHA256 = "other"
			}
			add("control-ready", []string{"terminal", "read", "--terminal", "control"}, map[string]any{"terminal": map[string]any{"handle": "control", "tail": []string{codex.RemoteControlReadyPrefix + encoded(readSetup)}}}, false)
			add("startup", []string{"terminal", "send", "--terminal", "business", "--text", "startup"}, map[string]any{"send": map[string]any{"handle": "business", "accepted": true}}, false)
			add("ready", []string{"terminal", "read", "--terminal", "business"}, map[string]any{"terminal": map[string]any{"handle": "business", "tail": []string{codex.RemoteReadyPrefix + encoded(ready)}}}, false)
			wire := "[SteerRunner command=" + strings.Repeat("c", 32) + " input=" + strings.Repeat("d", 64) + "]\nreport"
			add("business", []string{"terminal", "send", "--terminal", "business", "--text", wire}, map[string]any{"send": map[string]any{"handle": "business", "accepted": true}}, false)
			query := codex.RemoteControlRequest{ID: strings.Repeat("e", 32), Binding: setup.Binding, Method: "original-turn", Session: ready.SessionID, InputDigest: receiptHash(wire)}
			if mode == "wrong-session" {
				query.Session = "01a1453f-0327-7f21-b0b9-ee79dad66109"
			}
			if mode == "wrong-input" {
				query.InputDigest = strings.Repeat("f", 64)
			}
			for _, method := range []string{"original-turn", "authentication-stop"} {
				if method == "authentication-stop" {
					query.ID = strings.Repeat("f", 32)
					query.Method = method
					query.InputDigest = ""
				}
				args := []string{"terminal", "send", "--terminal", "control", "--text", encoded(query), "--enter", "--retry-request", controlRetryID(query.ID), "--json", "--environment", p.Environment}
				if mode == "extra-send-flag" {
					args = append(args, "--interrupt")
				}
				add("send-"+method, args, map[string]any{"send": map[string]any{"handle": "control", "accepted": true}}, false)
				if mode == "lost-send-receipt" && method == "original-turn" {
					calls = calls[:len(calls)-1]
				}
				response := codex.RemoteControlResponse{ID: query.ID, Binding: setup.Binding, RequestDigest: codex.RemoteControlDigest(query)}
				if method == "original-turn" {
					response.Turn = &codex.RemoteTurnReceipt{Source: codex.RemoteTurnSource, Readiness: ready, InputDigest: query.InputDigest, Started: true, Completed: true, Status: "completed", NativeTurnID: "01a14502-5798-78f1-aaa3-2d7cd15594c8", UserItemID: "user", FinalItemID: "final", FinalText: "complete final", FinalDigest: receiptHash("complete final")}
					if mode == "changed-final" {
						response.Turn.FinalDigest = "other"
					}
				} else {
					response.Authentication = &codex.RemoteAuthenticationCleanup{Readiness: ready, Revoked: mode != "bad-auth", BrokerClosed: true, SocketCleaned: true, Mode: "host-gh-repository-broker"}
				}
				if mode == "wrong-response" {
					response.RequestDigest = "other"
				}
				show("show-" + method)
				if mode != "unresolved-send" || method != "original-turn" {
					add("read-"+method, []string{"terminal", "read", "--terminal", "control"}, map[string]any{"terminal": map[string]any{"handle": "control", "tail": []string{codex.RemoteControlPrefix + encoded(response)}}}, false)
				}
			}
			show("close-show")
			if mode != "missing-close" {
				add("close", []string{"terminal", "close", "--terminal", "control"}, map[string]any{"close": map[string]any{"handle": "control", "ptyKilled": mode != "unconfirmed-close"}}, mode == "lost-close-inventory")
			}
			if mode == "lost-close-inventory" {
				add("absent", []string{"terminal", "list", "--worktree", "id:" + wt}, inventory([]any{}), false)
			}
			raw := []byte{}
			for _, c := range calls {
				line, _ := json.Marshal(c)
				raw = append(raw, line...)
				raw = append(raw, '\n')
			}
			facts, err := AnalyzeNativeAudit(raw, "runtime", p)
			passed := err == nil && facts.Complete && facts.Creations == 1 && facts.Inspections == 1 && facts.ClosedInspections == 1 && len(facts.Prompts) == 2
			want := mode == "valid" || mode == "lost-send-receipt" || mode == "lost-close-inventory" || mode == "initial-valid"
			if passed != want {
				t.Fatalf("mode=%s facts=%+v err=%v want=%t", mode, facts, err, want)
			}
		})
	}
}
