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

func TestCandidateWorkingStartRejectsForeignProofAndRetainsConflicts(t *testing.T) {
	for _, fault := range []string{"legacy-profile", "same-preview-other-full-input", "foreign-process", "wire-proof", "stale-revision", "provider-error", "metadata-authority-error", "known-incarnation", "primary-authority"} {
		t.Run(fault, func(t *testing.T) {
			l, _, _, snapshot, input := candidateWorkingFixture(t, "paired-worker")
			state, err := l.state(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			turn := turnFor(&state, input.CommandID)
			s := messageStart(input)
			s.ProviderSession.TranscriptPath = "original.jsonl"
			s.MetadataObservation, s.Observation = s.Observation, nil
			if fault == "primary-authority" {
				primary := *s.MetadataObservation
				primary.AuthorityID = "replacement"
				s.Observation = &primary
			}
			if fault == "legacy-profile" {
				l.profile.NativeTerminal = false
			}
			if fault == "known-incarnation" {
				turn.Native.Authority, turn.Native.Incarnation = "hooks", 1
			}
			if fault == "stale-revision" {
				turn.Native.Revision = 12
			}
			queries := 0
			l.queryInput = func(_ context.Context, p b.ProcessIdentity, _ string, baseline int64, current native.AgentState) (originalInputState, error) {
				queries++
				if fault == "provider-error" {
					current.ProviderSession.ID = "replacement"
					return originalInputState{AgentState: current}, errors.New("query fence failed")
				}
				if fault == "metadata-authority-error" {
					current.MetadataObservation.AuthorityID = "replacement"
					return originalInputState{AgentState: current}, errors.New("query fence failed")
				}
				i := &nativeInput{ID: "original", Text: nativePromptText(input.Prompt), Timestamp: 11000, Baseline: baseline}
				if fault == "same-preview-other-full-input" {
					i.Text += " foreign suffix"
				}
				if fault == "foreign-process" {
					p.Incarnation = "replacement"
				}
				i.query = &originalInputQuery{process: p, state: current, input: *i, working: true}
				if fault == "wire-proof" {
					raw, _ := json.Marshal(i)
					var decoded nativeInput
					_ = json.Unmarshal(raw, &decoded)
					i = &decoded
				}
				return originalInputState{AgentState: current, NativeInput: i}, nil
			}
			_, err = l.queryWorkingOriginalStart(context.Background(), snapshot, &state, turn, s)
			if turn.Started || turn.Native.hasFinal() {
				t.Fatal("foreign proof granted start/final", fault)
			}
			conflict := fault == "provider-error" || fault == "metadata-authority-error" || fault == "known-incarnation" || fault == "primary-authority"
			if conflict && (turn.Native.Conflict == "" || err == nil) {
				t.Fatal("query lost positive identity contradiction", fault, err)
			}
			if fault == "legacy-profile" && queries != 0 {
				t.Fatal("legacy profile acquired candidate start semantics")
			}
		})
	}
}

func candidateWorkingFixture(t *testing.T, environment string) (*Lifecycle, *fakeHost, *controlledStateFeed, b.LifecycleSnapshot, b.TurnInput) {
	t.Helper()
	l, host, s := sharedNativeFixture(t, environment)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	l.clock = func() time.Time { return time.Unix(100, 0).UTC() }
	base := fixtureState(10, "previous", "done")
	base.ProviderSession.TranscriptPath = "original.jsonl"
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

func TestCandidateWorkingStartNeverGrantsFinalOrStreamCoverage(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		for _, fault := range []string{"none", "second-user-same-prefix", "provider", "partial-done", "truncated-final", "same-prefix-notification", "authority"} {
			t.Run(environment+"/"+fault, func(t *testing.T) {
				l, host, feed, s, input := candidateWorkingFixture(t, environment)
				working := messageStart(input)
				working.ProviderSession.TranscriptPath = "original.jsonl"
				working.MetadataObservation, working.Observation = working.Observation, nil
				feed.current = working
				queries := 0
				l.queryInput = func(_ context.Context, p b.ProcessIdentity, _ string, baseline int64, current native.AgentState) (originalInputState, error) {
					queries++
					out := originalInputState{AgentState: current}
					if current.MainAgent.State == "done" && (fault == "second-user-same-prefix" || fault == "same-prefix-notification") {
						return out, errors.New("another complete user row after original baseline")
					}
					i := &nativeInput{ID: "original", Text: nativePromptText(input.Prompt), Timestamp: 11000, Baseline: baseline}
					frozen := current
					main := *current.MainAgent
					frozen.MainAgent = &main
					if current.MetadataObservation != nil {
						metadata := *current.MetadataObservation
						frozen.MetadataObservation = &metadata
					}
					i.query = &originalInputQuery{process: p, state: frozen, input: *i, working: current.MainAgent.State == "working"}
					out.NativeInput = i
					return out, nil
				}
				l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
				r, message, err := l.observeOnce(context.Background(), s)
				if err != nil || !r.State.TurnStarted || r.State.TurnCompleted || r.TurnEvidence != nil || message != nil || queries != 1 {
					t.Fatal("working read failed to establish only Started", r, message, queries, err)
				}
				bind(&s, r)
				if fault == "same-prefix-notification" {
					next := messageProgress(input, 13, "foreign streaming body")
					next.MainAgent.StateStartedAt, next.TurnStartedAt = 13000, 13000
					feed.events = []native.AgentState{next}
				}
				end := fixtureState(14, input.Prompt, "done")
				end.TurnStartedAt, end.LastAssistantMessage = 11000, "complete original result"
				end.ProviderSession.TranscriptPath = "original.jsonl"
				end.MetadataObservation, end.Observation = end.Observation, nil
				switch fault {
				case "provider":
					end.ProviderSession.ID = "replacement"
				case "partial-done":
					end.MetadataObservation = nil
				case "truncated-final":
					end.LastAssistantMessage = strings.Repeat("x", 8000)
				case "authority":
					end.MetadataObservation.AuthorityID = "replacement"
				}
				feed.current = end
				l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
				r, message, err = l.observeOnce(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
				state, err := l.state(s)
				if err != nil {
					t.Fatal(err)
				}
				turn := turnFor(&state, input.CommandID)
				if turn.Native.WorkingAt != 0 || turn.Native.RootWorkingAt != 0 || l.nativeObservation.continuous || message != nil && message.Text == "foreign streaming body" || host.nativeSends != 1 {
					t.Fatal("query Started acquired streaming authority or another send")
				}
				if fault == "none" {
					if r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || queries != 2 {
						t.Fatal("fresh done input proof did not settle original final", r.Diagnostic, queries)
					}
				} else if r.TurnEvidence != nil || turn.Native.hasFinal() {
					t.Fatal("working Started bypassed independent final proof", fault, r)
				}
			})
		}
	}
}
