package orcatransport

import (
	"context"
	"fmt"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func recordedInput(input b.TurnInput, s native.AgentState) *native.NativeInput {
	return &native.NativeInput{ID: "original-user-row", Text: input.Prompt, Timestamp: 11001, Baseline: 10000, Observation: s.Observation}
}

func TestOrcaNativeInputQueryRecoversCompactedWorkingAndFinal(t *testing.T) {
	for _, restored := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-observer", true: "restored"}[restored], func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(input)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			if restored {
				l.closeStateFeed()
				port, err := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
				if err != nil {
					t.Fatal(err)
				}
				l = port.(*Lifecycle)
				t.Cleanup(l.closeStateFeed)
			}
			progress := messageProgress(input, 12, "")
			progress.Prompt, progress.TurnStartedAt = "", 11500 // SessionStart compact; same main working root.
			feed.initial, feed.current = progress, progress
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			queries := 0
			l.queryInput = func(_ context.Context, _ b.ProcessIdentity, _ string, baseline int64, current native.AgentState) (native.AgentState, error) {
				queries++
				if baseline != 10000 {
					t.Fatal("wrong original baseline", baseline)
				}
				current.NativeInput = recordedInput(input, current)
				return current, nil
			}
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic != "" || r.TurnEvidence != nil || queries != 1 {
				t.Fatal("working did not recover", r.Diagnostic, err, queries)
			}
			bind(&s, r)
			for rev := 13; rev <= 14; rev++ {
				progress = messageProgress(input, rev, "new progress")
				progress.Prompt, progress.TurnStartedAt = "", 11500
				feed.events = []native.AgentState{progress}
				r, m, err := l.observeOnce(context.Background(), s)
				if err != nil || r.Diagnostic != "" || r.TurnEvidence != nil || queries != 1 {
					t.Fatal("same original working lost input attribution", r.Diagnostic, err, queries)
				}
				if rev == 13 && (m == nil || m.Text != "new progress") {
					t.Fatal("progress lost", m)
				}
				bind(&s, r)
				if m != nil {
					l.acknowledgeMessage(m)
				}
			}
			end := fixtureState(15, "", "done")
			end.TurnStartedAt = 11500
			end.LastAssistantMessage = "original full result"
			feed.current = end
			feed.events = []native.AgentState{end}
			l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic != "" || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage || queries != 2 || host.nativeSends != 1 {
				t.Fatal("original final did not recover", r.Diagnostic, r.TurnEvidence, err, queries, host.nativeSends)
			}
		})
	}
}

func TestOrcaNativeInputCachedBindingCannotCrossBoundary(t *testing.T) {
	for _, fault := range []string{"marker", "provider", "pane", "authority", "incarnation", "boundary", "interrupted", "tool-output", "root", "gap"} {
		t.Run(fault, func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(input)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			progress := messageProgress(input, 12, "")
			progress.Prompt, progress.TurnStartedAt = "", 11500
			feed.current = progress
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			l.queryInput = func(_ context.Context, _ b.ProcessIdentity, _ string, _ int64, current native.AgentState) (native.AgentState, error) {
				current.NativeInput = recordedInput(input, current)
				return current, nil
			}
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic != "" {
				t.Fatal(r.Diagnostic, err)
			}
			bind(&s, r)
			bad := messageProgress(input, 13, "unverified progress")
			bad.Prompt, bad.TurnStartedAt = "", 11500
			switch fault {
			case "marker":
				bad.TurnStartedAt++
			case "provider":
				bad.ProviderSession.ID = "foreign"
			case "pane":
				bad.PaneKey = "foreign"
			case "authority":
				bad.Observation.AuthorityID = "foreign"
			case "incarnation":
				bad.Observation.Incarnation++
			case "boundary":
				bad.Observation.Boundary = true
			case "interrupted":
				bad.Interrupted = true
			case "tool-output":
				bad.ToolOutput = true
			case "root":
				bad.MainAgent.StateStartedAt++
			case "gap":
				l.feed = gappedMessageFeed{feed}
			}
			feed.events = []native.AgentState{bad}
			r, m, err := l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic == "" || m != nil || r.TurnEvidence != nil || host.nativeSends != 1 {
				t.Fatal("cached input crossed a boundary", fault, r.Diagnostic, m, err)
			}
		})
	}
}

