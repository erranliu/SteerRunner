package orcatransport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaCompletionQueryRecoversEmptyPromptAfterDisconnect(t *testing.T) {
	for _, mode := range []string{"reconnect", "restored", "missing-body"} {
		t.Run(mode, func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			start := messageStart(input)
			feed.events = []native.AgentState{start}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			// Persist the original start before losing all notification coverage.
			l.feed = gappedMessageFeed{feed}
			feed.err = errors.New("remote runtime disconnected")
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.TurnEvidence != nil || r.Diagnostic == "" {
				t.Fatal("disconnection did not remain unverified", r, err)
			}
			bind(&s, r)
			l.closeStateFeed()
			if mode == "restored" {
				port, err := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
				if err != nil {
					t.Fatal(err)
				}
				l = port.(*Lifecycle)
				t.Cleanup(l.closeStateFeed)
			}
			end := fixtureState(12, "", "done")
			end.TurnStartedAt = start.MainAgent.StateStartedAt
			end.Observation = nil // The independent hook-store query has no stream revision.
			end.LastAssistantMessage = "Complete original result after reconnect"
			feed.initial, feed.current, feed.err = end, end, nil
			l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
			if mode == "missing-body" {
				feed.current.LastAssistantMessage = ""
				r, _, err = l.observeOnce(context.Background(), s)
				if err != nil || r.TurnEvidence != nil {
					t.Fatal("missing body completed the turn", r, err)
				}
				bind(&s, r)
				feed.current = end
				l.clock = func() time.Time { return time.Unix(193, 0).UTC() }
			}
			before := len(host.calls)
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic != "" || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage || r.TurnEvidence.NativeOutcome != "unreported" || host.nativeSends != 1 {
				t.Fatal("retained original turn did not recover", r.Diagnostic, r.TurnEvidence, err)
			}
			bind(&s, r)
			replay, _, err := l.observeOnce(context.Background(), s)
			if err != nil || replay.TurnEvidence == nil || replay.TurnEvidence.LastMessage != end.LastAssistantMessage {
				t.Fatal("recovered final was not durable", replay, err)
			}
			for _, call := range host.calls[before:] {
				if len(call) >= 2 && call[0] == "terminal" && (call[1] == "read" || call[1] == "send") {
					t.Fatal("recovery read the screen or resent input", call)
				}
			}
		})
	}
}

func TestOrcaCompletionQueryRetainedStartRejectsForeignOrMissingEvidence(t *testing.T) {
	for _, fault := range []string{"not-accepted", "not-started", "no-start", "no-root", "baseline-root", "no-authority", "no-marker", "other-marker", "pane", "provider", "agent", "prompt", "boundary", "restored", "interrupted", "tool-output", "outcome", "stale", "authority", "incarnation", "old-revision", "conflict", "truncated"} {
		t.Run(fault, func(t *testing.T) {
			now := time.Unix(100, 0).UTC()
			turn := Turn{Accepted: true, Started: true, Prompt: "original"}
			turn.Native = nativeBaseline(fixtureState(10, "previous", "done"), now)
			turn.Native.signal(messageStart(b.TurnInput{Prompt: turn.Prompt}), &turn, now, false)
			end := fixtureState(12, "", "done")
			end.TurnStartedAt = 11000
			end.LastAssistantMessage = "foreign or unverifiable result"
			switch fault {
			case "not-accepted":
				turn.Accepted = false
			case "not-started":
				turn.Started = false
			case "no-start":
				turn.Native.WorkingAt = 0
			case "no-root":
				turn.Native.RootWorkingAt = 0
			case "baseline-root":
				turn.Native.RootWorkingAt = turn.Native.BaseRootAt
				end.TurnStartedAt = turn.Native.BaseRootAt
			case "no-authority":
				turn.Native.Authority = ""
			case "no-marker":
				end.TurnStartedAt = 0
			case "other-marker":
				end.TurnStartedAt++
			case "pane":
				end.PaneKey = "other:pane"
			case "provider":
				end.ProviderSession.ID = "other"
			case "agent":
				end.AgentType = "pi"
			case "prompt":
				end.Prompt = "other input"
			case "boundary":
				end.SessionBoundary = true
			case "restored":
				end.RestoredUnconfirmed = true
			case "interrupted":
				end.Interrupted = true
			case "tool-output":
				end.ToolOutput = true
			case "outcome":
				end.MainAgent.Outcome = "failure"
			case "stale":
				end.UpdatedAt = end.MainAgent.StateStartedAt - 1
			case "authority":
				end.Observation.AuthorityID = "other"
			case "incarnation":
				end.Observation.Incarnation++
			case "old-revision":
				end.Observation.Revision = turn.Native.BaseRevision
			case "conflict":
				turn.Native.Conflict = "historical provenance conflict"
			case "truncated":
				end.LastAssistantMessage = strings.Repeat("x", 8000)
			}
			turn.Native.query(end, &turn, now.Add(time.Minute), false)
			if turn.Native.hasFinal() {
				t.Fatal("unverified result became a final", turn.Native)
			}
		})
	}
}

func TestOrcaCompletionQueryReconnectObserverCommitsFinal(t *testing.T) {
	l, host, feed, s, input := publicMessageFixture(t)
	feed.events = []native.AgentState{messageStart(input)}
	r, _, err := l.observeOnce(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	l.closeStateFeed()
	l.clearObservation(l.nativeObservation)
	end := fixtureState(12, "", "done")
	end.TurnStartedAt = 11000
	end.Observation = nil
	end.LastAssistantMessage = "Original final collected by the reattached observer"
	feed.initial, feed.current = end, end
	l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
	obs := l.observe(s, nil)
	defer obs.Close(context.Background())
	completed, cleared := 0, false
	for {
		select {
		case event, ok := <-obs.Events():
			if !ok {
				if completed != 1 || !cleared || host.nativeSends != 1 {
					t.Fatal("recovery lost completion or diagnostic clearance", completed, cleared, host.nativeSends)
				}
				return
			}
			if event.Observation != nil {
				cleared = event.Observation.Diagnostic == ""
			}
			if event.Fact.Kind == b.EventCompleted {
				completed++
				if !event.Fact.Completed || event.Fact.StartedTurnID != input.CommandID || event.Fact.LastMessage != end.LastAssistantMessage || len(event.Implementation) == 0 {
					t.Fatal("recovered final lost its original identity or proof", event.Fact)
				}
			}
			event.Persisted <- nil
			event.Processed <- nil
		case <-time.After(2 * time.Second):
			t.Fatal("reattached observer did not finish")
		}
	}
}
