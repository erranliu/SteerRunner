package orca

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/nacl/box"
)

// Observe the real websocket read's cancellation registration. The barrier
// prevents the cancellation case from passing with an already-cancelled call.
type stateStreamReadContext struct {
	context.Context
	reading chan struct{}
}

func (c stateStreamReadContext) Done() <-chan struct{} {
	select {
	case c.reading <- struct{}{}:
	default:
	}
	return c.Context.Done()
}

func TestOrcaNativeStateStream(t *testing.T) {
	for _, scenario := range []string{"subscription-query", "subscription-updates-cancel", "subscription-updates-disconnect", "wrong-runtime", "wrong-request", "cancelled", "terminal-query", "terminal-query-runtime", "terminal-query-request", "terminal-query-cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			releaseUpdate := make(chan struct{})
			disconnect := make(chan struct{})
			serverDone := make(chan struct{})
			pub, private, err := box.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				ctx := r.Context()
				_, data, err := conn.Read(ctx)
				var hello struct{ Type, PublicKeyB64 string }
				if err != nil || json.Unmarshal(data, &hello) != nil || hello.Type != "e2ee_hello" {
					return
				}
				bytes, err := base64.StdEncoding.DecodeString(hello.PublicKeyB64)
				if err != nil || len(bytes) != 32 {
					return
				}
				var peer [32]byte
				copy(peer[:], bytes)
				socket := &stateSocket{conn: conn}
				box.Precompute(&socket.key, &peer, private)
				if conn.Write(ctx, websocket.MessageText, []byte(`{"type":"e2ee_ready"}`)) != nil {
					return
				}
				data, err = socket.read(ctx)
				var auth struct {
					Type, DeviceToken  string
					ClientCapabilities []string
				}
				if err != nil || json.Unmarshal(data, &auth) != nil || auth.Type != "e2ee_auth" || auth.DeviceToken != "test-only-token" || len(auth.ClientCapabilities) != 1 {
					return
				}
				if socket.write(ctx, map[string]string{"type": "e2ee_authenticated"}) != nil {
					return
				}
				for {
					data, err := socket.read(ctx)
					var request struct {
						ID, DeviceToken, Method string
						Params                  map[string]json.RawMessage
					}
					if err != nil || json.Unmarshal(data, &request) != nil || request.DeviceToken != "test-only-token" {
						return
					}
					id, runtime := request.ID, "runtime"
					if scenario == "wrong-runtime" {
						runtime = "replacement"
					}
					if scenario == "wrong-request" {
						id = "foreign"
					}
					result := any(map[string]string{"runtimeId": runtime})
					if request.Method == "terminal.show" {
						if string(request.Params["terminal"]) != `"terminal"` {
							t.Error("wrong terminal selector")
							return
						}
						if scenario == "terminal-query-cancelled" {
							_, _ = socket.read(ctx)
							return
						}
						if scenario == "terminal-query-runtime" {
							runtime = "replacement"
						}
						if scenario == "terminal-query-request" {
							id = "foreign"
						}
						result = map[string]any{"terminal": Terminal{Handle: "terminal", IncarnationID: "incarnation", WorktreeID: "worktree", WorktreePath: "/original", Branch: "worker", AgentIdentity: "codex", Connected: true}}
					} else if request.Method == "worktree.ps" {
						if string(request.Params["limit"]) != "100" {
							return
						}
						result = map[string]any{"worktrees": []any{}, "totalCount": 0, "truncated": false}
					} else if request.Method != "status.get" {
						if string(request.Params["worktree"]) != `"id:worktree"` {
							return
						}
						if scenario == "cancelled" {
							<-ctx.Done()
							return
						}
						result = map[string]any{"type": "snapshot", "worktree": "worktree", "tabs": []any{}}
					}
					if strings.HasPrefix(scenario, "subscription-updates-") && request.Method == "session.tabs.subscribe" {
						result = map[string]any{"type": "snapshot", "worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"state": "working", "lastAssistantMessage": "old"}}}}
					}
					if socket.write(ctx, map[string]any{"id": id, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
						return
					}
					if strings.HasPrefix(scenario, "subscription-updates-") && request.Method == "session.tabs.subscribe" {
						select {
						case <-releaseUpdate:
						case <-serverDone:
							return
						}
						update := map[string]any{"type": "updated", "worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"state": "working", "lastAssistantMessage": "public"}}}}
						if socket.write(ctx, map[string]any{"id": id, "ok": true, "result": update, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
							return
						}
						if scenario == "subscription-updates-disconnect" {
							select {
							case <-disconnect:
							case <-serverDone:
							}
							return
						}
					}
				}
			}))
			defer server.Close()
			defer close(serverDone)
			dir := t.TempDir()
			t.Setenv("ORCA_USER_DATA_PATH", dir)
			store, _ := json.Marshal(map[string]any{"environments": []any{map[string]any{"id": "test-remote", "preferredEndpointId": "endpoint", "endpoints": []any{map[string]string{"id": "endpoint", "endpoint": "ws" + strings.TrimPrefix(server.URL, "http"), "deviceToken": "test-only-token", "publicKeyB64": base64.StdEncoding.EncodeToString(pub[:])}}}}})
			if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), store, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if strings.HasPrefix(scenario, "terminal-query") {
				queryCtx, stop := context.WithTimeout(ctx, 300*time.Millisecond)
				defer stop()
				terminal, err := QueryBoundTerminal(queryCtx, "test-remote", TerminalIdentity{RuntimeID: "runtime", Handle: "terminal", IncarnationID: "incarnation"})
				switch scenario {
				case "terminal-query":
					if err != nil || terminal.RuntimeID != "runtime" || terminal.WorktreeID != "worktree" || terminal.WorktreePath != "/original" || terminal.AgentIdentity != "codex" {
						t.Fatal(terminal, err)
					}
				case "terminal-query-cancelled":
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Fatal(err)
					}
				default:
					if !errors.Is(err, ErrStateIdentityChanged) {
						t.Fatal("identity conflict lost", err)
					}
				}
				return
			}
			if strings.HasPrefix(scenario, "subscription-updates-") {
				initCtx, initCancel := context.WithCancel(ctx)
				sub, raw, err := SubscribeState(initCtx, "test-remote", "runtime", "id:worktree")
				initCancel() // Only initialization belongs to this context.
				if err != nil {
					t.Fatal(err)
				}
				defer sub.Close()
				original := StateResource{RuntimeID: "runtime", WorktreeID: "worktree", Handle: "terminal", IncarnationID: "incarnation"}
				initial, err := DecodeOriginalState(raw, original)
				if err != nil || initial.LastAssistantMessage != "old" || initial.PaneKey != "tab:leaf" {
					t.Fatal(initial, err)
				}
				// Independent queries complete while the same subscription is live.
				// The server releases its update only after both queries return.
				if _, err := QueryState(ctx, "test-remote", "runtime", "id:worktree"); err != nil {
					t.Fatal(err)
				}
				if _, err := QueryWorktreeAgents(ctx, "test-remote", "runtime"); err != nil {
					t.Fatal(err)
				}
				close(releaseUpdate)
				raw, err = sub.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}
				var frame struct{ Type string }
				if err := json.Unmarshal(raw, &frame); err != nil || frame.Type != "updated" {
					t.Fatal(string(raw), err)
				}
				updated, err := DecodeOriginalState(raw, original)
				if err != nil || updated.LastAssistantMessage != "public" || updated.PaneKey != initial.PaneKey {
					t.Fatal(updated, err)
				}
				nextCtx, nextCancel := context.WithCancel(ctx)
				defer nextCancel()
				reading, finished := make(chan struct{}, 1), make(chan error, 1)
				go func() { _, err := sub.Next(stateStreamReadContext{nextCtx, reading}); finished <- err }()
				select {
				case <-reading:
				case <-ctx.Done():
					t.Fatal("Next did not start reading")
				}
				if scenario == "subscription-updates-cancel" {
					nextCancel()
				} else {
					close(disconnect)
				}
				select {
				case err := <-finished:
					if scenario == "subscription-updates-cancel" {
						if !errors.Is(err, context.Canceled) {
							t.Fatal("Next lost caller cancellation", err)
						}
					} else if !IsStateConnectionError(err) {
						t.Fatal("Next lost peer disconnect", err)
					}
				case <-ctx.Done():
					t.Fatal("Next did not end on cancellation/disconnect")
				}
				return
			}
			sub, _, err := SubscribeState(ctx, "test-remote", "runtime", "id:worktree")
			if sub != nil {
				defer sub.Close()
			}
			if scenario != "subscription-query" {
				if err == nil {
					t.Fatal("unverified subscription was accepted")
				}
				if (scenario == "wrong-runtime" || scenario == "wrong-request") && !errors.Is(err, ErrStateIdentityChanged) {
					t.Fatal("native identity conflict was classified as retryable", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = QueryState(ctx, "test-remote", "runtime", "id:worktree"); err != nil {
				t.Fatal(err)
			}
			if _, err = QueryWorktreeAgents(ctx, "test-remote", "runtime"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
