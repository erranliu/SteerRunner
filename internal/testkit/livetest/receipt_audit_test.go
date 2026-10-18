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

func TestReadonlyReceiptAuditKeepsBusinessIdentityAndClosureStrict(t *testing.T) {
	modes := []string{"valid", "canceled-read-closed", "pending-native-input", "title-only", "extra-agent-flag", "other-helper", "other-query-session", "other-query-workspace", "other-query-input", "other-worktree", "missing-ready", "changed-ready-hash", "no-close", "unknown-create", "close-not-confirmed", "changed-incarnation", "other-read-runtime", "other-read-handle", "other-receipt-session", "other-receipt-input", "changed-final", "auxiliary-input", "missing-close-intent", "duplicate-close-receipt", "inventory-absent", "inventory-truncated", "inventory-other-runtime", "inventory-other-worktree", "inventory-other-incarnation", "inventory-unknown-create"}
	modes = append(modes, "initial-valid", "initial-missing-empty-proof", "initial-truncated", "initial-extra-command", "initial-wrong-title", "initial-unknown-create", "initial-changed-incarnation", "initial-duplicate")
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			p := NativeReceiptAuditPolicy{Executable: "C:/runtime/helper.exe", SHA256: strings.Repeat("b", 64), ConfigurationDigest: strings.Repeat("a", 64), AccessMode: "sandbox", Environment: "paired"}
			ready := codex.RemoteReadiness{Verified: true, NativeTransport: codex.RemoteNativeTransport, Contract: codex.PolicyContract, Workspace: "C:/worker", Repository: RemoteURL, RuntimeSHA256: p.SHA256, ConfigurationDigest: p.ConfigurationDigest, AccessMode: p.AccessMode, SessionID: "01a1453f-0327-7f21-b0b9-ee79dad66108", Process: b.ProcessIdentity{PID: 7, Created: "native-creation"}, CheckedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
			command := strings.Repeat("c", 32)
			wire := "[SteerRunner command=" + command + " input=" + strings.Repeat("d", 64) + "]\nreport"
			q := codex.RemoteTurnQuery{Workspace: ready.Workspace, Session: ready.SessionID, InputDigest: receiptHash(wire)}
			switch mode {
			case "other-query-session":
				q.Session = "01a1453f-0327-7f21-b0b9-ee79dad66109"
			case "other-query-workspace":
				q.Workspace = "C:/other"
			case "other-query-input":
				q.InputDigest = strings.Repeat("e", 64)
			}
			encoded := func(v any) string { raw, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(raw) }
			commandLine := "& '" + p.Executable + "' '--original-turn' '" + encoded(q) + "'"
			if mode == "title-only" {
				commandLine = "codex --dangerously-bypass-approvals-and-sandbox"
			}
			if mode == "other-helper" {
				commandLine = strings.ReplaceAll(commandLine, p.Executable, "C:/other/helper.exe")
			}
			wt := "repo::C:/worker"
			createWT := wt
			if mode == "other-worktree" {
				createWT = "repo::C:/other"
			}
			args := []string{"terminal", "create", "--worktree", "id:" + createWT, "--title", "SteerRunner original native receipt " + command, "--command", commandLine, "--json", "--environment", p.Environment}
			if mode == "extra-agent-flag" {
				args = append(args, "--agent", "codex")
			}
			calls := []nativeAuditCall{}
			add := func(id string, args []string, result any, runtime string, failed bool) {
				response, _ := json.Marshal(map[string]any{"ok": !failed, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
				calls = append(calls, nativeAuditCall{Kind: "intent", ID: id, Args: args}, nativeAuditCall{Kind: "receipt", ID: id, Args: args, Response: response, Failed: failed})
			}
			add("workspace", []string{"worktree", "create"}, map[string]any{"worktree": map[string]any{"id": wt, "path": ready.Workspace, "displayName": "sl-test"}}, "runtime", false)
			if strings.HasPrefix(mode, "initial-") {
				inventory := func(terminals []any, truncated bool) any {
					return map[string]any{"terminals": terminals, "totalCount": len(terminals), "truncated": truncated, "hostScope": map[string]any{"hostIds": []string{"host"}, "omittedHostIds": []string{}}}
				}
				if mode != "initial-missing-empty-proof" {
					add("initial-empty", []string{"terminal", "list", "--worktree", "id:" + wt}, inventory([]any{}, mode == "initial-truncated"), "runtime", false)
				}
				initialArgs := []string{"terminal", "create", "--worktree", "id:" + wt, "--title", "SteerRunner initial execution sl-test", "--json", "--environment", p.Environment}
				if mode == "initial-extra-command" {
					initialArgs = append(initialArgs, "--command", "arbitrary code")
				}
				if mode == "initial-wrong-title" {
					initialArgs[5] = "foreign"
				}
				initial := map[string]any{"handle": "original", "incarnationId": "initial-inc", "worktreeId": wt}
				add("initial-create", initialArgs, map[string]any{"terminal": initial}, "runtime", mode == "initial-unknown-create")
				sole := initial
				if mode == "initial-changed-incarnation" {
					sole = map[string]any{"handle": "original", "incarnationId": "replacement", "worktreeId": wt}
				}
				add("initial-sole", []string{"terminal", "list", "--worktree", "id:" + wt}, inventory([]any{sole}, false), "runtime", false)
				add("initial-shown", []string{"terminal", "show", "--terminal", "original"}, map[string]any{"terminal": sole}, "runtime", false)
				if mode == "initial-duplicate" {
					add("initial-create-again", initialArgs, map[string]any{"terminal": initial}, "runtime", false)
				}
			}
			add("launch", []string{"terminal", "send", "--terminal", "original", "--text", "helper launch"}, map[string]any{"send": map[string]any{"handle": "original", "accepted": true}}, "runtime", false)
			if mode != "missing-ready" {
				r := ready
				if mode == "changed-ready-hash" {
					r.RuntimeSHA256 = strings.Repeat("f", 64)
				}
				add("ready", []string{"terminal", "read", "--terminal", "original"}, map[string]any{"terminal": map[string]any{"handle": "original", "tail": []string{codex.RemoteReadyPrefix + encoded(r)}}}, "runtime", false)
			}
			add("business", []string{"terminal", "send", "--terminal", "original", "--text", wire}, map[string]any{"send": map[string]any{"handle": "original", "accepted": true}}, "runtime", false)
			terminal := func(inc string) any {
				return map[string]any{"terminal": map[string]any{"handle": "inspector", "incarnationId": inc, "worktreeId": wt}}
			}
			add("query", args, terminal("incarnation"), "runtime", mode == "unknown-create" || mode == "inventory-unknown-create")
			inc := "incarnation"
			if mode == "changed-incarnation" {
				inc = "replacement"
			}
			add("bound-before-read", []string{"terminal", "show", "--terminal", "inspector"}, terminal(inc), "runtime", false)
			receipt := codex.RemoteTurnReceipt{Readiness: ready, Source: codex.RemoteTurnSource, InputDigest: receiptHash(wire), Started: true, Completed: true, Status: "completed", NativeTurnID: "01a1453f-92e5-7921-a116-8957c21b5a93", UserItemID: "original-user", FinalItemID: "original-final", FinalText: "report", FinalDigest: receiptHash("report")}
			switch mode {
			case "other-receipt-session":
				receipt.Readiness.SessionID = "01a1453f-0327-7f21-b0b9-ee79dad66109"
			case "other-receipt-input":
				receipt.InputDigest = strings.Repeat("e", 64)
			case "changed-final":
				receipt.FinalText = "changed"
			case "pending-native-input":
				receipt.Started = false
				receipt.Completed = false
				receipt.NativeTurnID = ""
				receipt.UserItemID = ""
				receipt.FinalText = ""
				receipt.FinalDigest = ""
				receipt.FinalItemID = ""
				receipt.Status = ""
			}
			readRuntime, readHandle := "runtime", "inspector"
			if mode == "other-read-runtime" {
				readRuntime = "replacement"
			}
			if mode == "other-read-handle" {
				readHandle = "other"
			}
			add("native-receipt", []string{"terminal", "read", "--terminal", "inspector"}, map[string]any{"terminal": map[string]any{"handle": readHandle, "tail": []string{codex.RemoteTurnPrefix + encoded(receipt)}}}, readRuntime, mode == "canceled-read-closed")
			add("bound-before-close", []string{"terminal", "show", "--terminal", "inspector"}, terminal("incarnation"), "runtime", false)
			if mode == "auxiliary-input" {
				add("foreign-input", []string{"terminal", "send", "--terminal", "inspector", "--text", "do work"}, map[string]any{"send": map[string]any{"handle": "inspector", "accepted": true}}, "runtime", false)
			}
			if strings.HasPrefix(mode, "inventory-") {
				add("lost-close", []string{"terminal", "close", "--terminal", "inspector"}, nil, "runtime", true)
				add("absent-show", []string{"terminal", "show", "--terminal", "inspector"}, nil, "runtime", true)
				inventoryRuntime, inventoryWT := "runtime", wt
				if mode == "inventory-other-runtime" {
					inventoryRuntime = "foreign"
				}
				if mode == "inventory-other-worktree" {
					inventoryWT = "repo::C:/other"
				}
				terminals := []any{}
				if mode == "inventory-other-incarnation" {
					terminals = append(terminals, map[string]any{"handle": "inspector", "incarnationId": "replacement", "worktreeId": wt})
				}
				add("inventory", []string{"terminal", "list", "--worktree", "id:" + inventoryWT}, map[string]any{"terminals": terminals, "truncated": mode == "inventory-truncated", "totalCount": len(terminals), "hostScope": map[string]any{"hostIds": []string{"host"}, "omittedHostIds": []string{}}}, inventoryRuntime, false)
			} else if mode != "no-close" {
				add("close", []string{"terminal", "close", "--terminal", "inspector"}, map[string]any{"close": map[string]any{"handle": "inspector", "ptyKilled": mode != "close-not-confirmed"}}, "runtime", false)
			}
			if mode == "missing-close-intent" {
				calls = append(calls[:len(calls)-2], calls[len(calls)-1])
			}
			if mode == "duplicate-close-receipt" {
				calls = append(calls, calls[len(calls)-1])
			}
			raw := []byte{}
			for _, c := range calls {
				line, _ := json.Marshal(c)
				raw = append(raw, line...)
				raw = append(raw, '\n')
			}
			got, err := AnalyzeNativeAudit(raw, "runtime", p)
			admitted := err == nil && got.Complete && got.Creations == 1
			want := mode == "valid" || mode == "canceled-read-closed" || mode == "pending-native-input" || mode == "inventory-absent" || mode == "initial-valid"
			if admitted != want {
				t.Fatalf("mode=%s facts=%+v err=%v", mode, got, err)
			}
			if want && (got.Inspections != 1 || got.ClosedInspections != 1 || len(got.Prompts) != 2) {
				t.Fatal(got)
			}
			if mode == "initial-valid" && got.InitialTerminals != 1 {
				t.Fatal("initial creation was not separately recorded", got)
			}
		})
	}
}