func TestOrcaNativeInputQueryCannotHideLaterNotification(t *testing.T) {
	for _, fault := range []string{"foreign-prompt", "missing-prompt-new-marker"} {
		t.Run(fault, func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(input)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			end := fixtureState(12, "", "done")
			end.TurnStartedAt, end.LastAssistantMessage = 11500, "original result"
			feed.current = end
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			l.queryInput = func(_ context.Context, _ b.ProcessIdentity, _ string, _ int64, current native.AgentState) (native.AgentState, error) {
				current.NativeInput = recordedInput(input, current)
				later := messageProgress(input, 13, "foreign progress")
				later.TurnStartedAt = 12500
				if fault == "foreign-prompt" {
					later.Prompt = "foreign"
				} else {
					later.Prompt = ""
				}
				feed.events = []native.AgentState{later}
				return current, nil
			}
			r, m, err := l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic == "" || r.TurnEvidence != nil || m != nil || host.nativeSends != 1 {
				t.Fatal("query hid later source", r.Diagnostic, r.TurnEvidence, m, err)
			}
		})
	}
}

func TestOrcaNativeInputIdentityErrorRemainsDurable(t *testing.T) {
	for _, mode := range []string{"identity-error", "pre-outcome", "stale-outcome", "post-outcome", "post-boundary"} {
		t.Run(mode, func(t *testing.T) {
			l, _, feed, s, input := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(input)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			progress := messageProgress(input, 12, "")
			progress.Prompt, progress.TurnStartedAt = "", 11500
			if mode == "pre-outcome" || mode == "stale-outcome" {
				progress = fixtureState(12, "", "done")
				progress.MainAgent.Outcome = "failure"
				if mode == "stale-outcome" {
					progress.UpdatedAt = progress.MainAgent.StateStartedAt - 1
				}
			}
			feed.current = progress
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			calls := 0
			l.queryInput = func(_ context.Context, _ b.ProcessIdentity, _ string, _ int64, current native.AgentState) (native.AgentState, error) {
				calls++
				if mode == "identity-error" {
					return current, fmt.Errorf("%w: original pane changed", native.ErrStateIdentityChanged)
				}
				after := fixtureState(13, "", "done")
				if mode == "post-outcome" {
					after.MainAgent.Outcome = "failure"
				} else {
					after.SessionBoundary = true
				}
				return after, fmt.Errorf("original native input changed during read")
			}
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			conflict := turnFor(&state, input.CommandID).Native.Conflict
			if (conflict != "") != (mode != "stale-outcome") {
				t.Fatal("wrong durable conflict", mode, conflict, r.Diagnostic)
			}
			if (mode == "pre-outcome" || mode == "stale-outcome") && calls != 0 {
				t.Fatal("queried supplemental input before native contradiction/freshness checks", calls)
			}
		})
	}
}

