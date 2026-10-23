package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaNativeIncompleteMetadataRecovers(t *testing.T) {
	for _, source := range []string{"notification", "query"} {
		for _, field := range []string{"prompt", "agent", "provider", "root"} {
			for _, phase := range []string{"working", "done"} {
				t.Run(source+"/"+field+"/"+phase, func(t *testing.T) {
					l, host, s := fixture(t)
					r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
					if err != nil {
						t.Fatal(err)
					}
					bind(&s, r)
					now := time.Unix(100, 0).UTC()
					l.clock = func() time.Time { return now }
					base := fixtureState(10, "previous", "done")
					feed := &controlledStateFeed{initial: base, current: base}
					l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
					s, input := prepared(t, l, s, strings.Repeat("c", 32))
					r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
					if err != nil {
						t.Fatal(err)
					}
					bind(&s, r)
					poll := func() b.LifecycleReceipt {
						r, _, err := l.observeOnce(context.Background(), s)
						if err != nil {
							t.Fatal(err)
						}
						bind(&s, r)
						return r
					}
					start := fixtureState(11, input.Prompt, "working")
					if field != "prompt" {
						feed.events, feed.current = []native.AgentState{start}, start
						poll()
					}
					partial := fixtureState(12, input.Prompt, phase)
					if phase == "working" {
						partial.MainAgent.StateStartedAt = start.MainAgent.StateStartedAt
					}
					partial.LastAssistantMessage = "Complete original final"
					switch field {
					case "prompt":
						partial.Prompt = ""
					case "agent":
						partial.AgentType = ""
					case "provider":
						partial.ProviderSession.ID = ""
					case "root":
						partial.MainAgent = nil
					}
					feed.current = partial
					if source == "notification" {
						feed.events = []native.AgentState{partial}
					}
					now = now.Add(30 * time.Second)
					r = poll()
					if r.TurnEvidence != nil || r.Diagnostic == "" {
						t.Fatal("partial metadata completed or was invisible", r)
					}
					state, err := l.state(s)
					if err != nil || turnFor(&state, input.CommandID).Native.Conflict != "" {
						t.Fatal("missing metadata poisoned the original", err)
					}
					queries := feed.queries
					if poll().TurnEvidence != nil || feed.queries != queries {
						t.Fatal("partial metadata caused a tight query loop")
					}
					// Recreate the adapter from its persisted envelope while incomplete.
					l.closeStateFeed()
					port, err := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
					if err != nil {
						t.Fatal(err)
					}
					l = port.(*Lifecycle)
					l.clock = func() time.Time { return now }
					end := fixtureState(13, input.Prompt, "done")
					end.LastAssistantMessage = partial.LastAssistantMessage
					feed.current = end
					now = now.Add(30 * time.Second)
					r = poll()
					if r.TurnEvidence == nil || r.TurnEvidence.LastMessage != end.LastAssistantMessage || r.Diagnostic != "" || host.nativeSends != 1 {
						t.Fatal("original final did not recover after restore", r.Diagnostic, host.nativeSends)
					}
				})
			}
		}
	}
}

func TestOrcaNativeMissingPromptCannotHideConflict(t *testing.T) {
	for _, kind := range []string{"provider", "pane", "agent", "session", "restored", "boundary", "outcome", "interrupted"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Unix(100, 0).UTC()
			turn := Turn{Accepted: true, Started: true, Prompt: "original", Native: nativeBaseline(fixtureState(10, "previous", "done"), now)}
			turn.Native.signal(fixtureState(11, turn.Prompt, "working"), &turn, now, false)
			changed := fixtureState(12, "", "done")
			changed.LastAssistantMessage = "must not accept"
			switch kind {
			case "provider":
				changed.ProviderSession.ID = "foreign"
			case "pane":
				changed.PaneKey = "foreign"
			case "agent":
				changed.AgentType = "foreign"
			case "session":
				changed.SessionBoundary = true
			case "restored":
				changed.RestoredUnconfirmed = true
			case "boundary":
				changed.MainAgent.State = "working"
				changed.Observation.Boundary = true
			case "outcome":
				changed.MainAgent.Outcome = "unknown"
			case "interrupted":
				changed.Interrupted = true
			}
			if kind == "boundary" {
				turn.Native.signal(changed, &turn, now, false)
			} else {
				turn.Native.query(changed, &turn, now, false)
			}
			if turn.Native.Conflict == "" || turn.Native.Waiting != "" {
				t.Fatal("missing prompt hid positive conflict", turn.Native)
			}
			raw, _ := json.Marshal(turn.Native)
			if err := json.Unmarshal(raw, &turn.Native); err != nil {
				t.Fatal(err)
			}
			end := fixtureState(13, turn.Prompt, "done")
			end.LastAssistantMessage = "later valid-looking final"
			if turn.Native.query(end, &turn, now.Add(time.Minute), false) {
				t.Fatal("restore or later query erased conflict")
			}
		})
	}
}

