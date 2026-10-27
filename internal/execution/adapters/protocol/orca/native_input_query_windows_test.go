package orca

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestThinCompletionInputRequiresStableOriginalResources(t *testing.T) {
	for _, fault := range []string{"none", "preview", "changed-preview", "incarnation", "terminal", "worktree", "disconnected", "agent", "workspace", "branch", "provider", "path", "main-time", "body", "partial-main", "partial-observation", "partial-marker", "boundary", "restored", "duplicate", "same-text-duplicate", "assistant-only", "runtime", "child"} {
		t.Run(fault, func(t *testing.T) {
			reads, shows := 0, 0
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				main := map[string]any{"state": "done", "stateStartedAt": 20}
				provider := map[string]string{"id": "provider", "transcriptPath": "original.jsonl"}
				state := map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "state": "done", "prompt": "", "mainAgent": main, "updatedAt": 20, "lastAssistantMessage": "canonical final"}
				if fault == "preview" || fault == "changed-preview" {
					state["prompt"] = "original full input"
				}
				if reads > 0 {
					switch fault {
					case "changed-preview":
						state["prompt"] = "other input"
					case "provider":
						provider["id"] = "other"
					case "path":
						provider["transcriptPath"] = "other.jsonl"
					case "main-time":
						main["stateStartedAt"], state["updatedAt"] = 21, 21
					case "body":
						state["lastAssistantMessage"] = "changed"
					case "child":
						state["parentPaneKey"] = "foreign"
					}
				}
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "terminal.show":
					shows++
					terminal := map[string]any{"handle": "terminal", "incarnationId": "incarnation", "worktreeId": "worktree", "worktreePath": "original", "branch": "original-branch", "agentIdentity": "codex", "connected": true}
					if shows > 1 {
						switch fault {
						case "incarnation":
							terminal["incarnationId"] = "replacement"
						case "terminal":
							terminal["handle"] = "replacement"
						case "worktree":
							terminal["worktreeId"] = "other"
						case "disconnected":
							terminal["connected"] = false
						case "agent":
							terminal["agentIdentity"] = "pi"
						case "workspace":
							terminal["worktreePath"] = "other"
						case "branch":
							terminal["branch"] = "other"
						}
					}
					runtime := "runtime"
					if fault == "runtime" && shows > 1 {
						runtime = "replacement"
					}
					ipcReply(conn, req, runtime, map[string]any{"terminal": terminal})
				case "nativeChat.readSession":
					reads++
					messages := []any{inputMessage("original-user", 12, "original full input"), map[string]any{"role": "assistant", "blocks": []any{map[string]string{"type": "text", "text": "TRANSCRIPT IS NOT FINAL"}}}}
					if fault == "duplicate" || fault == "same-text-duplicate" {
						messages = append(messages, inputMessage("other", 13, "original full input"))
					}
					if fault == "assistant-only" {
						messages = messages[1:]
					}
					ipcReply(conn, req, "runtime", map[string]any{"messages": messages, "hasMore": false, "beforeOffset": 0})
				case "worktree.ps":
					ipcReply(conn, req, "runtime", map[string]any{"worktrees": []any{map[string]any{"worktreeId": "worktree", "agents": []any{state}}}, "totalCount": 1, "truncated": false})
				case "session.tabs.list":
					metadata := map[string]any{"state": "done", "agentType": "codex", "providerSession": provider}
					switch fault {
					case "partial-main":
						metadata["mainAgent"] = map[string]any{"state": "done", "stateStartedAt": 19}
					case "partial-observation":
						metadata["observation"] = map[string]any{"authorityId": "replacement"}
					case "partial-marker":
						metadata["turnStartedAt"] = 13
					case "boundary":
						metadata["sessionBoundary"] = true
					case "restored":
						metadata["restoredUnconfirmed"] = true
					}
					ipcReply(conn, req, "runtime", map[string]any{"worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": metadata}}})
				default:
					t.Errorf("unexpected method %s", req.Method)
				}
			})
			ctx := context.Background()
			p := b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "worktree", Terminal: "terminal", Incarnation: "incarnation"}
			before, err := QueryOriginalAgent(ctx, "", p, "tab:leaf")
			if err != nil {
				t.Fatal(err)
			}
			after, err := QueryOriginalInput(ctx, "", p, "tab:leaf", 10, before)
			if fault != "none" && fault != "preview" {
				if err == nil || after.NativeInput.MatchesOriginalQuery(after) {
					t.Fatal("unverified completion acquired input evidence", fault, err)
				}
				return
			}
			i := after.NativeInput
			if err != nil || !i.MatchesOriginalQuery(after) || !i.MatchesOriginalProcess(p) || i.Observation != nil || after.LastAssistantMessage != "canonical final" || shows != 2 {
				t.Fatal("original query did not corroborate the canonical record", after, err, shows)
			}
			changed := after
			changed.LastAssistantMessage = "different final"
			if i.MatchesOriginalQuery(changed) {
				t.Fatal("query evidence authorized another completion")
			}
			p.Incarnation = "other"
			if i.MatchesOriginalProcess(p) {
				t.Fatal("query evidence authorized another process")
			}
			raw, _ := json.Marshal(i)
			var decoded NativeInput
			_ = json.Unmarshal(raw, &decoded)
			if decoded.MatchesOriginalQuery(after) {
				t.Fatal("serialized input fabricated query authority")
			}
		})
	}
}
