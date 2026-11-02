package orca

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrcaSessionReadCapability(t *testing.T) {
	for _, scenario := range []string{"multiple-users", "paged", "shortened-foreign", "missing-messages", "missing-more", "negative-offset", "stuck-offset", "budget", "runtime", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			var reads, handshakes atomic.Int32
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method == "status.get" {
					handshakes.Add(1)
					ipcReply(conn, req, "runtime", map[string]any{})
					return
				}
				if req.Method != "nativeChat.readSession" {
					t.Errorf("unexpected method/mutation %s", req.Method)
					return
				}
				n := reads.Add(1)
				var params struct {
					Agent, SessionID, TranscriptPath string
					Limit                            int
					BeforeOffset                     *int64
				}
				if json.Unmarshal(req.Params, &params) != nil || params.Agent != "codex" || params.SessionID != "provider" || params.TranscriptPath != "original.jsonl" || params.Limit != 100 || (params.BeforeOffset == nil) != (n == 1) {
					t.Error("session target, page size or original offset changed", string(req.Params))
				}
				if scenario == "cancelled" {
					return // leave the single read pending until its original deadline
				}
				first := map[string]any{"id": "first", "role": "user", "source": "transcript", "timestamp": 12, "blocks": []any{map[string]string{"type": "text", "text": "identical 中文\ntext"}}}
				second := map[string]any{"id": "second", "role": "user", "source": "transcript", "timestamp": 13, "blocks": []any{map[string]string{"type": "text", "text": "identical 中文\ntext"}}}
				value := map[string]any{"messages": []any{first, second, map[string]any{"role": "assistant", "blocks": []any{}}}, "hasMore": false, "beforeOffset": 0}
				switch scenario {
				case "paged", "stuck-offset", "budget":
					if n == 1 || scenario != "paged" {
						value["hasMore"], value["beforeOffset"] = true, 100
					}
					if scenario == "budget" && n > 1 {
						value["beforeOffset"] = 90
					}
					if n > 1 && (params.BeforeOffset == nil || *params.BeforeOffset != 100) {
						t.Error("reader did not reuse original cursor")
					}
				case "shortened-foreign":
					first["shortened"], first["source"], first["blocks"] = true, "hook", []any{}
				case "missing-messages":
					delete(value, "messages")
				case "missing-more":
					delete(value, "hasMore")
				case "negative-offset":
					value["beforeOffset"] = -1
				}
				runtime := "runtime"
				if scenario == "runtime" {
					runtime = "foreign"
				}
				ipcReply(conn, req, runtime, value)
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if scenario == "cancelled" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			reader, err := OpenSessionReader(ctx, "", "runtime", SessionReadOptions{Agent: "codex", SessionID: "provider", TranscriptPath: "original.jsonl", Limit: 100, MaxPages: 2})
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			page, err := reader.Next()
			valid := scenario == "multiple-users" || scenario == "shortened-foreign" || scenario == "paged" || scenario == "budget" || scenario == "stuck-offset"
			if (err == nil) != valid || handshakes.Load() != 1 || reads.Load() != 1 {
				t.Fatal("invalid frame accepted or target reconnected", page, err, handshakes.Load(), reads.Load())
			}
			if !valid {
				if scenario == "runtime" && !errors.Is(err, ErrStateIdentityChanged) || scenario == "cancelled" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("identity/cancellation classification lost", err)
				}
				if _, again := reader.Next(); again != err || reads.Load() != 1 {
					t.Fatal("failed read issued another RPC", again, reads.Load())
				}
				return
			}
			if len(page.Messages) != 3 || page.Messages[0].ID != "first" || page.Messages[1].ID != "second" || page.Messages[2].Role != "assistant" || len(page.Raw) == 0 {
				t.Fatal("raw session filtered user rows or lost page facts", page)
			}
			if scenario == "shortened-foreign" {
				if !page.Messages[0].Shortened || page.Messages[0].Source != "hook" || len(page.Messages[0].Blocks) != 0 {
					t.Fatal("raw reader imposed unique-input policy", page)
				}
			} else if page.Messages[0].Blocks[0].Text != page.Messages[1].Blocks[0].Text || page.Messages[0].Blocks[0].Text != "identical 中文\ntext" {
				t.Fatal("duplicate text was normalized or deduplicated", page)
			}
			if scenario == "paged" || scenario == "budget" || scenario == "stuck-offset" {
				_, err = reader.Next()
				if (err != nil) != (scenario == "stuck-offset") || reads.Load() != 2 {
					t.Fatal("original pagination boundary changed", err, reads.Load())
				}
			}
			if scenario == "budget" {
				if _, err := reader.Next(); !errors.Is(err, ErrSessionReadBudget) || reads.Load() != 2 {
					t.Fatal("page budget was reset or exceeded", err, reads.Load())
				}
			} else if scenario != "stuck-offset" {
				if _, err := reader.Next(); err != io.EOF {
					t.Fatal("finished page reopened a session", err)
				}
			}
			reader.Close()
			reader.Close()
			if handshakes.Load() != 1 {
				t.Fatal("reader reconnected", handshakes.Load())
			}
		})
	}
	t.Run("invalid-identity-zero-connect", func(t *testing.T) {
		var calls atomic.Int32
		localIPCFixture(t, func(net.Conn, ipcTestRequest) { calls.Add(1) })
		for _, options := range []SessionReadOptions{{}, {Agent: "codex", SessionID: "provider", TranscriptPath: "original.jsonl", Limit: 100}, {Agent: "codex", SessionID: "provider", Limit: 100, MaxPages: 16}} {
			reader, err := OpenSessionReader(context.Background(), "", "runtime", options)
			if err == nil || reader != nil || calls.Load() != 0 {
				t.Fatal("invalid session selected a target or connected", reader, err, calls.Load())
			}
		}
	})
	for _, mode := range []string{"explicit-close", "parent-cancel"} {
		t.Run(mode, func(t *testing.T) {
			closed := make(chan error, 1)
			var reads atomic.Int32
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method == "status.get" {
					ipcReply(conn, req, "runtime", map[string]any{})
					return
				}
				if req.Method != "nativeChat.readSession" {
					t.Errorf("unexpected method %s", req.Method)
					return
				}
				reads.Add(1)
				ipcReply(conn, req, "runtime", map[string]any{"messages": []any{}, "hasMore": false, "beforeOffset": 0})
				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, err := conn.Read(make([]byte, 1))
				closed <- err // the client must close the actual original pipe
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader, err := OpenSessionReader(ctx, "", "runtime", SessionReadOptions{Agent: "codex", SessionID: "provider", TranscriptPath: "original.jsonl", Limit: 100, MaxPages: 16})
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			if _, err := reader.Next(); err != nil {
				t.Fatal(err)
			}
			if mode == "explicit-close" {
				reader.Close()
			} else {
				cancel()
			}
			select {
			case err := <-closed:
				var timeout net.Error
				if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
					t.Fatal("reader did not close its original connection", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("reader original pipe remained open")
			}
			reader.Close()
			if _, err := reader.Next(); !errors.Is(err, context.Canceled) || reads.Load() != 1 {
				t.Fatal("closed reader reopened or lost cancellation", err, reads.Load())
			}
		})
	}
}