func TestOrcaNativeFinalRetainsVerifiedInputBinding(t *testing.T) {
	for _, source := range []string{"notification", "query", "restored-query"} {
		t.Run(source, func(t *testing.T) {
			l, host, s := fixture(t)
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			now := time.Unix(100, 0).UTC()
			l.clock = func() time.Time { return now }
			base := fixtureState(10, "previous", "done")
			feed := &controlledStateFeed{initial: base, current: base}
			l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			start := fixtureState(11, input.Prompt, "working")
			feed.events, feed.current = []native.AgentState{start}, start
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.TurnEvidence != nil {
				t.Fatal("invalid original start", err)
			}
			bind(&s, r)
			// Actual tool hooks clear the display prompt for the rest of the turn.
			tool := fixtureState(12, "", "working")
			tool.MainAgent.StateStartedAt = start.MainAgent.StateStartedAt
			feed.events, feed.current = []native.AgentState{tool}, tool
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.Diagnostic != "" || r.TurnEvidence != nil {
				t.Fatal("tool display erased verified input binding", err, r.Diagnostic)
			}
			bind(&s, r)
			if source == "restored-query" {
				l.closeStateFeed()
				port, err := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
				if err != nil {
					t.Fatal(err)
				}
				l = port.(*Lifecycle)
				l.clock = func() time.Time { return now }
			}
			end := fixtureState(13, "", "done")
			end.LastAssistantMessage = "Complete original final with no display prompt"
			feed.current = end
			if source == "notification" {
				feed.events = []native.AgentState{end}
			}
			now = now.Add(30 * time.Second)
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.TurnEvidence == nil || r.TurnEvidence.LastMessage != end.LastAssistantMessage || r.TurnEvidence.NativeOutcome != "unreported" || host.nativeSends != 1 {
				t.Fatal("original bound final could not complete", err, r.Diagnostic, host.nativeSends)
			}
		})
	}
}

func TestOrcaNativeVerifiedFinalSurvivesWaitingDisplay(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	turn := Turn{Accepted: true, Started: true, Prompt: "original", Native: nativeBaseline(fixtureState(10, "previous", "done"), now)}
	turn.Native.signal(fixtureState(11, turn.Prompt, "working"), &turn, now, false)
	end := fixtureState(12, turn.Prompt, "done")
	end.LastAssistantMessage = "Complete original final"
	turn.Native.signal(end, &turn, now, false)
	if !turn.Native.hasFinal() {
		t.Fatal("original final missing")
	}
	waiting := fixtureState(13, turn.Prompt, "waiting")
	turn.Native.signal(waiting, &turn, now, false)
	turn.Native.query(waiting, &turn, now, false)
	if turn.Native.diagnostic() != "" || turn.Native.ReplyBody != end.LastAssistantMessage {
		t.Fatal("later display state revoked original final", turn.Native)
	}
	// A subsequent positively identified input boundary is still a conflict.
	other := fixtureState(14, "", "working")
	other.Observation.Boundary = true
	turn.Native.signal(other, &turn, now, false)
	if turn.Native.Conflict == "" {
		t.Fatal("saved final hid an observed foreign input")
	}
}

func TestOrcaObserverReportsChangingGapAndRecovery(t *testing.T) {
	l, host, s := fixture(t)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	now := time.Unix(100, 0).UTC()
	l.clock = func() time.Time { return now }
	base := fixtureState(10, "previous", "done")
	feed := &controlledStateFeed{initial: base, current: base}
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
	s, input := prepared(t, l, s, strings.Repeat("c", 32))
	r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	start := fixtureState(11, input.Prompt, "working")
	partial := fixtureState(12, "", "working")
	partial.MainAgent.StateStartedAt = start.MainAgent.StateStartedAt
	partial.ProviderSession.ID = ""
	feed.events, feed.current = []native.AgentState{start, partial}, partial
	observer := l.observe(s, nil)
	defer observer.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var diagnostics []string
	for {
		select {
		case event, ok := <-observer.Events():
			if !ok {
				t.Fatal("observer stopped before completion", diagnostics)
			}
			if status := event.Observation; status != nil {
				diagnostics = append(diagnostics, status.Diagnostic)
				if event.Fact.StartedTurnID != input.CommandID {
					t.Fatal("unbound observation diagnostic")
				}
				// The observer is blocked on its acknowledgements while fixtures change.
				switch len(diagnostics) {
				case 1:
					feed.err = errors.New("temporary original query failure")
					now = now.Add(30 * time.Second)
				case 2:
					feed.err = nil
					feed.current = fixtureState(13, "", "done")
					feed.current.LastAssistantMessage = "Complete original final"
					now = now.Add(30 * time.Second)
				}
			}
			event.Persisted <- nil
			event.Processed <- nil
			if event.Fact.Kind == b.EventCompleted {
				if len(diagnostics) != 3 || diagnostics[0] == "" || diagnostics[1] == "" || diagnostics[0] == diagnostics[1] || diagnostics[2] != "" || host.nativeSends != 1 {
					t.Fatal("diagnostic lifecycle lost", diagnostics, host.nativeSends)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("original completion timed out", diagnostics)
		}
	}
}
