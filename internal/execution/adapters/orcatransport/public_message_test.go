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

func publicMessageFixture(t *testing.T) (*Lifecycle, *fakeHost, *controlledStateFeed, b.LifecycleSnapshot, b.TurnInput) {
	t.Helper()
	l, host, s := fixtureOn(t, "paired-worker")
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	now := time.Unix(100, 0).UTC()
	l.clock = func() time.Time { return now }
	base := fixtureState(10, "previous", "done")
	base.LastAssistantMessage = "old answer"
	feed := &controlledStateFeed{initial: base, current: base}
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
	s, input := prepared(t, l, s, strings.Repeat("c", 32))
	r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	t.Cleanup(l.closeStateFeed)
	return l, host, feed, s, input
}

func messageStart(input b.TurnInput) native.AgentState {
	s := fixtureState(11, input.Prompt, "working")
	s.TurnStartedAt = s.MainAgent.StateStartedAt
	s.LastAssistantMessage = "old answer"
	return s
}

func messageProgress(input b.TurnInput, revision int, body string) native.AgentState {
	s := fixtureState(revision, input.Prompt, "working")
	s.MainAgent.StateStartedAt, s.TurnStartedAt = 11000, 11000
	s.LastAssistantMessage = body
	return s
}

func TestOrcaPublicMessageObservedBeforeFinalAndDeduplicated(t *testing.T) {
	l, host, feed, s, input := publicMessageFixture(t)
	feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "public progress")}
	r, m, err := l.observeOnce(context.Background(), s)
	if err != nil || m == nil || m.Text != "public progress" || r.TurnEvidence != nil || r.State.TurnCompleted {
		t.Fatal(r, m, err)
	}
	if m.Generation != s.Session.Generation || m.Generation == s.ResourceGeneration || m.TurnID != input.CommandID || !m.ObservedAt.Equal(time.UnixMilli(12000).UTC()) {
		t.Fatal("wrong identity/receive timestamp", m)
	}
	bind(&s, r)
	l.acknowledgeMessage(m)
	// Native status/revision changed without a new public message.
	feed.events = []native.AgentState{messageProgress(input, 13, m.Text)}
	r, dup, err := l.observeOnce(context.Background(), s)
	if err != nil || dup != nil || host.nativeSends != 1 {
		t.Fatal("duplicate or extra input", dup, err, host.nativeSends)
	}
	bind(&s, r)
	feed.events = []native.AgentState{messageProgress(input, 12, "stale text")}
	if _, stale, err := l.observeOnce(context.Background(), s); err != nil || stale != nil {
		t.Fatal("old source replaced message", stale, err)
	}
}

func TestOrcaPublicMessageCandidateInvalidatedByLaterConflict(t *testing.T) {
	for _, fault := range []string{"prompt", "provider", "pane", "boundary", "restored", "tool-output", "interrupted"} {
		t.Run(fault, func(t *testing.T) {
			l, _, feed, s, input := publicMessageFixture(t)
			bad := messageProgress(input, 13, "foreign text")
			switch fault {
			case "prompt":
				bad.Prompt = "other input"
			case "provider":
				bad.ProviderSession.ID = "replacement"
			case "pane":
				bad.PaneKey = "other:pane"
			case "boundary":
				bad.SessionBoundary = true
			case "restored":
				bad.RestoredUnconfirmed = true
			case "tool-output":
				bad.ToolOutput = true
			case "interrupted":
				bad.Interrupted = true
			}
			feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "valid before conflict"), bad}
			_, m, err := l.observeOnce(context.Background(), s)
			if err != nil || m != nil {
				t.Fatal("batch conflict leaked candidate", m, err)
			}
		})
	}
}

func TestOrcaPublicMessageFirstReadAndCancellation(t *testing.T) {
	l, host, feed, s, input := publicMessageFixture(t)
	feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "first read")}
	_, obs, err := l.ObserveLifecycle(context.Background(), s)
	if err != nil || obs == nil {
		t.Fatal(err)
	}
	select {
	case e := <-obs.Events():
		if e.PublicMessage == nil || e.PublicMessage.Text != "first read" || len(e.Implementation) > 0 || e.Fact.Completed || e.Observation != nil {
			t.Fatal("first read lost or mixed with durable facts", e)
		}
		// Deliberately do not acknowledge; close must interrupt both waits.
	case <-time.After(time.Second):
		t.Fatal("first candidate not delivered")
	}
	_ = obs.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := obs.(*observer).awaitCleanup(ctx); err != nil || host.nativeSends != 1 {
		t.Fatal("cancel blocked or resent", err)
	}
	if l.nativeObservation != nil {
		t.Fatal("observation context leaked")
	}
}

