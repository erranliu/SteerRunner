package orcatransport

import (
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const nativeWaiting = "Native main agent is waiting; no next input was sent"
const nativePromptMissing = "Native original input preview is missing; awaiting original attribution evidence"

type provenanceKind uint8

const (
	provenanceMatched provenanceKind = iota
	provenanceMissing
	provenanceConflict
)

type provenance struct {
	kind       provenanceKind
	diagnostic string
}

// attribution is shared by notifications, independent queries and public
// sampling. Missing fields are not durable contradictions.
func (p NativeProof) attribution(s native.AgentState, turn *Turn, linked bool) provenance {
	if s.SessionBoundary || s.RestoredUnconfirmed || s.Prompt != "" && s.Prompt != preview(turn.Prompt) {
		return provenance{provenanceConflict, "Native original completion/input provenance is unverified"}
	}
	if s.ToolOutput {
		return provenance{provenanceMissing, "Native original final body is tool output; awaiting original assistant evidence"}
	}
	if s.Prompt == preview(turn.Prompt) {
		return provenance{}
	}
	if s.Prompt == "" {
		if linked {
			return provenance{}
		}
		return provenance{provenanceMissing, nativePromptMissing}
	}
	return provenance{provenanceConflict, "Native original completion/input provenance is unverified"}
}

// nativeObservation owns only transient observation coverage and one bounded
// public candidate. NativeProof remains the sole durable execution authority.
type nativeObservation struct {
	identity      b.PublicMessage
	feed          native.StateFeed
	continuous    bool
	turnStartedAt int64
	lastDigest    string
	lastReceived  time.Time
	lastSourceAt  int64
	pending       *b.PublicMessage
	diagnostic    string
}

func (l *Lifecycle) observation(s b.LifecycleSnapshot, feed native.StateFeed) *nativeObservation {
	id := b.PublicMessage{AttemptID: s.Session.AttemptID, Generation: s.Session.Generation, SessionID: SessionID(s.Ref.External), TurnID: command(s), Kind: "assistant"}
	o := l.nativeObservation
	if o == nil || o.identity != id {
		o = &nativeObservation{identity: id, feed: feed}
		if feed != nil {
			o.lastDigest = digest([]byte(feed.Initial().LastAssistantMessage))
		}
		l.nativeObservation = o
	} else if o.feed != feed {
		o.feed, o.continuous = feed, false
	}
	return o
}

func (o *nativeObservation) invalidate() {
	o.continuous, o.turnStartedAt, o.pending = false, 0, nil
}

func (o *nativeObservation) reattach(s native.AgentState, p NativeProof, turn *Turn) {
	conflict := s.PaneKey != p.Pane || s.ProviderSession.ID != "" && p.Provider != "" && s.ProviderSession.ID != p.Provider ||
		s.AgentType != "" && s.AgentType != p.agentName() || s.SessionBoundary || s.RestoredUnconfirmed || s.ToolOutput || s.Interrupted ||
		s.MainAgent != nil && s.MainAgent.StateStartedAt > p.BaseRootAt && s.Prompt != "" && p.attribution(s, turn, false).kind == provenanceConflict ||
		o.turnStartedAt > 0 && s.TurnStartedAt > 0 && o.turnStartedAt != s.TurnStartedAt
	if conflict {
		o.invalidate()
		return
	}
	// A previously selected, undelivered public sample remains valid. A new
	// initial snapshot cannot establish coverage or replace that sample.
	o.continuous, o.turnStartedAt = false, 0
}

// Orca coalesces tab snapshots. A live connection alone is not boundary
// coverage. The retained native turn-start stamp must survive unchanged from
// the observed original start; absent stamps never grant empty-prompt proof.
func (o *nativeObservation) linked(sample native.StateSample, p NativeProof) bool {
	s := sample.AgentState
	return s.MainAgent != nil && s.MainAgent.State == "done" && o.continuous && o.turnStartedAt > 0 && s.TurnStartedAt == o.turnStartedAt &&
		sample.Kind == native.SampleNotification && trustedState(s) &&
		s.Observation.AuthorityID == p.Authority && s.Observation.Incarnation == p.Incarnation &&
		s.Observation.Revision > p.Revision
}

// A query may fill a missing body for an already proved end. It cannot
// establish an empty-prompt end or restore coverage lost on reconnect.
func (o *nativeObservation) supplements(sample native.StateSample, p NativeProof) bool {
	s := sample.AgentState
	return sample.Kind == native.SampleQuery && o.continuous && o.turnStartedAt > 0 && s.TurnStartedAt == o.turnStartedAt &&
		p.EndedAt > 0 && s.MainAgent != nil && s.MainAgent.State == "done" && s.MainAgent.StateStartedAt == p.EndedAt
}

func (o *nativeObservation) offer(sample native.StateSample, p NativeProof, turn *Turn, fresh bool) {
	s := sample.AgentState
	if s.TurnStartedAt > 0 && o.turnStartedAt > 0 && s.TurnStartedAt != o.turnStartedAt {
		o.invalidate()
		return
	}
	if p.Conflict != "" {
		o.invalidate()
		return
	}
	if !turn.Accepted || !turn.Started || s.MainAgent == nil || s.PaneKey != p.Pane ||
		s.AgentType != p.agentName() || s.ProviderSession.ID == "" || s.ProviderSession.ID != p.Provider ||
		s.SessionBoundary || s.RestoredUnconfirmed || s.ToolOutput || s.Interrupted {
		return
	}
	if sample.Kind == native.SampleInitial {
		return
	}
	if sample.Kind == native.SampleNotification && !fresh {
		return
	}
	if s.UpdatedAt < o.lastSourceAt {
		return
	}
	o.lastSourceAt = s.UpdatedAt
	ended := p.EndedAt > 0 && p.EndedAt == s.MainAgent.StateStartedAt
	working := fresh && sample.Kind == native.SampleNotification && trustedState(s) &&
		p.WorkingAt > 0 && s.MainAgent.State == "working" && s.MainAgent.StateStartedAt == p.RootWorkingAt &&
		p.attribution(s, turn, false).kind == provenanceMatched
	if !ended && !working {
		return
	}
	if strings.TrimSpace(s.LastAssistantMessage) == "" || len(s.LastAssistantMessage) > 32000 || sample.ReceivedAt.IsZero() || !sample.ReceivedAt.After(o.lastReceived) {
		return
	}
	hash := digest([]byte(s.LastAssistantMessage))
	if hash == o.lastDigest {
		o.pending = nil
		return
	}
	if o.pending != nil && o.pending.Text == s.LastAssistantMessage {
		return
	}
	if o.pending != nil && !sample.ReceivedAt.After(o.pending.ObservedAt) {
		return
	}
	m := o.identity
	m.Text, m.ObservedAt = s.LastAssistantMessage, sample.ReceivedAt
	o.pending = &m
}

func (l *Lifecycle) publicCandidate() *b.PublicMessage {
	l.observationMu.Lock()
	defer l.observationMu.Unlock()
	if l.profile.Environment == "" || l.nativeObservation == nil {
		return nil
	}
	m := l.nativeObservation.pending
	if m == nil {
		return nil
	}
	copy := *m
	return &copy
}

func (l *Lifecycle) acknowledgeMessage(m *b.PublicMessage) {
	if m == nil {
		return
	}
	l.observationMu.Lock()
	defer l.observationMu.Unlock()
	o := l.nativeObservation
	if o != nil && o.identity.AttemptID == m.AttemptID && o.identity.Generation == m.Generation && o.identity.SessionID == m.SessionID && o.identity.TurnID == m.TurnID {
		o.lastDigest = digest([]byte(m.Text))
		o.lastReceived = m.ObservedAt
		if o.pending != nil && o.pending.Text == m.Text && o.pending.ObservedAt.Equal(m.ObservedAt) {
			o.pending = nil
		}
	}
}

func received(sample native.StateSample, fallback time.Time) time.Time {
	if sample.ReceivedAt.IsZero() {
		return fallback
	}
	return sample.ReceivedAt
}

func (l *Lifecycle) clearObservation(owned *nativeObservation) {
	l.observationMu.Lock()
	defer l.observationMu.Unlock()
	if l.nativeObservation == owned {
		l.nativeObservation = nil
	}
}
