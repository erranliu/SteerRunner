package orcatransport

import (
	"context"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Completion/progress unit tests below begin with an independently confirmed
// original input. This is a state precondition, never a fabricated send receipt.
// Submission qualification and the actual history read are exercised separately
// by native_submit, native_start and fixed-control lifecycle regressions.
func confirmedNativeTurn(t *testing.T, l *Lifecycle, s b.LifecycleSnapshot) b.LifecycleSnapshot {
	t.Helper()
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	turn := turnFor(&state, command(s))
	if turn == nil || !turn.Accepted {
		t.Fatal("cannot confirm an unaccepted fixture input")
	}
	turn.Started = true
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type hostStateFeed struct {
	host    *fakeHost
	initial native.AgentState
	seen    int
	hinted  bool
}

func fixtureState(seq int, prompt, state string) native.AgentState {
	s := native.AgentState{PaneKey: "tab:pane", AgentType: "codex", Prompt: preview(prompt), State: state, UpdatedAt: int64(seq * 1000), StateStartedAt: int64(seq * 1000), MainAgent: &native.MainAgentState{State: state, StateStartedAt: int64(seq * 1000)}, Observation: &native.StateObservation{Origin: "hook", Kind: "transition", AuthorityID: "hooks", Revision: int64(seq), ObservedAt: int64(seq * 1000)}}
	s.ProviderSession.ID = "provider"
	return s
}
func (f *fakeHost) stateFactory(_ context.Context, _ string, _ b.ProcessIdentity) (native.StateFeed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := fixtureState(f.nativeSends*2, "", "done")
	return &hostStateFeed{host: f, initial: s}, nil
}
func fixtureSample(s native.AgentState, kind native.SampleKind) native.StateSample {
	return native.StateSample{AgentState: s, ReceivedAt: time.UnixMilli(s.UpdatedAt).UTC(), Kind: kind}
}
func (f *hostStateFeed) Initial() native.StateSample {
	return fixtureSample(f.initial, native.SampleInitial)
}
func (f *hostStateFeed) Drain() ([]native.StateSample, error) {
	f.host.mu.Lock()
	defer f.host.mu.Unlock()
	h := f.host
	var states []native.AgentState
	if h.nativeSends > f.seen {
		states = append(states, fixtureState(h.nativeSends*2-1, h.nativePrompt, "working"))
		f.seen = h.nativeSends
		f.hinted = false
	}
	if h.nativeSends > 0 && h.writeReply && !h.busy && !f.hinted {
		s := fixtureState(h.nativeSends*2, h.nativePrompt, "done")
		s.Observation = nil
		s.MainAgent = nil
		states = append(states, s)
		f.hinted = true
	}
	samples := make([]native.StateSample, len(states))
	for i, s := range states {
		samples[i] = fixtureSample(s, native.SampleNotification)
	}
	return samples, nil
}
func (f *hostStateFeed) Query(context.Context) (native.StateSample, error) {
	f.host.mu.Lock()
	defer f.host.mu.Unlock()
	h := f.host
	if h.nativeSends == 0 {
		return fixtureSample(fixtureState(0, "", "done"), native.SampleQuery), nil
	}
	state := "working"
	seq := h.nativeSends*2 - 1
	if h.writeReply && !h.busy {
		state = "done"
		seq++
	}
	s := fixtureState(seq, h.nativePrompt, state)
	if state == "done" {
		s.MainAgent.Outcome = "success"
		s.LastAssistantMessage = h.nativeReply
	}
	return fixtureSample(s, native.SampleQuery), nil
}
func (*hostStateFeed) Close() {}
