package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOriginalAgentQueryCoversLimitedSnapshot(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		limits []int
		ok     bool
	}{
		{"beyond-first-100", []int{100, 151}, true},
		{"growing-snapshot", []int{100, 151, 152}, true},
		{"server-still-truncates", []int{100, 151}, false},
		{"inconsistent-count", []int{100}, false},
		{"missing-completeness", []int{100}, false},
		{"keeps-growing", []int{100, 101, 102}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var mu sync.Mutex
			var limits []int
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "session.tabs.list":
					ipcReply(conn, req, "runtime", map[string]any{"worktree": "original", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"providerSession": map[string]string{"id": "provider"}}}}})
				case "worktree.ps":
					var params struct{ Limit int }
					_ = json.Unmarshal(req.Params, &params)
					mu.Lock()
					limits = append(limits, params.Limit)
					call := len(limits)
					mu.Unlock()
					total := 151
					if scenario.name == "growing-snapshot" && call > 1 {
						total = 152
					}
					if scenario.name == "keeps-growing" {
						total = params.Limit + 1
					}
					count := min(params.Limit, total)
					if scenario.name == "server-still-truncates" {
						count = 100
					}
					rows := make([]any, count)
					for i := range rows {
						id := fmt.Sprintf("other-%d", i)
						if i == total-1 {
							id = "original"
						}
						// The same pane key on other worktrees must never be selected.
						rows[i] = map[string]any{"worktreeId": id, "agents": []any{map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "mainAgent": map[string]any{"state": "done", "stateStartedAt": 20}, "lastAssistantMessage": id, "providerSession": map[string]string{"id": "provider"}}}}
					}
					page := map[string]any{"worktrees": rows, "totalCount": total, "truncated": count < total}
					if scenario.name == "inconsistent-count" {
						page["truncated"] = false
					}
					if scenario.name == "missing-completeness" {
						delete(page, "totalCount")
					}
					ipcReply(conn, req, "runtime", page)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			state, err := QueryOriginalAgent(ctx, "", b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "original", Terminal: "terminal", Incarnation: "incarnation"}, "tab:leaf")
			if scenario.ok {
				if err != nil || state.LastAssistantMessage != "original" || state.ProviderSession.ID != "provider" {
					t.Fatal("original query lost target beyond first page", state, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "absent") {
				t.Fatal("incomplete snapshot became an absence result", state, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(limits, scenario.limits) {
				t.Fatal("unexpected query budget/limits", limits)
			}
		})
	}
}

func TestOriginalAgentQueryRetainedStampRequiresSameMainRecord(t *testing.T) {
	for _, mode := range []string{"same", "matching-query-marker", "conflicting-query-marker", "missing-metadata-marker", "different-state", "different-time"} {
		t.Run(mode, func(t *testing.T) {
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				main := map[string]any{"state": "done", "stateStartedAt": 20}
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "session.tabs.list":
					stamp := 10
					if mode == "missing-metadata-marker" {
						stamp = 0
					}
					if mode == "different-state" {
						main["state"] = "working"
					}
					if mode == "different-time" {
						main["stateStartedAt"] = 21
					}
					ipcReply(conn, req, "runtime", map[string]any{"worktree": "original", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"mainAgent": main, "turnStartedAt": stamp, "providerSession": map[string]string{"id": "provider"}, "lastAssistantMessage": "metadata must not replace body"}}}})
				case "worktree.ps":
					stamp := 10
					if mode == "same" {
						stamp = 0 // Current hook-store projections omit the marker.
					} else if mode == "conflicting-query-marker" {
						stamp++
					}
					ipcReply(conn, req, "runtime", map[string]any{"totalCount": 1, "truncated": false, "worktrees": []any{map[string]any{"worktreeId": "original", "agents": []any{map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "mainAgent": main, "turnStartedAt": stamp, "lastAssistantMessage": "original"}}}}})
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			s, err := QueryOriginalAgent(ctx, "", b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "original", Terminal: "terminal", Incarnation: "incarnation"}, "tab:leaf")
			if err != nil || (s.TurnStartedAt == 10) != (mode == "same" || mode == "matching-query-marker") || s.LastAssistantMessage != "original" {
				t.Fatal("query mixed independent records", s, err)
			}
		})
	}
}
