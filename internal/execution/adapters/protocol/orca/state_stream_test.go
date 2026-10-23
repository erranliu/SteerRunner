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
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"golang.org/x/crypto/nacl/box"
)

func TestOrcaNativeStateStream(t *testing.T) {
	for _, scenario := range []string{"subscription-query", "sample-feed", "wrong-runtime", "wrong-request", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
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
					if request.Method == "worktree.ps" {
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
					if scenario == "sample-feed" && request.Method == "session.tabs.subscribe" {
						result = map[string]any{"type": "snapshot", "worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"state": "working", "lastAssistantMessage": "old"}}}}
					}
					if socket.write(ctx, map[string]any{"id": id, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
						return
					}
					if scenario == "sample-feed" && request.Method == "session.tabs.subscribe" {
						update := map[string]any{"type": "updated", "worktree": "worktree", "tabs": []any{map[string]any{"terminal": "terminal", "incarnationId": "incarnation", "parentTabId": "tab", "leafId": "leaf", "agentStatus": map[string]any{"state": "working", "lastAssistantMessage": "public"}}}}
						if socket.write(ctx, map[string]any{"id": id, "ok": true, "result": update, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
							return
						}
					}
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			t.Setenv("ORCA_USER_DATA_PATH", dir)
			store, _ := json.Marshal(map[string]any{"environments": []any{map[string]any{"id": "test-remote", "preferredEndpointId": "endpoint", "endpoints": []any{map[string]string{"id": "endpoint", "endpoint": "ws" + strings.TrimPrefix(server.URL, "http"), "deviceToken": "test-only-token", "publicKeyB64": base64.StdEncoding.EncodeToString(pub[:])}}}}})
			if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), store, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if scenario == "sample-feed" {
				feed, err := OpenStateFeed(ctx, "test-remote", b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "worktree", Terminal: "terminal", Incarnation: "incarnation"})
				if err != nil {
					t.Fatal(err)
				}
				defer feed.Close()
				f := feed.(*nativeStateFeed)
				for {
					f.mu.Lock()
					ready := len(f.updates) == 2
					f.mu.Unlock()
					if ready {
						break
					}
					select {
					case <-ctx.Done():
						t.Fatal("notification not buffered")
					case <-time.After(time.Millisecond):
					}
				}
				consumedAt := time.Now().UTC()
				samples, err := feed.Drain()
				if err != nil || len(samples) != 2 || samples[0].Kind != SampleInitial || samples[1].Kind != SampleNotification || samples[1].LastAssistantMessage != "public" || samples[1].ReceivedAt.IsZero() || !samples[1].ReceivedAt.Before(consumedAt) || !samples[0].ReceivedAt.Equal(feed.Initial().ReceivedAt) {
					t.Fatal("receive metadata was lost or stamped at drain", samples, err)
				}
				if samples, err = feed.Drain(); err != nil || len(samples) != 0 {
					t.Fatal(samples, err)
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