func TestAuthenticationRevokerAuditRequiresOriginalProofAndClosure(t *testing.T) {
	for _, mode := range []string{"valid", "no-control", "foreign-helper", "foreign-session", "foreign-workspace", "no-proof", "unconfirmed-revoke", "unconfirmed-socket", "foreign-receipt", "no-close", "unknown-create", "extra-agent"} {
		t.Run(mode, func(t *testing.T) {
			policy := NativeReceiptAuditPolicy{Executable: "C:/trusted/helper.exe", SHA256: strings.Repeat("a", 64), ConfigurationDigest: strings.Repeat("b", 64), AccessMode: "sandbox", Environment: "paired"}
			ready := codex.RemoteReadiness{AuthenticationControl: mode != "no-control", Verified: true, NativeTransport: codex.RemoteNativeTransport, RuntimeSHA256: policy.SHA256, ConfigurationDigest: policy.ConfigurationDigest, AccessMode: policy.AccessMode, Contract: codex.PolicyContract, Repository: RemoteURL, Workspace: "C:/worker", SessionID: "01a1453f-0327-7f21-b0b9-ee79dad66108", Process: b.ProcessIdentity{PID: 1, Created: "original"}, CheckedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
			encoded := func(v any) string { raw, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(raw) }
			calls := []nativeAuditCall{}
			add := func(id string, args []string, result any, failed bool) {
				raw, _ := json.Marshal(map[string]any{"ok": !failed, "result": result, "_meta": map[string]string{"runtimeId": "runtime"}})
				calls = append(calls, nativeAuditCall{Kind: "intent", ID: id, Args: args}, nativeAuditCall{Kind: "receipt", ID: id, Args: args, Response: raw, Failed: failed})
			}
			wt := "repo::C:/worker"
			add("work", []string{"worktree", "create"}, map[string]any{"worktree": map[string]string{"id": wt, "path": ready.Workspace}}, false)
			add("launch", []string{"terminal", "send", "--terminal", "original", "--text", "launch"}, map[string]any{"send": map[string]any{"handle": "original", "accepted": true}}, false)
			add("ready", []string{"terminal", "read", "--terminal", "original"}, map[string]any{"terminal": map[string]any{"handle": "original", "tail": []string{codex.RemoteReadyPrefix + encoded(ready)}}}, false)
			query := codex.RemoteAuthenticationQuery{Workspace: ready.Workspace, Session: ready.SessionID}
			if mode == "foreign-session" {
				query.Session = "foreign"
			}
			if mode == "foreign-workspace" {
				query.Workspace = "C:/foreign"
			}
			helper := policy.Executable
			if mode == "foreign-helper" {
				helper = "C:/foreign/helper.exe"
			}
			args := []string{"terminal", "create", "--worktree", "id:" + wt, "--title", "SteerRunner original authentication revocation", "--command", "& '" + helper + "' '--authentication-stop' '" + encoded(query) + "'", "--json", "--environment", policy.Environment}
			if mode == "extra-agent" {
				args = append(args, "--agent", "codex")
			}
			term := map[string]any{"terminal": map[string]any{"handle": "revoker", "incarnationId": "owned-inc", "worktreeId": wt}}
			add("create", args, term, mode == "unknown-create")
			add("show", []string{"terminal", "show", "--terminal", "revoker"}, term, false)
			proof := codex.RemoteAuthenticationCleanup{Readiness: ready, Revoked: mode != "unconfirmed-revoke", BrokerClosed: true, SocketCleaned: mode != "unconfirmed-socket", Mode: "host-gh-repository-broker"}
			if mode == "foreign-receipt" {
				proof.Readiness.SessionID = "foreign"
			}
			if mode != "no-proof" {
				add("read", []string{"terminal", "read", "--terminal", "revoker"}, map[string]any{"terminal": map[string]any{"handle": "revoker", "tail": []string{codex.RemoteAuthenticationPrefix + encoded(proof)}}}, false)
			}
			add("before-close", []string{"terminal", "show", "--terminal", "revoker"}, term, false)
			if mode != "no-close" {
				add("close", []string{"terminal", "close", "--terminal", "revoker"}, map[string]any{"close": map[string]any{"handle": "revoker", "ptyKilled": true}}, false)
			}
			var lines []string
			for _, call := range calls {
				raw, _ := json.Marshal(call)
				lines = append(lines, string(raw))
			}
			facts, err := AnalyzeNativeAudit([]byte(strings.Join(lines, "\n")), "runtime", policy)
			passing := err == nil && facts.Complete && facts.Creations == 1 && facts.Inspections == 1 && facts.ClosedInspections == 1
			if passing != (mode == "valid") {
				t.Fatal("authentication cleanup audit lost strict identity/proof/closure checks", mode, facts, err)
			}
		})
	}
}
