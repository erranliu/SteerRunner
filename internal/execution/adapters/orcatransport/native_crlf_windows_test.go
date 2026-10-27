package orcatransport

import (
	"context"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestNativeCRLFPastePreservesOriginalInputAndCompletion(t *testing.T) {
	for _, mode := range []string{"missed", "observed", "restored", "changed-input", "expanded-lines", "duplicate-input"} {
		t.Run(mode, func(t *testing.T) {
			l, host, s := fixtureOn(t, "paired-worker")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			base := fixtureState(10, "previous", "done")
			feed := &controlledStateFeed{initial: base, current: base}
			l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
			l.clock = func() time.Time { return time.Unix(100, 0).UTC() }
			input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: "原任务\r\n第一行\r\n\r\n第二行\nWrite-Output 'original'\r\n"}
			s.PendingTurn = &input
			r, _, err = l.Execute(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			host.screen = true
			l.queryNative = func(context.Context, b.ProcessIdentity, string) (native.StateSample, error) {
				return native.StateSample{AgentState: base}, nil
			}
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			t.Cleanup(l.closeStateFeed)
			// Real ConPTY paste turns CR and LF into separate newlines. Use the
			// captured actual send, not a fabricated normalized transcript.
			transcript := strings.TrimRight(strings.ReplaceAll(host.nativePrompt, "\r", "\n"), "\r\n")
			const expected = "原任务\n第一行\n\n第二行\nWrite-Output 'original'"
			if transcript != expected || host.nativeSends != 1 {
				t.Fatalf("one original paste changed logical lines: sends=%d transcript=%q", host.nativeSends, transcript)
			}
			if mode == "observed" {
				start := messageStart(input)
				start.Observation.Incarnation = 1
				feed.events = []native.AgentState{start}
				r, _, err = l.observeOnce(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
			} else if mode == "restored" {
				l.closeStateFeed()
				port, e := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
				if e != nil {
					t.Fatal(e)
				}
				l = port.(*Lifecycle)
				t.Cleanup(l.closeStateFeed)
			}
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			frozen := turnFor(&state, input.CommandID)
			if frozen.Prompt != input.Prompt || frozen.InputDigest != digest([]byte(input.Prompt)) {
				t.Fatal("wire compilation rewrote the frozen logical input")
			}
			end := fixtureState(12, "", "done")
			meta := *end.Observation
			meta.Incarnation = 1
			end.Observation, end.MetadataObservation = nil, &meta
			end.TurnStartedAt = 11500 // Same original turn after compaction.
			end.ProviderSession.TranscriptPath = "original.jsonl"
			end.LastAssistantMessage = "canonical native completion"
			if mode == "changed-input" {
				transcript = strings.Replace(transcript, "original", "changed", 1)
			}
			if mode == "expanded-lines" {
				transcript = strings.Replace(transcript, "\n", "\n\n", 1)
			}
			thinInputRPC(t, state.Process, end, transcript, mode == "duplicate-input", true)
			l.queryNative = func(ctx context.Context, p b.ProcessIdentity, pane string) (native.StateSample, error) {
				return native.QueryOriginalSample(ctx, "", p, pane)
			}
			l.queryInput = func(ctx context.Context, p b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (native.AgentState, error) {
				return native.QueryOriginalInput(ctx, "", p, pane, baseline, before)
			}
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			r, message, err := l.observeOnce(context.Background(), s)
			if err != nil || host.nativeSends != 1 {
				t.Fatal("observation failed or replayed input", err, host.nativeSends)
			}
			if mode == "changed-input" || mode == "expanded-lines" || mode == "duplicate-input" {
				if r.TurnEvidence != nil || message != nil || r.Diagnostic == "" {
					t.Fatal("changed or duplicate input authorized completion")
				}
				return
			}
			if r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage || message == nil || message.Text != end.LastAssistantMessage || r.Diagnostic != "" {
				t.Fatalf("original CRLF input did not complete: evidence=%+v diagnostic=%q message=%+v", r.TurnEvidence, r.Diagnostic, message)
			}
		})
	}
}
