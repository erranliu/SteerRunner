package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestNativePromptWireFormats(t *testing.T) {
	for _, format := range []string{"command", "model"} {
		for _, body := range []string{"中文\r\n\r\nsecond\r\n", strings.Repeat("中文🙂\n\n", 500)} {
			t.Run(format+"/"+digest([]byte(body))[:8], func(t *testing.T) {
				l, host, s, control := fixtureFixedControl(t)
				s = prepareFixedControl(t, l, s, control)
				input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: body}
				if format == "model" {
					input.Model, input.ReasoningEffort, input.StepID = "writer", "high", "implement.execute"
				}
				s.PendingTurn = &input
				r, err := l.prepareInput(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
				before, err := l.state(s)
				if err != nil {
					t.Fatal(err)
				}
				frozen := turnFor(&before, input.CommandID)
				wire, budget := wirePrompt(frozen), *frozen.WireBudget
				r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
				if err != nil || !r.State.InputAccepted || r.State.TurnStarted || host.nativeSends != 1 {
					t.Fatal(r, err)
				}
				if host.nativePrompt != strings.ReplaceAll(wire, "\n", "\r") {
					t.Fatal("original logical wire changed at submission", host.nativePrompt)
				}
				if strings.ReplaceAll(host.nativePrompt, "\r", "\n") != wire {
					t.Fatal("terminal encoding changed logical wire")
				}
				if format == "model" {
					if v, err := codex.ParseRemoteModelPrompt(strings.ReplaceAll(host.nativePrompt, "\r", "\n")); err != nil || v.Command != input.CommandID {
						t.Fatal("model integrity proof lost", v, err)
					}
				}
				bind(&s, r)
				after, err := l.state(s)
				if err != nil {
					t.Fatal(err)
				}
				got := turnFor(&after, input.CommandID)
				if got.Prompt != input.Prompt || got.InputDigest != digest([]byte(input.Prompt)) || wirePrompt(got) != wire || got.WireBudget.SHA256 != budget.SHA256 {
					t.Fatal("frozen body or logical wire accounting changed")
				}
			})
		}
	}
}

// Exercise the real lifecycle submission, with independent control over the
// provider's processing and receipt. No draft projection authorizes this fixture.
func promptFixture(t *testing.T, environment, text string) (*Lifecycle, *fakeHost, b.LifecycleSnapshot, b.TurnInput) {
	t.Helper()
	l, h, s := fixtureOn(t, environment)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: text}
	s.PendingTurn = &input
	r, err = l.prepareInput(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	t.Cleanup(l.closeStateFeed)
	return l, h, s, input
}

func TestNativePromptSubmission(t *testing.T) {
	for _, target := range []string{"local", "remote"} {
		t.Run(target, func(t *testing.T) {
			env := ""
			if target == "remote" {
				env = "paired-worker"
			}
			for _, tc := range []struct{ name, text string }{
				{"single", "Original task"}, {"multiline", "First\nSecond"}, {"chinese", "中文🙂\n原始正文"},
				{"blank-lines", "First\n\nThird\n\n"}, {"crlf", "First\r\n\r\n中文\r\n"},
				{"long", strings.Repeat("中文 fragment\n\n", 500)},
			} {
				t.Run(tc.name, func(t *testing.T) {
					l, h, s, input := promptFixture(t, env, tc.text)
					// Every projection is deliberately false and must remain unread.
					h.screen, h.draft = true, "[Pasted Content 3 chars]\nwrong projection"
					r, err := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
					if err != nil || !r.State.InputAccepted || r.State.TurnStarted || len(r.Facts) != 0 {
						t.Fatal(r, err)
					}
					if h.nativeSends != 1 || h.nativePrompt != strings.ReplaceAll(nativePromptText(input.Prompt), "\n", "\r") {
						t.Fatal("changed original bytes", h.nativeSends, h.nativePrompt)
					}
					bind(&s, r)
					st, err := l.state(s)
					if err != nil {
						t.Fatal(err)
					}
					frozen := turnFor(&st, input.CommandID)
					if frozen.Prompt != input.Prompt || frozen.InputDigest != digest([]byte(input.Prompt)) || frozen.Request != r.RequestID || !frozen.Attempted || !frozen.Accepted || frozen.Started {
						t.Fatal("changed frozen input")
					}
					sends := 0
					for _, a := range h.calls {
						if a[0] == "terminal" && a[1] == "read" {
							t.Fatal("screen dependency", a)
						}
						if a[0] != "terminal" || a[1] != "send" {
							continue
						}
						sends++
						flags := strings.Join(a, " ")
						if !strings.Contains(flags, "--enter") || !strings.Contains(flags, "--retry-request "+r.RequestID) || strings.Contains(flags, "\x1b") {
							t.Fatal("not generic original submit", a)
						}
						if strings.Contains(flags, "--wait-submit 20") != (target == "remote") {
							t.Fatal("changed existing wait policy", a)
						}
					}
					if sends != 1 {
						t.Fatal("wrong call count", sends)
					}
				})
			}
		})
	}
}

