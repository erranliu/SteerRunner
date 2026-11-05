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
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/nacl/box"
)

// This fixture speaks the real encrypted transport; no Runner lifecycle or
// resource snapshot is needed to exercise the public conversation client.
func remoteAgentFixture(t *testing.T, scenario string, business *atomic.Int32, authenticated *atomic.Bool) string {
	t.Helper()
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
		_, raw, err := conn.Read(ctx)
		var hello struct{ Type, PublicKeyB64 string }
		if err != nil || json.Unmarshal(raw, &hello) != nil || hello.Type != "e2ee_hello" {
			t.Error("missing encrypted handshake")
			return
		}
		peerBytes, err := base64.StdEncoding.DecodeString(hello.PublicKeyB64)
		if err != nil || len(peerBytes) != 32 {
			t.Error("invalid peer key")
			return
		}
		var peer [32]byte
		copy(peer[:], peerBytes)
		socket := &stateSocket{conn: conn}
		box.Precompute(&socket.key, &peer, private)
		if conn.Write(ctx, websocket.MessageText, []byte(`{"type":"e2ee_ready"}`)) != nil {
			return
		}
		raw, err = socket.read(ctx)
		var auth struct {
			Type, DeviceToken  string
			ClientCapabilities []string
		}
		if err != nil || json.Unmarshal(raw, &auth) != nil || auth.Type != "e2ee_auth" || auth.DeviceToken != "fixture-token" {
			t.Error("invalid authentication")
			return
		}
		if !slices.Equal(auth.ClientCapabilities, remoteAgentSessionCapabilities()) {
			t.Error("authentication capabilities changed")
			return
		}
		if scenario == "auth-refused" {
			_ = socket.write(ctx, map[string]string{"type": "e2ee_rejected"})
			return
		}
		if socket.write(ctx, map[string]string{"type": "e2ee_authenticated"}) != nil {
			return
		}
		for {
			raw, err := socket.read(ctx)
			var request struct {
				ID, DeviceToken, Method string
				Params                  json.RawMessage
				ClientCapabilities      []string
			}
			if err != nil {
				return
			}
			if json.Unmarshal(raw, &request) != nil || request.DeviceToken != "fixture-token" {
				t.Error("invalid request envelope")
				return
			}
			runtime, id := "original-runtime", request.ID
			var result any
			if request.Method == "status.get" {
				var shape map[string]json.RawMessage
				_ = json.Unmarshal(raw, &shape)
				if _, found := shape["clientCapabilities"]; found || string(request.Params) != "null" {
					t.Error("bootstrap status capabilities/params changed")
					return
				}
				caps := agentSessionCapabilities()
				if scenario == "missing-capability" {
					caps = slices.DeleteFunc(caps, func(c string) bool { return c == "agentSession.create.message.v1" })
				}
				if scenario == "missing-send-capability" {
					caps = slices.DeleteFunc(caps, func(c string) bool { return c == "agent-session.send-selection.v1" })
				}
				result = map[string]any{"runtimeId": runtime, "capabilities": caps}
				if scenario == "wrong-runtime" {
					runtime = "replacement"
				}
				if scenario == "wrong-request" {
					id = "foreign"
				}
				authenticated.Store(true)
			} else {
				business.Add(1)
				if !slices.Equal(request.ClientCapabilities, auth.ClientCapabilities) {
					t.Error("business request lost authentication capabilities")
					return
				}
				switch request.Method {
				case "agentSession.send":
					var send AgentSend
					var shape map[string]json.RawMessage
					want := originalAgentSend()
					if json.Unmarshal(request.Params, &send) != nil || json.Unmarshal(request.Params, &shape) != nil || len(shape) != 3 || send.Envelope.ExpectedRuntimeFence == nil || *send.Envelope.ExpectedRuntimeFence != 3 || send.Envelope.SessionID != want.Envelope.SessionID || send.Envelope.ClientOperationID != want.Envelope.ClientOperationID || send.Envelope.PayloadFingerprint != want.Envelope.PayloadFingerprint || len(send.Body.Blocks) != 1 || send.Body.Blocks[0].Text != want.Body.Blocks[0].Text || send.Selection == nil || *send.Selection != *want.Selection {
						t.Error("send identity/body/selection changed or implicit delivery/retry added")
						return
					}
					if scenario == "lost-response" {
						return
					}
					result = map[string]bool{"ok": true}
				case "agentSession.createSupport":
					result = map[string]bool{"supported": true}
				case "aiVault.listSessions":
					result = map[string]any{"sessions": []any{map[string]any{"agent": "codex", "sessionId": "provider-session", "executionHostId": "local", "structuredSession": map[string]string{"sessionId": "oas_fixture", "workspaceId": "fixture-worktree"}}}}
				case "projectGroup.create", "folderWorkspace.create", "folderWorkspace.delete", "projectGroup.delete", "files.stat":
					if scenario != "usage-scope" {
						t.Error("unexpected usage scope operation", request.Method)
						return
					}
					want := map[string]any{}
					switch request.Method {
					case "projectGroup.create":
						want = map[string]any{"name": "scope-marker", "createdFrom": "manual"}
						result = map[string]any{"group": UsageGroup{ID: "scope-group"}}
					case "folderWorkspace.create":
						want = map[string]any{"projectGroupId": "scope-group", "name": "scope-marker", "folderPath": "C:/scope"}
						result = map[string]any{"folderWorkspace": UsageFolder{ID: "scope-folder"}}
					case "folderWorkspace.delete":
						want = map[string]any{"folderWorkspaceId": "scope-folder"}
						result = map[string]bool{"deleted": true}
					case "projectGroup.delete":
						want = map[string]any{"groupId": "scope-group"}
						result = map[string]bool{"deleted": true}
					case "files.stat":
						want = map[string]any{"worktree": "folder:scope-folder", "relativePath": "log.jsonl"}
					}
					var params map[string]any
					if json.Unmarshal(request.Params, &params) != nil || !reflect.DeepEqual(params, want) {
						t.Error("usage scope request parameters changed", request.Method)
						return
					}
					if request.Method == "files.stat" {
						if socket.write(ctx, map[string]any{"id": id, "ok": false, "error": &StateRPCError{Code: "selector_not_found"}, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
							return
						}
						continue
					}
				case "agentSession.create":
					var create AgentCreate
					if json.Unmarshal(request.Params, &create) != nil || create.FirstMessage.Body.Blocks[0].Text != "one\r\n\r\n中文😀\n```\npath\\with space\n```  \r\n" || create.FirstMessage.Selection.Model != "fixture-model" || create.FirstMessage.Selection.Effort != "medium" || create.Options["permissionMode"] != "full-access" {
						t.Error("atomic first message/selection/permission changed")
						return
					}
					if scenario == "lost-response" {
						return
					}
					result = map[string]bool{"ok": true}
				case "agentSession.close":
					result = map[string]bool{"ok": true}
				case "agentSession.history":
					if scenario == "oversized" {
						result = map[string]any{"ok": true, "padding": strings.Repeat("x", 4<<20)}
					} else {
						_, _ = socket.read(ctx)
						return
					}
				default:
					t.Errorf("unexpected method %s", request.Method)
					return
				}
			}
			if socket.write(ctx, map[string]any{"id": id, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	store, _ := json.Marshal(map[string]any{"environments": []any{map[string]any{"id": "fixture-env", "preferredEndpointId": "endpoint", "endpoints": []any{map[string]string{"id": "endpoint", "endpoint": "ws" + strings.TrimPrefix(server.URL, "http"), "deviceToken": "fixture-token", "publicKeyB64": base64.StdEncoding.EncodeToString(pub[:])}}}}})
	if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), store, 0600); err != nil {
		t.Fatal(err)
	}
	host, err := UsageHostID("fixture-env")
	if err != nil {
		t.Fatal(err)
	}
	return host
}

func TestRemoteAgentSendWireAndDispatch(t *testing.T) {
	for _, scenario := range []string{"accepted", "missing-capability", "commit-failed", "revoked", "lost-response", "missing-send-capability", "auth-refused", "wrong-runtime", "wrong-request", "wrong-host"} {
		t.Run(scenario, func(t *testing.T) {
			var business, checks atomic.Int32
			var authenticated atomic.Bool
			host := remoteAgentFixture(t, scenario, &business, &authenticated)
			if scenario == "wrong-host" {
				host = "sha256:foreign"
			}
			client := NewRemoteAgentSessionClient("fixture-env", "original-runtime", host)
			denied := errors.New("durable send commit failed")
			ctx, cancel := context.WithTimeout(WithDispatchCheckpoint(context.Background(), func() error {
				checks.Add(1)
				if !authenticated.Load() {
					t.Error("handshake consumed send checkpoint")
				}
				if scenario == "commit-failed" {
					return denied
				}
				return nil
			}), 3*time.Second)
			defer cancel()
			if scenario == "revoked" {
				cancel()
			}
			_, err := client.Send(ctx, originalAgentSend())
			switch scenario {
			case "accepted", "missing-capability", "lost-response":
				// A follow-up only requires send capabilities, not create-first-message.
				if (err != nil) != (scenario == "lost-response") || business.Load() != 1 || checks.Load() != 1 {
					t.Fatal(err, business.Load(), checks.Load())
				}
				if _, err := client.Send(ctx, originalAgentSend()); err == nil || business.Load() != 1 || checks.Load() != 1 {
					t.Fatal("original send replayed", err, business.Load(), checks.Load())
				}
			case "commit-failed":
				if !errors.Is(err, denied) || business.Load() != 0 || checks.Load() != 1 {
					t.Fatal(err, business.Load(), checks.Load())
				}
			default:
				if err == nil || business.Load() != 0 || checks.Load() != 0 {
					t.Fatal("unverified send escaped", err, business.Load(), checks.Load())
				}
			}
		})
	}
}

func TestStructuredUsageChannelDeclaresAuthAndRequestCapabilities(t *testing.T) {
	var reads atomic.Int32
	var authenticated atomic.Bool
	host := remoteAgentFixture(t, "accepted", &reads, &authenticated)
	checks := 0
	ctx := WithDispatchCheckpoint(context.Background(), func() error { checks++; return errors.New("business gate must not be used") })
	channel, err := OpenStructuredUsageFiles(ctx, "fixture-env", "original-runtime", host)
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	sessions, err := channel.Sessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].StructuredSession == nil || sessions[0].StructuredSession.SessionID != "oas_fixture" || reads.Load() != 1 || checks != 0 {
		t.Fatal("structured vault source hidden or read consumed mutation gate", sessions, err, reads.Load(), checks)
	}
}

func TestStructuredUsageScopeOperationsKeepCapabilities(t *testing.T) {
	var calls atomic.Int32
	var authenticated atomic.Bool
	host := remoteAgentFixture(t, "usage-scope", &calls, &authenticated)
	checks := 0
	ctx, cancel := context.WithTimeout(WithDispatchCheckpoint(context.Background(), func() error {
		checks++
		return errors.New("usage scope must not consume business authorization")
	}), 3*time.Second)
	defer cancel()
	channel, err := OpenStructuredUsageFiles(ctx, "fixture-env", "original-runtime", host)
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	group, err := channel.CreateGroup(ctx, "scope-marker")
	if err != nil || group.ID != "scope-group" {
		t.Fatal(group, err)
	}
	folder, err := channel.CreateFolder(ctx, group.ID, "scope-marker", "C:/scope")
	if err != nil || folder.ID != "scope-folder" {
		t.Fatal(folder, err)
	}
	if err := channel.DeleteFolder(ctx, folder.ID); err != nil {
		t.Fatal(err)
	}
	if !channel.SelectorRevoked(ctx, folder.ID, "log.jsonl") {
		t.Fatal("original selector revocation proof lost")
	}
	if err := channel.DeleteGroup(ctx, group.ID); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 5 || checks != 0 {
		t.Fatal("scope operation replayed or consumed business authorization", calls.Load(), checks)
	}
}

func remoteAgentCreate() AgentCreate {
	create := AgentCreate{Worktree: "id:fixture-worktree", Agent: "codex", Options: map[string]string{"permissionMode": "full-access"}, Envelope: AgentMutationEnvelope{SessionID: "oas_fixture", ClientOperationID: "1793680000000-00112233445566778899aabbccddeeff"}, FirstMessage: AgentFirstMessage{ClientMessageID: "1793680000000-112233445566778899aabbccddeeff00", Body: AgentMessage{Kind: "message", Role: "user", Blocks: []AgentTextBlock{{Type: "text", Text: "one\r\n\r\n中文😀\n```\npath\\with space\n```  \r\n"}}}, Selection: &AgentSelection{Model: "fixture-model", Effort: "medium"}}}
	create.Envelope.PayloadFingerprint, _ = AgentFingerprint("agentSession.create", create.Envelope.SessionID, map[string]any{"worktree": create.Worktree, "agent": create.Agent, "options": create.Options, "firstMessage": create.FirstMessage})
	return create
}

func TestRemoteAgentSessionWireAndDispatch(t *testing.T) {
	for _, scenario := range []string{"accepted", "commit-failed", "revoked", "lost-response", "missing-capability", "auth-refused", "wrong-runtime", "wrong-request", "wrong-host", "cancelled-read", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			var business, checks atomic.Int32
			var authenticated atomic.Bool
			host := remoteAgentFixture(t, scenario, &business, &authenticated)
			if scenario == "wrong-host" {
				host = "sha256:foreign"
			}
			client := NewRemoteAgentSessionClient("fixture-env", "original-runtime", host)
			denied := errors.New("durable dispatch failed")
			ctx, cancel := context.WithTimeout(WithDispatchCheckpoint(context.Background(), func() error {
				checks.Add(1)
				if !authenticated.Load() {
					t.Error("handshake consumed business checkpoint")
				}
				if scenario == "commit-failed" {
					return denied
				}
				return nil
			}), 3*time.Second)
			defer cancel()
			if scenario == "revoked" {
				cancel()
			}
			if scenario == "cancelled-read" || scenario == "oversized" {
				readCtx, stop := context.WithTimeout(ctx, 150*time.Millisecond)
				defer stop()
				_, err := client.History(readCtx, "oas_fixture", nil, 200)
				if err == nil || checks.Load() != 0 || business.Load() != 1 {
					t.Fatal("read limit/cancellation or mutation gate lost", err, checks.Load(), business.Load())
				}
				return
			}
			_, err := client.Create(ctx, remoteAgentCreate())
			switch scenario {
			case "accepted":
				if err != nil || checks.Load() != 1 || business.Load() != 1 {
					t.Fatal(err, checks.Load(), business.Load())
				}
				// A new connection cannot reuse this original durable intent.
				if _, err = client.Create(ctx, remoteAgentCreate()); err == nil || business.Load() != 1 || checks.Load() != 1 {
					t.Fatal("original mutation replayed", err, checks.Load(), business.Load())
				}
			case "lost-response":
				if err == nil || business.Load() != 1 || checks.Load() != 1 {
					t.Fatal("lost response retried or accepted", err, checks.Load(), business.Load())
				}
			case "commit-failed":
				if !errors.Is(err, denied) || business.Load() != 0 || checks.Load() != 1 {
					t.Fatal(err, checks.Load(), business.Load())
				}
			default:
				if err == nil || business.Load() != 0 || checks.Load() != 0 {
					t.Fatal("unverified mutation escaped", err, checks.Load(), business.Load())
				}
				if strings.HasPrefix(scenario, "wrong-") && !errors.Is(err, ErrStateIdentityChanged) {
					t.Fatal("identity conflict misclassified", err)
				}
			}
		})
	}
}

