package orcatransport

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func thinInputRPC(t *testing.T, p b.ProcessIdentity, end native.AgentState, prompt string, duplicate bool, fullMetadata ...bool) *atomic.Int32 {
	return thinInputRPCAt(t, p, end, prompt, duplicate, 11001, fullMetadata...)
}

func thinInputRPCAt(t *testing.T, p b.ProcessIdentity, end native.AgentState, prompt string, duplicate bool, timestamp int64, fullMetadata ...bool) *atomic.Int32 {
	t.Helper()
	_, workspace, ok := strings.Cut(p.WorktreeID, "::")
	if !ok || workspace == "" {
		t.Fatal("missing original fixture workspace")
	}
	endpoint := `\\.\pipe\sr-thin-input-` + digest([]byte(t.TempDir()))[:24]
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	stop := make(chan struct{})
	var inputQueries atomic.Int32
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			select {
			case <-stop:
				_ = conn.Close()
				return
			default:
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
						result = map[string]any{"terminal": map[string]any{"handle": p.Terminal, "incarnationId": p.Incarnation, "worktreeId": p.WorktreeID, "worktreePath": workspace, "branch": "worker", "connected": true, "agentIdentity": "codex"}}
					case "nativeChat.readSession":
						inputQueries.Add(1)
						user := func(id string) any {
							return map[string]any{"id": id, "role": "user", "source": "transcript", "timestamp": timestamp, "blocks": []any{map[string]string{"type": "text", "text": prompt}}}
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
	t.Cleanup(func() {
		// Stop Accept before closing winio's listener. Repeated tests exposed
		// an intermittent listener.Close/Accept shutdown hang on Windows.
		close(stop)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		wake, err := winio.DialPipeContext(ctx, endpoint)
		if err != nil {
			t.Errorf("wake test RPC accept: %v", err)
			return
		}
		// Keep the wake client open until Accept returns; an immediate close
		// may be retried internally as ERROR_NO_DATA instead of waking it.
		defer wake.Close()
		select {
		case <-done:
			_ = listener.Close()
		case <-ctx.Done():
			t.Error("test RPC accept did not stop", ctx.Err())
		}
	})
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	data, _ := json.Marshal(map[string]any{"runtimeId": p.RuntimeID, "authToken": "test-secret", "transport": map[string]string{"kind": "named-pipe", "endpoint": endpoint}})
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return &inputQueries
}

// The unmanaged local native path is still supported for non-GitHub repos.
// Exercise the production constructor without injecting queryInput or Started.
func TestLocalNativeProbeOriginalQueryConstructor(t *testing.T) {
	for _, mode := range []string{"thin", "corroborated", "duplicate", "altered"} {
		t.Run(mode, func(t *testing.T) {
			l, host, s := fixture(t)
			l.setup.RepositoryURL = "https://gitea.example/owner/repo.git"
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			base := fixtureState(10, "previous", "done")
			factory := func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
				return &controlledStateFeed{initial: base, current: base}, nil
			}
			l.stateFactory = factory
			l.clock = func() time.Time { return time.Unix(100, 0).UTC() }
			input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: strings.Repeat("本机原文\r\n\r\n", 40)}
			s.PendingTurn = &input
			r, err = l.prepareInput(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil || !r.State.InputAccepted || r.State.TurnStarted {
				t.Fatal("acceptance became business start", r, err)
			}
			bind(&s, r)
			l.closeStateFeed()
			state, err := l.state(s)
			if err != nil || state.Turns[0].WireFormat != "" || state.RemoteReadiness != nil {
				t.Fatal("not the ordinary local native path", state, err)
			}
			end := fixtureState(12, "", "done")
			end.Observation, end.TurnStartedAt = nil, 0
			end.ProviderSession.TranscriptPath = "original.jsonl"
			end.LastAssistantMessage = "canonical local native completion"
			if mode == "corroborated" {
				meta := *fixtureState(12, "", "done").Observation
				meta.Incarnation = 1
				end.MetadataObservation, end.TurnStartedAt = &meta, 11000
			}
			text := nativePromptText(input.Prompt)
			if mode == "altered" {
				text += " foreign input"
			}
			queries := thinInputRPC(t, state.Process, end, text, mode == "duplicate", mode == "corroborated")
			t.Setenv("LOCALAPPDATA", t.TempDir())
			setup := l.setup.Clone()
			setup.Snapshot = s
			port, err := NewLifecycle(setup)
			if err != nil {
				t.Fatal(err)
			}
			l = port.(*Lifecycle)
			l.stateFactory = factory
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			t.Cleanup(l.closeStateFeed)
			r, message, err := l.observeOnce(context.Background(), s)
			if err != nil || host.nativeSends != 1 {
				t.Fatal("query failed or repeated business input", err, host.nativeSends)
			}
			if queries.Load() != 1 {
				t.Fatalf("original input query calls=%d; diagnostic=%q", queries.Load(), r.Diagnostic)
			}
			positive := mode == "thin" || mode == "corroborated"
			if !positive {
				if r.State.TurnStarted || r.TurnEvidence != nil || message != nil || r.Diagnostic == "" {
					t.Fatal("foreign input claimed", r, message)
				}
				return
			}
			// Local native finals use TurnEvidence; the remote public-candidate
			// channel remains disabled for local targets.
			if !r.State.TurnStarted || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage || message != nil {
				t.Fatalf("accepted local native input cannot complete through constructor: started=%t final=%t diagnostic=%q", r.State.TurnStarted, r.TurnEvidence != nil, r.Diagnostic)
			}
			bind(&s, r)
			state, err = l.state(s)
			if err != nil || state.Turns[0].Native.WorkingAt != 0 || state.Turns[0].Recorded != nil {
				t.Fatal("fabricated streamed start or helper fallback", state, err)
			}
		})
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