func TestNativePromptReceiptFaults(t *testing.T) {
	faults := []string{"accepted-only", "started-hint-only", "response-lost", "unsupported", "old-host", "permission", "missing-provider", "missing-observation", "incarnation-replaced", "request", "handle", "runtime", "no-stages", "start-only", "reordered-stages", "future-stage", "rejected"}
	for _, target := range []string{"local", "remote"} {
		t.Run(target, func(t *testing.T) {
			env := ""
			if target == "remote" {
				env = "paired-worker"
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					l, h, s, input := promptFixture(t, env, "Original 中文\r\n\r\nSecond")
					h.unstarted = true
					h.rejectInput = fault == "rejected"
					l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
						raw, err := h.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
						if err != nil || args[0] != "terminal" || args[1] != "send" {
							return raw, err
						}
						if fault == "response-lost" {
							return nil, errors.New("applied input; receipt lost")
						}
						var v map[string]any
						if err = json.Unmarshal(raw, &v); err != nil {
							return nil, err
						}
						send := v["result"].(map[string]any)["send"].(map[string]any)
						p := send["prompt"].(map[string]any)
						switch fault {
						case "started-hint-only":
							p["stages"] = []string{"input_accepted", "turn_started"}
						case "unsupported", "old-host":
							p["provider"] = fault
							p["observation"] = "unsupported"
						case "permission":
							p["observation"] = "permission"
						case "missing-provider":
							delete(p, "provider")
						case "missing-observation":
							delete(p, "observation")
						case "incarnation-replaced":
							p["processIncarnation"] = "replacement"
						case "request":
							p["requestId"] = "other-request"
						case "handle":
							send["handle"] = "other-terminal"
						case "runtime":
							v["_meta"].(map[string]any)["runtimeId"] = "other-runtime"
						case "no-stages":
							delete(p, "stages")
						case "start-only":
							p["stages"] = []string{"turn_started"}
						case "reordered-stages":
							p["stages"] = []string{"turn_started", "input_accepted"}
						case "future-stage":
							p["stages"] = []string{"input_accepted", "new-stage"}
						case "rejected":
							send["accepted"] = false
							delete(p, "stages")
						}
						return json.Marshal(v)
					}})
					req := request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})
					r, err := l.submit(context.Background(), req)
					if fault == "accepted-only" || fault == "started-hint-only" {
						if err != nil || !r.State.InputAccepted || r.State.TurnStarted {
							t.Fatal("acceptance became start", r, err)
						}
					} else if fault == "rejected" {
						if err == nil || r.Disposition != b.OperationRejected || r.State.InputAccepted {
							t.Fatal("rejection lost", r, err)
						}
					} else if err == nil || r.Disposition != b.OperationUnknown || r.State.InputAccepted || r.State.TurnStarted || len(r.Operation.Receipt) == 0 && fault != "response-lost" {
						t.Fatal("unverified receipt became success", r, err)
					}
					bind(&s, r)
					// Simulate restart from only persisted data. Reconcile and duplicate dispatch
					// cannot reconstruct a new attempt, even when Orca has no saved receipt.
					port, err := NewLifecycleWithStateFactory(l.setup, h.stateFactory)
					if err != nil {
						t.Fatal(err)
					}
					fresh := port.(*Lifecycle)
					t.Cleanup(fresh.closeStateFeed)
					for i := 0; i < 2; i++ {
						req.Snapshot = s
						if _, err = fresh.submit(context.Background(), req); err == nil {
							t.Fatal("duplicate input admitted")
						}
						observed, _, e := fresh.Reconcile(context.Background(), req)
						if e == nil {
							bind(&s, observed)
						}
						if observed.State.TurnStarted || observed.TurnEvidence != nil {
							t.Fatal("unstarted request completed", observed)
						}
					}
					wantEffects := 1
					if fault == "rejected" {
						wantEffects = 0
					}
					if h.nativeSends != wantEffects {
						t.Fatal("duplicate provider input", h.nativeSends)
					}
					sends := 0
					for _, a := range h.calls {
						if a[0] == "terminal" && a[1] == "send" {
							sends++
						}
						if a[0] == "orchestration" {
							t.Fatal("new recovery path", a)
						}
					}
					if sends != 1 {
						t.Fatal("extra input", sends)
					}
				})
			}
		})
	}
}