type queuedAgentFrame struct{ encoded chan struct{} }

func (f queuedAgentFrame) MarshalJSON() ([]byte, error) {
	close(f.encoded)
	return []byte(`{"method":"agentSession.create"}`), nil
}

func TestRemoteAgentDispatchRechecksAfterWriterQueue(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit-failed", true: "revoked"}[revoked], func(t *testing.T) {
			var checks atomic.Int32
			denied := errors.New("durable commit failed")
			ctx, cancel := context.WithCancel(WithDispatchCheckpoint(context.Background(), func() error { checks.Add(1); return denied }))
			defer cancel()
			socket := &stateSocket{} // Any attempted wire write would panic.
			socket.mu.Lock()
			encoded := make(chan struct{})
			done := make(chan error, 1)
			go func() { done <- socket.writeWithCheckpoint(ctx, queuedAgentFrame{encoded}, true) }()
			<-encoded
			if checks.Load() != 0 {
				t.Error("checkpoint consumed before writer acquired")
			}
			if revoked {
				cancel()
			}
			socket.mu.Unlock()
			err := <-done
			if revoked && (!errors.Is(err, context.Canceled) || checks.Load() != 0) || !revoked && (!errors.Is(err, denied) || checks.Load() != 1) {
				t.Fatal(err, checks.Load())
			}
		})
	}
}
