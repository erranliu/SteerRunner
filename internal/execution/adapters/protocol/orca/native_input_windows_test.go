package orca

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func inputMessage(id string, ts int64, text string) map[string]any {
	return map[string]any{"id": id, "role": "user", "source": "transcript", "timestamp": ts, "blocks": []any{map[string]string{"type": "text", "text": text}}}
}

func TestOrcaNativeInputPagesRequireExactlyOneOriginalUser(t *testing.T) {
	for _, mode := range []string{"single", "paged", "foreign-same-text", "foreign-other-text", "shortened", "hook", "no-time", "no-text", "partial-page", "stuck-cursor", "absent", "budget", "negative-cursor-with-input", "nonadvancing-cursor-with-input"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method == "status.get" {
					ipcReply(conn, req, "runtime", map[string]any{})
					return
				}
				if req.Method != "nativeChat.readSession" {
					t.Errorf("unexpected mutation/method %s", req.Method)
					return
				}
				calls++
				var params struct {
					Agent, SessionID, TranscriptPath string
					BeforeOffset                     *int64
					Limit                            int
				}
				_ = json.Unmarshal(req.Params, &params)
				if params.Agent != "codex" || params.SessionID != "provider" || params.TranscriptPath != "original.jsonl" || params.Limit != 100 {
					t.Error("original identity was not forwarded")
				}
				original := inputMessage("original", 12, "original full prompt")
				messages := []any{inputMessage("bootstrap", 5, "READY"), original}
				page := map[string]any{"messages": messages, "hasMore": false, "beforeOffset": 0}
				switch mode {
				case "paged":
					if calls == 1 {
						page = map[string]any{"messages": []any{map[string]any{"role": "assistant"}}, "hasMore": true, "beforeOffset": 200}
					} else if calls == 2 {
						page = map[string]any{"messages": []any{original}, "hasMore": true, "beforeOffset": 100}
					} else {
						page["messages"] = messages[:1]
					}
				case "foreign-same-text":
					page["messages"] = append(messages, inputMessage("second", 13, "original full prompt"))
				case "foreign-other-text":
					page["messages"] = append(messages, inputMessage("second", 13, "foreign"))
				case "shortened":
					original["shortened"] = true
				case "hook":
					original["source"] = "hook"
				case "no-time":
					original["timestamp"] = nil
				case "no-text":
					original["blocks"] = []any{map[string]string{"type": "image-ref"}}
				case "partial-page":
					delete(page, "hasMore")
				case "stuck-cursor":
					page = map[string]any{"messages": []any{}, "hasMore": true, "beforeOffset": 100}
				case "absent":
					page["messages"] = messages[:1]
				case "budget":
					page = map[string]any{"messages": []any{}, "hasMore": true, "beforeOffset": 1000 - calls}
				case "negative-cursor-with-input":
					page["beforeOffset"] = -1
				case "nonadvancing-cursor-with-input":
					if calls == 1 {
						page = map[string]any{"messages": []any{}, "hasMore": true, "beforeOffset": 100}
					} else {
						page["beforeOffset"], page["hasMore"] = 100, true
					}
				}
				ipcReply(conn, req, "runtime", page)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			input, err := QueryNativeInput(ctx, "", "runtime", "provider", "original.jsonl", 10)
			ok := mode == "single" || mode == "paged"
			if (err == nil) != ok || ok && (input.ID != "original" || input.Text != "original full prompt" || input.Baseline != 10) {
				t.Fatal(mode, input, err)
			}
			if mode == "paged" && calls != 3 || mode == "stuck-cursor" && calls != 2 || mode == "budget" && calls != 16 {
				t.Fatal("wrong bounded query count", mode, calls)
			}
		})
	}
}