func TestNativePromptProcessingOrder(t *testing.T) {
	for _, target := range []string{"local", "remote"} {
		t.Run(target, func(t *testing.T) {
			for _, fault := range []string{"segmented", "cancel-processing", "lost-receipt-after-processing"} {
				t.Run(fault, func(t *testing.T) {
					env := ""
					if target == "remote" {
						env = "paired-worker"
					}
					l, h, s, input := promptFixture(t, env, "Original 中文\r\n\r\nSecond")
					entered, receipt := make(chan struct{}), make(chan struct{})
					chunks, consumed := make(chan []byte), make(chan int)
					processed := make(chan struct{})
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					var requestID string
					l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
						if args[0] != "terminal" || args[1] != "send" {
							return h.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
						}
						text := ""
						enter := false
						for i, a := range args {
							if a == "--text" {
								text = args[i+1]
							}
							if a == "--retry-request" {
								requestID = args[i+1]
							}
							if a == "--enter" {
								enter = true
							}
						}
						if !enter || text != strings.ReplaceAll(nativePromptText(input.Prompt), "\n", "\r") || requestID == "" {
							return nil, errors.New("not one bound generic request")
						}
						close(entered)
						var body []byte
						for {
							select {
							case fragment, ok := <-chunks:
								if !ok {
									if string(body) != text {
										return nil, errors.New("provider input is incomplete or changed")
									}
									raw, e := h.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
									close(processed)
									select {
									case <-receipt:
										return raw, e
									case <-ctx.Done():
										return nil, ctx.Err()
									}
								}
								body = append(body, fragment...)
								select {
								case consumed <- len(body):
								case <-ctx.Done():
									return nil, ctx.Err()
								}
							case <-ctx.Done():
								return nil, ctx.Err()
							}
						}
					}})
					type result struct {
						r b.LifecycleReceipt
						e error
					}
					done := make(chan result, 1)
					go func() {
						r, e := l.submit(ctx, request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
						done <- result{r, e}
					}()
					select {
					case <-entered:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					text := []byte(strings.ReplaceAll(nativePromptText(input.Prompt), "\n", "\r"))
					// Deliberately split a UTF-8 scalar and a newline boundary; the controlled
					// provider must assemble all bytes before applying the combined submission.
					cuts := []int{10, 12, len(text)}
					previous := 0
					for _, cut := range cuts {
						select {
						case chunks <- text[previous:cut]:
						case <-ctx.Done():
							t.Fatal(ctx.Err())
						}
						select {
						case n := <-consumed:
							if n != cut {
								t.Fatal("segment order changed", n, cut)
							}
						case <-ctx.Done():
							t.Fatal(ctx.Err())
						}
						previous = cut
						if h.nativeSends != 0 {
							t.Fatal("submitted partial input")
						}
						select {
						case v := <-done:
							t.Fatal("returned before body processing", v.e)
						default:
						}
						if fault == "cancel-processing" {
							break
						}
					}
					if fault == "cancel-processing" {
						cancel()
					} else {
						close(chunks)
						select {
						case <-processed:
						case <-ctx.Done():
							t.Fatal(ctx.Err())
						}
						select {
						case v := <-done:
							t.Fatal("returned before receipt", v.e)
						default:
						}
						if fault == "lost-receipt-after-processing" {
							cancel()
						} else {
							close(receipt)
						}
					}
					var v result
					select {
					case v = <-done:
					case <-time.After(5 * time.Second):
						t.Fatal("canceled request did not settle")
					}
					want := 1
					if fault == "cancel-processing" {
						want = 0
					}
					if h.nativeSends != want {
						t.Fatal("wrong effect count", h.nativeSends, want)
					}
					if fault == "segmented" {
						if v.e != nil || !v.r.State.InputAccepted || h.nativePrompt != string(text) {
							t.Fatal(v.r, v.e)
						}
					} else if v.e == nil || v.r.Disposition != b.OperationUnknown || v.r.State.InputAccepted {
						t.Fatal("uncertain processing became accepted", v.r, v.e)
					}
					if v.r.RequestID != requestID {
						t.Fatal("request identity changed")
					}
					bind(&s, v.r)
					before := h.nativeSends
					if _, e := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})); e == nil || h.nativeSends != before {
						t.Fatal("replayed after processing uncertainty", e)
					}
				})
			}
		})
	}
}

func TestNativePromptControlCharactersRejected(t *testing.T) {
	for _, control := range []string{"\x00", "\x1b"} {
		t.Run(digest([]byte(control))[:8], func(t *testing.T) {
			l, h, s, input := promptFixture(t, "paired-worker", "First\nSecond"+control)
			r, err := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err == nil || r.State.InputAccepted || h.nativeSends != 0 {
				t.Fatal("unsafe input sent", r, err)
			}
			l.feedMu.Lock()
			open := l.feed != nil
			l.feedMu.Unlock()
			if open {
				t.Fatal("rejected input retained its observation feed")
			}
		})
	}
}
