package orca

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaLocalQueryFeed(t *testing.T) {
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
			ipcReply(conn, req, "runtime", map[string]any{"worktrees": []any{map[string]any{"worktreeId": "worktree", "agents": []any{map[string]any{"paneKey": "tab:leaf", "agentType": "codex", "providerSession": map[string]string{"id": "provider"}, "prompt": "short turn", "mainAgent": map[string]any{"state": "done", "stateStartedAt": 20}, "updatedAt": 20, "lastAssistantMessage": "complete"}}}}})
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	p := b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "worktree", Terminal: "terminal", Incarnation: "incarnation"}
	feed, err := OpenStateFeed(ctx, "", p)
	cancel() // Initialization timeout does not own the feed lifetime.
	if err != nil {
		t.Fatal(err)
	}
	defer feed.Close()
	if feed.Initial().PaneKey != "tab:leaf" || feed.Initial().ProviderSession.ID != "provider" {
		t.Fatal(feed.Initial())
	}
	if events, err := feed.Drain(); err != nil || len(events) != 0 {
		t.Fatal("query feed synthesized events", events, err)
	}
	if queries.Load() != 0 || subscriptions.Load() != 0 {
		t.Fatal("unexpected background observation")
	}
	state, err := feed.Query(context.Background())
	if err != nil || state.LastAssistantMessage != "complete" || state.MainAgent.State != "done" || state.Observation != nil {
		t.Fatal(state, err)
	}
	replacement.Store(true)
	if _, err := feed.Query(context.Background()); !errors.Is(err, ErrStateIdentityChanged) {
		t.Fatal("provider mismatch accepted", err)
	}
	replacement.Store(false)
	block.Store(true)
	queryDone := make(chan error, 1)
	go func() { _, err := feed.Query(context.Background()); queryDone <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("query did not start")
	}
	feed.Close()
	feed.Close()
	select {
	case err := <-queryDone:
		if err == nil {
			t.Fatal("closed query succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not cancel query")
	}
	if _, err := feed.Query(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatal("closed feed queried", err)
	}
	if subscriptions.Load() != 0 {
		t.Fatal("local feed attempted subscription")
	}
}