func TestOrcaNativeInputCannotCombineDifferentMainRecords(t *testing.T) {
	for _, mode := range []string{"same", "state", "time", "outcome", "agent", "marker"} {
		t.Run(mode, func(t *testing.T) {
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				main := map[string]any{"state": "done", "stateStartedAt": 20}
				state := map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "mainAgent": main, "turnStartedAt": 15, "updatedAt": 20, "providerSession": map[string]string{"id": "provider", "transcriptPath": "original.jsonl"}, "observation": map[string]any{"origin": "hook", "kind": "transition", "authorityId": "authority", "incarnation": 1, "revision": 12, "observedAt": 20}}
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "nativeChat.readSession":
					ipcReply(conn, req, "runtime", map[string]any{"messages": []any{inputMessage("original", 12, "original")}, "hasMore": false, "beforeOffset": 0})
				case "worktree.ps":
					ipcReply(conn, req, "runtime", map[string]any{"worktrees": []any{map[string]any{"worktreeId": "worktree", "agents": []any{state}}}, "totalCount": 1, "truncated": false})
				case "session.tabs.list":
					switch mode {
					case "state":
						main["state"] = "working"
					case "time":
						main["stateStartedAt"] = 21
					case "outcome":
						main["outcome"] = "failure"
					case "agent":
						state["agentType"] = "pi"
					case "marker":
						state["turnStartedAt"] = 16
					}
					ipcReply(conn, req, "runtime", map[string]any{"worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": state}}})
				default:
					t.Errorf("unexpected method %s", req.Method)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			p := b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "worktree", Terminal: "terminal", Incarnation: "incarnation"}
			before, err := QueryOriginalAgent(ctx, "", p, "tab:leaf")
			if err != nil {
				t.Fatal(err)
			}
			after, err := QueryOriginalInput(ctx, "", p, "tab:leaf", 10, before)
			if (err == nil) != (mode == "same") || mode == "same" && after.NativeInput == nil {
				t.Fatal("mixed source acquired input proof", mode, after.NativeInput, err)
			}
		})
	}
}

func TestOrcaNativeInputQueryRejectsIdentityChangeDuringRead(t *testing.T) {
	for _, mode := range []string{"same", "provider", "path", "marker", "main", "boundary", "restored", "interrupted", "tool-output", "prompt", "authority", "incarnation", "revision-backwards"} {
		t.Run(mode, func(t *testing.T) {
			before := AgentState{PaneKey: "tab:leaf", AgentType: "codex", MainAgent: &MainAgentState{State: "working", StateStartedAt: 11}, TurnStartedAt: 15, UpdatedAt: 20, MetadataObservation: &StateObservation{Origin: "hook", Kind: "transition", AuthorityID: "authority", Incarnation: 1, Revision: 12, ObservedAt: 20}}
			before.ProviderSession.ID, before.ProviderSession.TranscriptPath = "provider", "original.jsonl"
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				state := map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "mainAgent": map[string]any{"state": "working", "stateStartedAt": 11}, "turnStartedAt": 15, "updatedAt": 20, "providerSession": map[string]string{"id": "provider", "transcriptPath": "original.jsonl"}}
				observation := *before.MetadataObservation
				state["observation"] = &observation
				switch mode {
				case "provider":
					state["providerSession"] = map[string]string{"id": "other", "transcriptPath": "original.jsonl"}
				case "path":
					state["providerSession"] = map[string]string{"id": "provider", "transcriptPath": "other.jsonl"}
				case "marker":
					state["turnStartedAt"] = 16
				case "main":
					state["mainAgent"] = map[string]any{"state": "done", "stateStartedAt": 21}
				case "boundary":
					state["sessionBoundary"] = true
				case "restored":
					state["restoredUnconfirmed"] = true
				case "interrupted":
					state["interrupted"] = true
				case "tool-output":
					state["lastAssistantMessageIsToolOutput"] = true
				case "prompt":
					state["prompt"] = "foreign"
				case "authority":
					observation.AuthorityID = "replacement"
				case "incarnation":
					observation.Incarnation++
				case "revision-backwards":
					observation.Revision--
				}
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "nativeChat.readSession":
					ipcReply(conn, req, "runtime", map[string]any{"messages": []any{inputMessage("original", 12, "original")}, "hasMore": false, "beforeOffset": 0})
				case "worktree.ps":
					ipcReply(conn, req, "runtime", map[string]any{"worktrees": []any{map[string]any{"worktreeId": "worktree", "agents": []any{state}}}, "totalCount": 1, "truncated": false})
				case "session.tabs.list":
					ipcReply(conn, req, "runtime", map[string]any{"worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": state}}})
				default:
					t.Errorf("unexpected method %s", req.Method)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			s, err := QueryOriginalInput(ctx, "", b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "worktree", Terminal: "terminal", Incarnation: "incarnation"}, "tab:leaf", 10, before)
			if (err == nil) != (mode == "same") || mode == "same" && (s.NativeInput == nil || s.NativeInput.Text != "original") {
				t.Fatal(mode, s.NativeInput, err)
			}
		})
	}
}
