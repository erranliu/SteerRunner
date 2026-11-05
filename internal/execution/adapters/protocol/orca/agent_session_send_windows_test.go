package orca

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalAgentCreateSupportKeepsAgentCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name, agent string
		missing     bool
	}{
		{name: "codex", agent: "codex"},
		{name: "pi", agent: "pi"},
		{name: "missing-atomic-create", agent: "codex", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests, checks atomic.Int32
			ctx, cancel := context.WithTimeout(WithDispatchCheckpoint(context.Background(), func() error {
				checks.Add(1)
				return nil
			}), 3*time.Second)
			defer cancel()
			client := NewLocalAgentSessionClient("runtime")
			want := agentSessionCapabilities()
			if tc.agent == "pi" {
				client = NewLocalPiAgentSessionClient("runtime")
				want = append(want, "agent-session.structured.client-launch-mode.v1", "agent-session.structured.registered-agents.v1")
			}
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method == "status.get" {
					caps := agentSessionCapabilities()
					if tc.missing {
						caps = slices.DeleteFunc(caps, func(c string) bool { return c == "agentSession.create.message.v1" })
					}
					ipcReply(conn, req, "runtime", map[string]any{"capabilities": caps})
					return
				}
				requests.Add(1)
				if req.Method != "agentSession.createSupport" || !slices.Equal(req.ClientCapabilities, want) {
					t.Error("local create probe lost the original client capability set")
				}
				ipcReply(conn, req, "runtime", map[string]bool{"supported": true})
			})
			err := client.SupportsCreate(ctx, "id:fixture-worktree", tc.agent)
			wantRequests := int32(1)
			if tc.missing {
				wantRequests = 0
			}
			if (err != nil) != tc.missing || requests.Load() != wantRequests || checks.Load() != 0 {
				t.Fatal(err, requests.Load(), checks.Load())
			}
		})
	}
}

func TestLocalAgentSendWireAndDispatch(t *testing.T) {
	for _, scenario := range []string{"accepted", "commit-failed", "revoked-after-status", "lost-response", "missing-send-capability", "wrong-runtime"} {
		t.Run(scenario, func(t *testing.T) {
			var business, checks atomic.Int32
			denied := errors.New("durable send commit failed")
			ctx, cancel := context.WithTimeout(WithDispatchCheckpoint(context.Background(), func() error {
				checks.Add(1)
				if scenario == "commit-failed" {
					return denied
				}
				return nil
			}), 3*time.Second)
			defer cancel()
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method == "status.get" {
					caps := agentSessionCapabilities()
					if scenario == "missing-send-capability" {
						caps = slices.DeleteFunc(caps, func(c string) bool { return c == "agent-session.send-selection.v1" })
					}
					runtime := "runtime"
					if scenario == "wrong-runtime" {
						runtime = "replacement"
					}
					if scenario == "revoked-after-status" {
						cancel()
					}
					ipcReply(conn, req, runtime, map[string]any{"capabilities": caps})
					return
				}
				business.Add(1)
				want := originalAgentSend()
				var got AgentSend
				var shape map[string]json.RawMessage
				if req.Method != "agentSession.send" || json.Unmarshal(req.Params, &got) != nil || json.Unmarshal(req.Params, &shape) != nil || len(shape) != 3 || got.Envelope.ExpectedRuntimeFence == nil || *got.Envelope.ExpectedRuntimeFence != 3 || got.Envelope.SessionID != want.Envelope.SessionID || got.Envelope.ClientOperationID != want.Envelope.ClientOperationID || got.Envelope.PayloadFingerprint != want.Envelope.PayloadFingerprint || len(got.Body.Blocks) != 1 || got.Body.Blocks[0].Text != want.Body.Blocks[0].Text || got.Selection == nil || *got.Selection != *want.Selection {
					t.Error("local original send identity/body/selection changed")
					_ = conn.Close()
					return
				}
				if scenario == "lost-response" {
					_ = conn.Close()
					return
				}
				ipcReply(conn, req, "runtime", map[string]bool{"ok": true})
			})
			client := NewLocalAgentSessionClient("runtime")
			_, err := client.Send(ctx, originalAgentSend())
			switch scenario {
			case "accepted", "lost-response":
				if (err != nil) != (scenario == "lost-response") || business.Load() != 1 || checks.Load() != 1 {
					t.Fatal(err, business.Load(), checks.Load())
				}
				if _, err := client.Send(ctx, originalAgentSend()); err == nil || business.Load() != 1 || checks.Load() != 1 {
					t.Fatal("local original send replayed", err, business.Load(), checks.Load())
				}
			case "commit-failed":
				if !errors.Is(err, denied) || business.Load() != 0 || checks.Load() != 1 {
					t.Fatal(err, business.Load(), checks.Load())
				}
			default:
				if err == nil || business.Load() != 0 || checks.Load() != 0 {
					t.Fatal("unverified local send escaped", err, business.Load(), checks.Load())
				}
			}
		})
	}
}
