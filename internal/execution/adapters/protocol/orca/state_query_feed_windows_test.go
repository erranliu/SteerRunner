package orca

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrcaLocalOriginalQuery(t *testing.T) {
	var subscriptions, queries atomic.Int32
	var replacement atomic.Bool
	var block atomic.Bool
	entered := make(chan struct{}, 1)
	localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
		provider := "provider"
		if replacement.Load() {
			provider = "replacement"
		}
		switch req.Method {
		case "status.get":
			ipcReply(conn, req, "runtime", map[string]any{})
		case "session.tabs.subscribe":
			subscriptions.Add(1)
		case "session.tabs.list":
			ipcReply(conn, req, "runtime", map[string]any{"worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"providerSession": map[string]string{"id": provider}}}}})
		case "worktree.ps":
			queries.Add(1)
			if block.Load() {
				entered <- struct{}{}
				var one [1]byte
				_, _ = conn.Read(one[:])
				return
			}
			ipcReply(conn, req, "runtime", map[string]any{"totalCount": 1, "truncated": false, "worktrees": []any{map[string]any{"worktreeId": "worktree", "agents": []any{map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "providerSession": map[string]string{"id": "provider"}, "prompt": "short turn", "mainAgent": map[string]any{"state": "done", "stateStartedAt": 20}, "updatedAt": 20, "lastAssistantMessage": "complete"}}}}})
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	p := StateResource{RuntimeID: "runtime", WorktreeID: "worktree", Handle: "terminal", IncarnationID: "incarnation"}
	raw, err := QueryState(ctx, "", p.RuntimeID, "id:"+p.WorktreeID)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	initial, err := DecodeOriginalState(raw, p)
	if err != nil || initial.PaneKey != "tab:leaf" || initial.ProviderSession.ID != "provider" {
		t.Fatal(initial, err)
	}
	if queries.Load() != 0 || subscriptions.Load() != 0 {
		t.Fatal("unexpected background observation")
	}
	state, err := QueryOriginalAgent(context.Background(), "", p, initial.PaneKey)
	if err != nil || state.LastAssistantMessage != "complete" || state.MainAgent == nil || state.MainAgent.State != "done" || state.Observation != nil {
		t.Fatal(state, err)
	}

	replacement.Store(true)
	if _, err := QueryOriginalAgent(context.Background(), "", p, initial.PaneKey); !errors.Is(err, ErrStateIdentityChanged) {
		t.Fatal("provider mismatch accepted", err)
	}
	replacement.Store(false)
	block.Store(true)
	queryCtx, queryCancel := context.WithCancel(context.Background())
	defer queryCancel()
	queryDone := make(chan error, 1)
	go func() { _, err := QueryOriginalAgent(queryCtx, "", p, initial.PaneKey); queryDone <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("query did not start")
	}
	queryCancel()
	select {
	case err := <-queryDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("query lost caller cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("caller cancellation did not cancel query")
	}
	if subscriptions.Load() != 0 {
		t.Fatal("local query attempted subscription")
	}
}
