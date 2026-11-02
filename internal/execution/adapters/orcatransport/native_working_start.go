package orcatransport

import (
	"context"
	"errors"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// This stricter working read requires a positive, corroborated input hook.
// Thin/partial metadata can never prove an active start. Completion keeps its
// separate proof: this query cannot grant a final or notification coverage.
func corroboratedWorkingInput(s native.AgentState) bool {
	o := s.MetadataObservation
	return !s.ThinMetadata && s.MainAgent != nil && s.State == "working" && s.MainAgent.State == "working" &&
		s.AgentType == "codex" && s.PaneKey != "" && s.ProviderSession.ID != "" && s.ProviderSession.TranscriptPath != "" &&
		s.Prompt != "" && s.MainAgent.StateStartedAt > 0 && s.TurnStartedAt == s.MainAgent.StateStartedAt &&
		s.UpdatedAt >= s.MainAgent.StateStartedAt && o.ValidHookTransition() && o.ObservedAt >= s.MainAgent.StateStartedAt &&
		(s.Observation == nil || *s.Observation == *o) &&
		!s.SessionBoundary && !s.RestoredUnconfirmed && !s.Interrupted && !s.ToolOutput
}

func sameWorkingInput(a, s native.AgentState) bool {
	return corroboratedWorkingInput(a) && corroboratedWorkingInput(s) &&
		a.PaneKey == s.PaneKey && a.ProviderSession == s.ProviderSession && *a.MainAgent == *s.MainAgent &&
		a.Prompt == s.Prompt && a.UpdatedAt == s.UpdatedAt && a.TurnStartedAt == s.TurnStartedAt &&
		a.LastAssistantMessage == s.LastAssistantMessage && *a.MetadataObservation == *s.MetadataObservation &&
		(a.Observation == nil && s.Observation == nil || a.Observation != nil && s.Observation != nil && *a.Observation == *s.Observation)
}

func (p *NativeProof) rejectWorkingMetadata(s native.AgentState) bool {
	for _, o := range []*native.StateObservation{s.MetadataObservation, s.Observation} {
		if !o.ValidHookTransition() {
			continue
		}
		if p.BaseAuthority != "" && o.AuthorityID != p.BaseAuthority ||
			p.Authority != "" && (o.AuthorityID != p.Authority || o.Incarnation != p.Incarnation) {
			p.reject("Native working input metadata authority changed")
			return true
		}
	}
	return false
}

func (i *nativeInput) matchesWorkingQuery(p b.ProcessIdentity, s native.AgentState) bool {
	return i != nil && i.query != nil && i.query.working && i.query.process == p &&
		i.ID == i.query.input.ID && i.Text == i.query.input.Text && i.Timestamp == i.query.input.Timestamp &&
		i.Baseline == i.query.input.Baseline && sameWorkingInput(i.query.state, s)
}

func queryFencedWorkingInput(ctx context.Context, environment string, p b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (originalInputState, error) {
	if !corroboratedWorkingInput(before) || baseline <= 0 || before.MainAgent.StateStartedAt <= baseline {
		return originalInputState{AgentState: before}, errors.New("original working input metadata is unverified")
	}
	terminal, err := queryInputTerminal(ctx, environment, p)
	if err != nil {
		return originalInputState{AgentState: before}, err
	}
	input, err := queryNativeInput(ctx, environment, p.RuntimeID, before.ProviderSession.ID, before.ProviderSession.TranscriptPath, baseline)
	if err != nil {
		return originalInputState{AgentState: before}, err
	}
	current, err := native.QueryOriginalAgent(ctx, environment, orcaStateResource(p), pane)
	after := originalInputState{AgentState: current}
	if err != nil {
		return after, err
	}
	if !sameWorkingInput(before, current) {
		return after, errors.New("original working input changed during native query")
	}
	confirmed, err := queryInputTerminal(ctx, environment, p)
	if err != nil {
		return after, err
	}
	if terminal.WorktreePath != confirmed.WorktreePath || terminal.Branch != confirmed.Branch {
		return after, originalInputIdentityError("original working input workspace changed during read")
	}
	frozen := current
	main, metadata := *current.MainAgent, *current.MetadataObservation
	frozen.MainAgent, frozen.MetadataObservation = &main, &metadata
	if current.Observation != nil {
		observation := *current.Observation
		frozen.Observation = &observation
	}
	input.query = &originalInputQuery{process: p, state: frozen, input: *input, working: true}
	after.NativeInput = input
	return after, nil
}

func (l *Lifecycle) queryWorkingOriginalStart(ctx context.Context, snapshot b.LifecycleSnapshot, state *State, turn *Turn, sample native.AgentState) (originalInputState, error) {
	before := originalInputState{AgentState: sample}
	if !l.profile.NativeTerminal || turn.Started || !turn.Accepted || l.queryInput == nil || turn.Native.agentName() != "codex" ||
		turn.Native.BaseRootAt <= 0 || turn.Native.Conflict != "" {
		return before, nil
	}
	if turn.Native.rejectIdentity(sample) || turn.Native.rejectProvenance(sample, turn) || turn.Native.rejectWorkingMetadata(sample) {
		return before, errors.New(turn.Native.diagnostic())
	}
	if !corroboratedWorkingInput(sample) || sample.MainAgent.StateStartedAt <= turn.Native.BaseRootAt {
		return before, nil
	}
	current, err := l.queryInput(ctx, state.Process, turn.Native.Pane, turn.Native.BaseRootAt, sample)
	if err != nil {
		if errors.Is(err, native.ErrStateIdentityChanged) {
			turn.Native.reject(err.Error())
		} else if current.MainAgent == nil || current.UpdatedAt >= current.MainAgent.StateStartedAt {
			turn.Native.rejectIdentity(current.AgentState)
			turn.Native.rejectProvenance(current.AgentState, turn)
			turn.Native.rejectWorkingMetadata(current.AgentState)
		}
		if turn.Native.Conflict != "" {
			return current, errors.New(turn.Native.diagnostic())
		}
		return current, err
	}
	if turn.Native.rejectIdentity(current.AgentState) || turn.Native.rejectProvenance(current.AgentState, turn) || turn.Native.rejectWorkingMetadata(current.AgentState) {
		return current, errors.New(turn.Native.diagnostic())
	}
	i := current.NativeInput
	s := current.AgentState
	o := s.MetadataObservation
	if i == nil || !i.matchesWorkingQuery(state.Process, s) || i.Baseline != turn.Native.BaseRootAt ||
		i.Timestamp <= i.Baseline || i.Timestamp > s.UpdatedAt || i.Timestamp > o.ObservedAt ||
		i.Text != nativePromptText(turn.Prompt) || s.Prompt != preview(turn.Prompt) ||
		s.PaneKey != turn.Native.Pane || s.ProviderSession.ID != turn.Native.Provider ||
		o.Revision <= turn.Native.BaseRevision || o.Revision < turn.Native.Revision ||
		turn.Native.BaseAuthority != "" && o.AuthorityID != turn.Native.BaseAuthority ||
		turn.Native.Authority != "" && (o.AuthorityID != turn.Native.Authority || o.Incarnation != turn.Native.Incarnation) {
		return current, nil
	}
	if _, err := l.original(ctx, snapshot); err != nil {
		return current, err
	}
	turn.Started = true
	return current, nil
}