func TestOrcaNativeInputCachedBindingHandlesFeedAndStaleSamples(t *testing.T) {
	for _, mode := range []string{"stale-notification", "stale-before-query", "feed-replacement"} {
		t.Run(mode, func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(input)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			progress := messageProgress(input, 12, "")
			progress.Prompt, progress.TurnStartedAt = "", 11500
			feed.current = progress
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			l.queryInput = func(_ context.Context, _ b.ProcessIdentity, _ string, _ int64, current native.AgentState) (native.AgentState, error) {
				current.NativeInput = recordedInput(input, current)
				return current, nil
			}
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic != "" {
				t.Fatal(r.Diagnostic, err)
			}
			bind(&s, r)
			if mode == "stale-before-query" {
				stale := messageProgress(input, 11, "old display")
				stale.Prompt = ""
				feed.events = []native.AgentState{stale}
				r, _, err = l.observeOnce(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
			}
			progress = messageProgress(input, 13, "first progress")
			progress.Prompt, progress.TurnStartedAt = "", 11500
			feed.events = []native.AgentState{progress}
			r, m, err := l.observeOnce(context.Background(), s)
			if err != nil || m == nil {
				t.Fatal(err, m)
			}
			bind(&s, r)
			l.acknowledgeMessage(m)
			if mode != "feed-replacement" {
				stale := progress
				stale.Observation = &native.StateObservation{}
				*stale.Observation = *progress.Observation
				stale.Observation.Revision = 12
				feed.events = []native.AgentState{stale}
				r, _, err = l.observeOnce(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
			} else {
				feed = &controlledStateFeed{initial: progress, current: progress}
				l.feed = feed
			}
			progress = messageProgress(input, 14, "next progress")
			progress.Prompt, progress.TurnStartedAt = "", 11500
			feed.events = []native.AgentState{progress}
			r, m, err = l.observeOnce(context.Background(), s)
			if err != nil || host.nativeSends != 1 {
				t.Fatal(err, host.nativeSends)
			}
			if mode != "feed-replacement" && (r.Diagnostic != "" || m == nil) {
				t.Fatal("stale sample revoked valid input binding", r.Diagnostic, m)
			}
			if mode == "feed-replacement" && (r.Diagnostic == "" || m != nil) {
				t.Fatal("new feed inherited input binding", r.Diagnostic, m)
			}
		})
	}
}

func TestOrcaNativeInputProofRejectsUnboundHistory(t *testing.T) {
	for _, fault := range []string{"text", "prefix-only", "baseline", "missing-id", "old-row", "future-row", "authority", "incarnation", "revision", "observation-time", "observation-before-main", "no-start", "no-acceptance", "no-working", "provider", "pane", "new-root", "boundary", "restored", "interrupted", "tool-output", "conflict"} {
		t.Run(fault, func(t *testing.T) {
			l, _, feed, s, input := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(input)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, _ := l.state(s)
			turn := turnFor(&state, input.CommandID)
			end := fixtureState(12, "", "done")
			end.TurnStartedAt = 11500
			end.LastAssistantMessage = "unverified"
			end.NativeInput = recordedInput(input, end)
			switch fault {
			case "text":
				end.NativeInput.Text = "foreign"
			case "prefix-only":
				end.NativeInput.Text = input.Prompt + "foreign suffix"
			case "baseline":
				end.NativeInput.Baseline--
			case "missing-id":
				end.NativeInput.ID = ""
			case "old-row":
				end.NativeInput.Timestamp = 10000
			case "future-row":
				end.NativeInput.Timestamp = end.UpdatedAt + 1
			case "authority":
				end.NativeInput.Observation.AuthorityID = "other"
			case "incarnation":
				end.NativeInput.Observation.Incarnation++
			case "revision":
				end.NativeInput.Observation.Revision = 0
			case "observation-time":
				end.NativeInput.Observation.ObservedAt = 0
			case "observation-before-main":
				end.NativeInput.Observation.ObservedAt = end.MainAgent.StateStartedAt - 1
			case "no-start":
				turn.Started = false
			case "no-acceptance":
				turn.Accepted = false
			case "no-working":
				turn.Native.WorkingAt = 0
			case "provider":
				end.ProviderSession.ID = "other"
			case "pane":
				end.PaneKey = "other"
			case "new-root":
				end.MainAgent.State = "working"
				end.MainAgent.StateStartedAt++
			case "boundary":
				end.SessionBoundary = true
			case "restored":
				end.RestoredUnconfirmed = true
			case "interrupted":
				end.Interrupted = true
			case "tool-output":
				end.ToolOutput = true
			case "conflict":
				turn.Native.Conflict = "historical conflict"
			}
			turn.Native.query(end, turn, time.Unix(131, 0), false)
			if turn.Native.hasFinal() {
				t.Fatal("foreign history accepted", fault)
			}
		})
	}
}