func TestOrcaPublicMessageReconcileDoesNotConsumeCandidate(t *testing.T) {
	l, _, feed, s, input := publicMessageFixture(t)
	feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "not consumed")}
	r, first, err := l.observeOnce(context.Background(), s)
	if err != nil || first == nil {
		t.Fatal(err, first)
	}
	bind(&s, r)
	_, again, err := l.observeOnce(context.Background(), s)
	if err != nil || again == nil || *again != *first {
		t.Fatal("read without delivery consumed candidate", again, err)
	}
}

type reattachedMessageFeed struct {
	*controlledStateFeed
	initialPending bool
}

func (f *reattachedMessageFeed) Drain() ([]native.StateSample, error) {
	samples, err := f.controlledStateFeed.Drain()
	if f.initialPending {
		f.initialPending = false
		samples = append([]native.StateSample{f.Initial()}, samples...)
	}
	return samples, err
}

func TestOrcaPublicMessageActualReconcileAndReattachRetainsCandidate(t *testing.T) {
	l, _, feed, s, input := publicMessageFixture(t)
	feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "retained across reattach")}
	r, obs, err := l.Reconcile(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
	if err != nil || obs != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
		return &reattachedMessageFeed{controlledStateFeed: feed, initialPending: true}, nil
	}
	_, message, err := l.observeOnce(context.Background(), s)
	if err != nil || message == nil || message.Text != "retained across reattach" {
		t.Fatal("Reconcile consumed the undelivered sample", message, err)
	}
}

func TestOrcaPublicMessageObserverKeepsStartedAndCompletedFacts(t *testing.T) {
	l, host, feed, s, input := publicMessageFixture(t)
	end := fixtureState(13, input.Prompt, "done")
	end.LastAssistantMessage = "native final"
	end.TurnStartedAt = 11000
	feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "progress"), end}
	obs := l.observe(s, nil)
	defer obs.Close(context.Background())
	started, completed, messages := 0, 0, 0
	events, done := obs.Events(), obs.Done()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if event.PublicMessage != nil {
				messages++
				if event.Fact.Kind != "" || len(event.Implementation) != 0 || event.Observation != nil {
					t.Fatal("mixed public message", event)
				}
			}
			if event.Fact.Kind == b.EventStarted {
				started++
			}
			if event.Fact.Kind == b.EventCompleted {
				completed++
				if len(event.Implementation) == 0 {
					t.Fatal("completion lost its proof")
				}
			}
			event.Persisted <- nil
			event.Processed <- nil
		case err := <-done:
			if err != nil || started != 1 || completed != 1 || messages != 1 || host.nativeSends != 1 {
				t.Fatal("message swallowed lifecycle facts", started, completed, messages, err)
			}
			if l.nativeObservation != nil || l.feed != nil {
				t.Fatal("Done preceded cleanup")
			}
			return
		case <-time.After(2 * time.Second):
			t.Fatal("observation did not complete")
		}
	}
}

type gappedMessageFeed struct{ *controlledStateFeed }

func (f gappedMessageFeed) Drain() ([]native.StateSample, error) {
	samples, _ := f.controlledStateFeed.Drain()
	return samples, errors.New("native observation gap")
}

