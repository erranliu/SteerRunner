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
	for _, mode := range []string{"valid", "reader-unverified", "query-failed", "failed-reader-unverified", "foreign-reader-owner", "lost-send-receipt", "lost-close-inventory", "wrong-helper", "title-only", "late-create", "extra-create-flag", "wrong-workspace", "wrong-ready", "wrong-incarnation", "wrong-session", "wrong-input", "wrong-response", "changed-final", "bad-auth", "retired-app-auth", "missing-close", "unconfirmed-close", "unknown-create", "unresolved-send", "extra-send-flag", "initial-valid", "initial-no-empty", "initial-extra-command", "model-valid", "model-switched", "workspace-valid", "workspace-dirty", "workspace-reader-live", "workspace-foreign-ready", "workspace-query-failed"} {
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
			if mode == "model-valid" || mode == "model-switched" {
				wire = codex.RemoteModelPrompt(strings.Repeat("c", 32), strings.Repeat("d", 64), "audit.code.execute", "reviewer", "high", "report")
			}
			add("business", []string{"terminal", "send", "--terminal", "business", "--text", wire}, map[string]any{"send": map[string]any{"handle": "business", "accepted": true}}, false)
			query := codex.RemoteControlRequest{ID: strings.Repeat("e", 32), Binding: setup.Binding, Method: "original-turn", Session: ready.SessionID, InputDigest: receiptHash(wire)}
			if mode == "wrong-session" {
				query.Session = "01a1453f-0327-7f21-b0b9-ee79dad66109"
			}
			if mode == "wrong-input" {
				query.InputDigest = strings.Repeat("f", 64)
			}
			methods := []string{"original-turn", "authentication-stop"}
			if strings.HasPrefix(mode, "workspace-") {
				methods = []string{"original-turn", "workspace-audit", "authentication-stop"}
			}
			for _, method := range methods {
				query.Audit = nil
				if method == "workspace-audit" {
					query.ID = strings.Repeat("a", 32)
					query.Method = method
					query.InputDigest = ""
					query.Audit = &codex.RemoteWorkspaceAuditQuery{Intent: b.WorkspaceIntent{Path: ready.Workspace, Branch: "main", BaseCommit: strings.Repeat("b", 40)}}
				}
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
					response.Turn = &codex.RemoteTurnReceipt{ReaderWritersAbsent: true, Source: codex.RemoteTurnSource, Readiness: ready, InputDigest: query.InputDigest, Started: true, Completed: true, Status: "completed", NativeTurnID: "01a14502-5798-78f1-aaa3-2d7cd15594c8", UserItemID: "user", FinalItemID: "final", FinalText: "complete final", FinalDigest: receiptHash("complete final")}
					if mode == "model-valid" || mode == "model-switched" {
						response.Turn.Model = "reviewer"
						response.Turn.ReasoningEffort = "high"
						if mode == "model-switched" {
							response.Turn.Model = "writer"
						}
					}
					if mode == "reader-unverified" || mode == "failed-reader-unverified" {
						response.Turn.ReaderWritersAbsent = false
					}
					if mode == "query-failed" || mode == "failed-reader-unverified" {
						response.Turn.InspectionFailed = true
						response.Error = "original_turn_unverified"
					}
					if mode == "foreign-reader-owner" {
						response.Turn.Readiness.Owner.Created = "foreign"
					}
					if mode == "changed-final" {
						response.Turn.FinalDigest = "other"
					}
				} else if method == "workspace-audit" {
					response.Workspace = &codex.RemoteWorkspaceAudit{Readiness: ready, WritersAbsent: true, Inspection: b.WorkspaceInspection{Repository: ready.Workspace, Worktree: ready.Workspace, Branch: query.Audit.Intent.Branch, Base: query.Audit.Intent.BaseCommit, Head: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40), Qualified: true, InspectionScope: "working_tree", CheckedAt: time.Now().UTC()}}
					if mode == "workspace-dirty" {
						response.Workspace.Inspection.Dirty = true
					}
					if mode == "workspace-reader-live" {
						response.Workspace.WritersAbsent = false
					}
					if mode == "workspace-foreign-ready" {
						response.Workspace.Readiness.SessionID = "foreign"
					}
					if mode == "workspace-query-failed" {
						response.Error = "workspace_audit_unverified"
						response.Workspace.Inspection.Qualified = false
					}
				} else {
					response.Authentication = &codex.RemoteAuthenticationCleanup{Readiness: ready, Revoked: mode != "bad-auth", BrokerClosed: true, SocketCleaned: true, Mode: "host-gh-repository-broker"}
					if mode == "retired-app-auth" {
						response.Authentication.Mode = "github-app-repository-broker"
					}
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
			if mode == "unresolved-send" && (err != nil || !facts.PendingControlRequests || facts.Complete || len(facts.Prompts) != 2) {
				t.Fatal("pending control input corrupted business fence or passed final audit", facts, err)
			}
			passed := err == nil && facts.Complete && facts.Creations == 1 && facts.Inspections == 1 && facts.ClosedInspections == 1 && len(facts.Prompts) == 2
			want := mode == "valid" || mode == "query-failed" || mode == "lost-send-receipt" || mode == "lost-close-inventory" || mode == "initial-valid" || mode == "model-valid" || mode == "workspace-valid" || mode == "workspace-query-failed"
			if passed != want {
				t.Fatalf("mode=%s facts=%+v err=%v want=%t", mode, facts, err, want)
			}
		})
	}
}
