package orcatransport

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func thinInputRPC(t *testing.T, p b.ProcessIdentity, end native.AgentState, prompt string, duplicate bool, fullMetadata ...bool) {
	t.Helper()
	endpoint := `\\.\pipe\sr-thin-input-` + filepath.Base(t.TempDir())
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func(conn net.Conn) {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var req struct{ ID, Method string }
					_ = json.Unmarshal(scanner.Bytes(), &req)
					var result any
					switch req.Method {
					case "status.get":
						result = map[string]any{}
					case "terminal.show":
						result = map[string]any{"terminal": map[string]any{"handle": p.Terminal, "incarnationId": p.Incarnation, "worktreeId": p.WorktreeID, "worktreePath": "original", "branch": "worker", "connected": true, "agentIdentity": "codex"}}
					case "nativeChat.readSession":
						user := func(id string) any {
							return map[string]any{"id": id, "role": "user", "source": "transcript", "timestamp": 11001, "blocks": []any{map[string]string{"type": "text", "text": prompt}}}
						}
						messages := []any{user("original"), map[string]any{"role": "assistant", "text": "NOT THE NATIVE FINAL"}}
						if duplicate {
							messages = append(messages, user("duplicate"))
						}
						result = map[string]any{"messages": messages, "hasMore": false, "beforeOffset": 0}
					case "worktree.ps":
						result = map[string]any{"worktrees": []any{map[string]any{"worktreeId": p.WorktreeID, "agents": []any{end}}}, "totalCount": 1, "truncated": false}
					case "session.tabs.list":
						metadata := map[string]any{"state": "done", "agentType": "codex", "providerSession": end.ProviderSession}
						if len(fullMetadata) > 0 && fullMetadata[0] {
							metadata["mainAgent"], metadata["observation"], metadata["turnStartedAt"] = end.MainAgent, end.MetadataObservation, end.TurnStartedAt
						}
						result = map[string]any{"worktree": p.WorktreeID, "tabs": []any{map[string]any{"terminal": p.Terminal, "incarnationId": p.Incarnation, "parentTabId": "tab", "leafId": "pane", "agentStatus": metadata}}}
					default:
						t.Errorf("unexpected RPC %s", req.Method)
					}
					data, _ := json.Marshal(map[string]any{"id": req.ID, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": p.RuntimeID}})
					_, _ = conn.Write(append(data, '\n'))
				}
			}(conn)
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	data, _ := json.Marshal(map[string]any{"runtimeId": p.RuntimeID, "authToken": "test-secret", "transport": map[string]string{"kind": "named-pipe", "endpoint": endpoint}})
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCorroboratedOriginalQueryCompletesThroughNativeLifecycle(t *testing.T) {
	for _, mode := range []string{"missed", "missed-restored", "duplicate", "authority-conflict", "stale-metadata"} {
		t.Run(mode, func(t *testing.T) {
			l, host, _, s, input := publicMessageFixture(t)
			if mode == "missed-restored" {
				l.closeStateFeed()
				port, err := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
				if err != nil {
					t.Fatal(err)
				}
				l = port.(*Lifecycle)
				t.Cleanup(l.closeStateFeed)
			}
			state, _ := l.state(s)
			end := fixtureState(12, "", "done")
			meta := *end.Observation
			meta.Incarnation = 1
			if mode == "authority-conflict" {
				meta.AuthorityID = "replacement"
			}
			if mode == "stale-metadata" {
				meta.Revision = 1
			}
			end.Observation = nil
			end.MetadataObservation = &meta
			end.TurnStartedAt = 11000
			end.ProviderSession.TranscriptPath = "original.jsonl"
			end.LastAssistantMessage = "canonical full-metadata completion"
			thinInputRPC(t, state.Process, end, input.Prompt, mode == "duplicate", true)
			l.queryNative = func(ctx context.Context, p b.ProcessIdentity, pane string) (native.StateSample, error) {
				return native.QueryOriginalSample(ctx, "", p, pane)
			}
			l.queryInput = func(ctx context.Context, p b.ProcessIdentity, pane string, base int64, before native.AgentState) (native.AgentState, error) {
				return native.QueryOriginalInput(ctx, "", p, pane, base, before)
			}
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			r, message, err := l.observeOnce(context.Background(), s)
			if err != nil || host.nativeSends != 1 {
				t.Fatal("query failed or resent original input", err, host.nativeSends)
			}
			positive := mode == "missed" || mode == "missed-restored"
			if !positive {
				if r.TurnEvidence != nil || message != nil || r.Diagnostic == "" {
					t.Fatal("unverified full metadata authorized completion", mode, r, message)
				}
				return
			}
			if r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage || r.TurnEvidence.NativeOutcome != "unreported" || message == nil || message.Text != end.LastAssistantMessage || r.Diagnostic != "" {
				t.Fatal("corroborated completion did not pass original lifecycle", r, message)
			}
			bind(&s, r)
			settled, err := l.state(s)
			if err != nil || turnFor(&settled, input.CommandID).Native.WorkingAt != 0 {
				t.Fatal("query fabricated streamed working evidence", err)
			}
		})
	}
}

func TestThinOriginalQueryCompletesThroughNativeLifecycle(t *testing.T) {
	for _, start := range []string{"observed", "missed", "missed-restored"} {
		t.Run(start, func(t *testing.T) {
			for _, duplicate := range []bool{false, true} {
				t.Run(map[bool]string{false: "original", true: "duplicate-rejected"}[duplicate], func(t *testing.T) {
					l, host, feed, s, input := publicMessageFixture(t)
					if start == "observed" {
						feed.events = []native.AgentState{messageStart(input)}
						r, _, err := l.observeOnce(context.Background(), s)
						if err != nil {
							t.Fatal(err)
						}
						bind(&s, r)
					} else if start == "missed-restored" {
						l.closeStateFeed()
						port, err := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
						if err != nil {
							t.Fatal(err)
						}
						l = port.(*Lifecycle)
						t.Cleanup(l.closeStateFeed)
					}
					state, _ := l.state(s)
					end := fixtureState(12, "", "done")
					end.TurnStartedAt, end.Observation = 0, nil
					end.ProviderSession.TranscriptPath = "original.jsonl"
					end.LastAssistantMessage = "canonical native completion"
					thinInputRPC(t, state.Process, end, input.Prompt, duplicate)
					l.queryNative = func(ctx context.Context, process b.ProcessIdentity, pane string) (native.StateSample, error) {
						return native.QueryOriginalSample(ctx, "", process, pane)
					}
					l.queryInput = func(ctx context.Context, process b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (native.AgentState, error) {
						return native.QueryOriginalInput(ctx, "", process, pane, baseline, before)
					}
					l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
					r, message, err := l.observeOnce(context.Background(), s)
					if err != nil || host.nativeSends != 1 {
						t.Fatal("observation resent input or failed", host.nativeSends, err)
					}
					if duplicate {
						if r.TurnEvidence != nil || message != nil || r.Diagnostic == "" {
							t.Fatal("duplicate input produced a final", r, message)
						}
						return
					}
					if r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage || r.Diagnostic != "" || message == nil || message.Text != end.LastAssistantMessage {
						t.Fatal("canonical result/message did not pass the full native chain", r, message)
					}
				})
			}
		})
	}
}
