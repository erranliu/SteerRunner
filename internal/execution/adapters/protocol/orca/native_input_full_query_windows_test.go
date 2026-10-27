package orca

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestCorroboratedCompletionInputRequiresStableOriginalResources(t *testing.T) {
	for _, fault := range []string{"none", "preview", "changed-preview", "incarnation", "workspace", "branch", "provider", "path", "main-time", "body", "authority", "revision", "marker", "missing-marker", "missing-observation", "partial-observation", "mismatched-main", "boundary", "restored", "tool-output", "duplicate", "same-text-duplicate", "shortened-user"} {
		t.Run(fault, func(t *testing.T) {
			reads, shows := 0, 0
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				main := map[string]any{"state": "done", "stateStartedAt": 20}
				provider := map[string]string{"id": "provider", "transcriptPath": "original.jsonl"}
				state := map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "state": "done", "prompt": "", "mainAgent": main, "updatedAt": 20, "turnStartedAt": 12, "lastAssistantMessage": "canonical final"}
				if fault == "preview" || fault == "changed-preview" {
					state["prompt"] = "original full input"
				}
				observation := map[string]any{"origin": "hook", "kind": "transition", "authorityId": "authority", "incarnation": 1, "revision": 2, "observedAt": 20}
				marker := int64(12)
				if reads > 0 {
					switch fault {
					case "changed-preview":
						state["prompt"] = "other input"
					case "provider":
						provider["id"] = "other"
					case "path":
						provider["transcriptPath"] = "other.jsonl"
					case "main-time":
						main["stateStartedAt"] = 21
						state["updatedAt"] = 21
					case "body":
						state["lastAssistantMessage"] = "other"
					case "authority":
						observation["authorityId"] = "other"
					case "revision":
						observation["revision"] = 3
					case "marker":
						marker = 13
						state["turnStartedAt"] = 13
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
							terminal["incarnationId"] = "other"
						case "workspace":
							terminal["worktreePath"] = "other"
						case "branch":
							terminal["branch"] = "other"
						}
					}
					ipcReply(conn, req, "runtime", map[string]any{"terminal": terminal})
				case "nativeChat.readSession":
					reads++
					original := inputMessage("original", 12, "original full input")
					if fault == "shortened-user" {
						original["shortened"] = true
					}
					messages := []any{original}
					if fault == "duplicate" || fault == "same-text-duplicate" {
						text := "another input"
						if fault == "same-text-duplicate" {
							text = "original full input"
						}
						messages = append(messages, inputMessage("other", 13, text))
					}
					ipcReply(conn, req, "runtime", map[string]any{"messages": messages, "hasMore": false, "beforeOffset": 0})
				case "worktree.ps":
					ipcReply(conn, req, "runtime", map[string]any{"worktrees": []any{map[string]any{"worktreeId": "worktree", "agents": []any{state}}}, "totalCount": 1, "truncated": false})
				case "session.tabs.list":
					metadata := map[string]any{"state": "done", "agentType": "codex", "providerSession": provider, "mainAgent": main, "observation": observation, "turnStartedAt": marker}
					switch fault {
					case "missing-marker":
						delete(metadata, "turnStartedAt")
					case "missing-observation":
						delete(metadata, "observation")
					case "partial-observation":
						metadata["observation"] = map[string]any{"authorityId": "authority"}
					case "mismatched-main":
						metadata["mainAgent"] = map[string]any{"state": "done", "stateStartedAt": 19}
					case "boundary":
						metadata["sessionBoundary"] = true
					case "restored":
						metadata["restoredUnconfirmed"] = true
					case "tool-output":
						metadata["lastAssistantMessageIsToolOutput"] = true
					}
					ipcReply(conn, req, "runtime", map[string]any{"worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": metadata}}})
				default:
					t.Errorf("unexpected read-only method %s", req.Method)
				}
			})
			p := b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "worktree", Terminal: "terminal", Incarnation: "incarnation"}
			before, err := QueryOriginalAgent(context.Background(), "", p, "tab:leaf")
			if err != nil {
				t.Fatal(err)
			}
			after, err := QueryOriginalInput(context.Background(), "", p, "tab:leaf", 10, before)
			if fault != "none" && fault != "preview" {
				if err == nil && after.NativeInput.MatchesOriginalQuery(after) {
					t.Fatal("unverified record acquired completion query proof", fault)
				}
				return
			}
			i := after.NativeInput
			if err != nil || !i.MatchesOriginalQuery(after) || !i.MatchesOriginalProcess(p) || i.Observation != nil || after.LastAssistantMessage != "canonical final" || shows != 2 {
				t.Fatal("full metadata did not bind exact canonical completion", err, after, shows)
			}
			changed := after
			meta := *after.MetadataObservation
			meta.Revision++
			changed.MetadataObservation = &meta
			if i.MatchesOriginalQuery(changed) {
				t.Fatal("proof authorized a different metadata revision")
			}
			after.MetadataObservation.Revision++
			if i.MatchesOriginalQuery(after) {
				t.Fatal("in-place metadata mutation changed the frozen query proof")
			}
			raw, _ := json.Marshal(i)
			var decoded NativeInput
			_ = json.Unmarshal(raw, &decoded)
			if decoded.MatchesOriginalQuery(after) {
				t.Fatal("wire input fabricated query authority")
			}
		})
	}
}