func TestOrcaCompletionQueryEmptyPromptRequiresCurrentStartAndRetainedBoundary(t *testing.T) {
	for _, mode := range []string{"linked", "no-marker", "changed-marker", "gap", "query-only", "restart", "conflict", "legacy-conflict"} {
		t.Run(mode, func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			start := messageStart(input)
			if mode == "no-marker" {
				start.TurnStartedAt = 0
			}
			feed.events = []native.AgentState{start}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			end := fixtureState(12, "", "done")
			end.TurnStartedAt = start.TurnStartedAt
			end.LastAssistantMessage = "native final"
			switch mode {
			case "changed-marker":
				end.TurnStartedAt++
			case "conflict":
				end.Prompt = "unrelated input"
			case "legacy-conflict":
				state, _ := l.state(s)
				turnFor(&state, input.CommandID).Native.Conflict = "Native original completion/input provenance is unverified"
				s.Implementation, _ = l.envelope(s, state)
			case "restart":
				l.clearObservation(l.nativeObservation)
			case "gap":
				l.feed = gappedMessageFeed{feed}
			}
			if mode == "query-only" {
				feed.current = end
				l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			} else {
				feed.events = []native.AgentState{end}
			}
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || (r.TurnEvidence != nil) != (mode == "linked") || host.nativeSends != 1 {
				t.Fatal("wrong empty-prompt verdict", mode, r, err)
			}
			state, err := l.state(b.LifecycleSnapshot{Ref: s.Ref, Session: s.Session, Implementation: r.Implementation, ConfigurationDigest: s.ConfigurationDigest, ResourceGeneration: s.ResourceGeneration, PendingTurn: s.PendingTurn})
			if err != nil {
				t.Fatal(err)
			}
			proof := turnFor(&state, input.CommandID).Native
			if mode == "linked" {
				if r.TurnEvidence.NativeOutcome != "unreported" || r.TurnEvidence.LastMessage != end.LastAssistantMessage {
					t.Fatal(r.TurnEvidence)
				}
			} else if mode != "conflict" && mode != "legacy-conflict" && proof.Conflict != "" {
				t.Fatal("missing evidence became permanent", proof.Conflict)
			}
		})
	}
}

func TestOrcaCompletionQuerySupplementsOnlyAlreadyProvenEmptyPromptEnd(t *testing.T) {
	for _, mode := range []string{"same-end", "changed-marker", "changed-end", "restart"} {
		t.Run(mode, func(t *testing.T) {
			l, host, feed, s, input := publicMessageFixture(t)
			end := fixtureState(12, "", "done")
			end.TurnStartedAt = 11000
			feed.events = []native.AgentState{messageStart(input), end}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil || r.TurnEvidence != nil {
				t.Fatal("empty body completed", r.TurnEvidence, err)
			}
			bind(&s, r)
			end.LastAssistantMessage = "later complete original body"
			switch mode {
			case "changed-marker":
				end.TurnStartedAt++
			case "changed-end":
				end.MainAgent.StateStartedAt++
			case "restart":
				l.clearObservation(l.nativeObservation)
			}
			feed.current = end
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || (r.TurnEvidence != nil) != (mode == "same-end") || host.nativeSends != 1 {
				t.Fatal("wrong body supplementation", mode, r.TurnEvidence, err)
			}
		})
	}
}

func TestOrcaObservationDiagnosticPersistsBetweenQueriesAndClearsOnProof(t *testing.T) {
	l, host, feed, s, input := publicMessageFixture(t)
	end := fixtureState(12, "", "done")
	end.LastAssistantMessage = "original full reply"
	feed.current = end
	for _, second := range []int64{131, 132} {
		l.clock = func() time.Time { return time.Unix(second, 0).UTC() }
		r, _, err := l.observeOnce(context.Background(), s)
		if err != nil || r.Diagnostic != nativePromptMissing || r.TurnEvidence != nil {
			t.Fatal("transient diagnostic cleared without evidence", r, err)
		}
		bind(&s, r)
	}
	// A restored subscription cannot clear the missing-evidence projection
	// merely because its transient diagnostic context starts empty.
	l.clearObservation(l.nativeObservation)
	feed.initial = end
	l.feed = &reattachedMessageFeed{controlledStateFeed: feed, initialPending: true}
	r, _, err := l.observeOnce(context.Background(), s)
	if err != nil || r.Diagnostic != nativePromptMissing || r.TurnEvidence != nil {
		t.Fatal("reattach cleared unavailable attribution", r, err)
	}
	bind(&s, r)
	end.Prompt = preview(input.Prompt)
	feed.current = end
	l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
	r, _, err = l.observeOnce(context.Background(), s)
	if err != nil || r.Diagnostic != "" || r.TurnEvidence == nil || host.nativeSends != 1 {
		t.Fatal("same original attribution did not recover", r, err)
	}
}
