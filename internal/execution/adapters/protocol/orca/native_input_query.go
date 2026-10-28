package orca

import (
	"context"
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// This is query evidence, not a fabricated hook observation. It is transient,
// cannot be decoded from the wire, and never supplies an assistant result.
type originalInputQuery struct {
	process b.ProcessIdentity
	state   AgentState
	input   NativeInput
}

// MatchesOriginalQuery binds a unique user row to the exact independently
// queried native completion. A copied proof cannot authorize another record.
func (i *NativeInput) MatchesOriginalQuery(s AgentState) bool {
	return i != nil && i.query != nil && i.ID == i.query.input.ID && i.Text == i.query.input.Text &&
		i.Timestamp == i.query.input.Timestamp && i.Baseline == i.query.input.Baseline && sameOriginalCompletion(i.query.state, s)
}

func (i *NativeInput) MatchesOriginalProcess(p b.ProcessIdentity) bool {
	return i != nil && i.query != nil && i.query.process == p
}

func sameThinCompletion(a, s AgentState) bool {
	return a.ThinMetadata && s.ThinMetadata && a.MainAgent != nil && s.MainAgent != nil &&
		a.State == "done" && s.State == "done" && a.MainAgent.State == "done" && *a.MainAgent == *s.MainAgent && a.AgentType == "codex" && s.AgentType == a.AgentType &&
		a.PaneKey != "" && a.PaneKey == s.PaneKey && a.ProviderSession == s.ProviderSession &&
		a.ProviderSession.ID != "" && a.ProviderSession.TranscriptPath != "" &&
		a.Prompt == s.Prompt && a.UpdatedAt == s.UpdatedAt && s.UpdatedAt >= s.MainAgent.StateStartedAt &&
		a.LastAssistantMessage == s.LastAssistantMessage && a.TurnStartedAt == 0 && s.TurnStartedAt == 0 &&
		a.Observation == nil && s.Observation == nil && a.MetadataObservation == nil && s.MetadataObservation == nil &&
		!a.SessionBoundary && !s.SessionBoundary && !a.RestoredUnconfirmed && !s.RestoredUnconfirmed &&
		!a.Interrupted && !s.Interrupted && !a.ToolOutput && !s.ToolOutput
}

// CanQueryCompletionInput admits only the existing thin projection or a fully
// corroborated metadata record. Partial metadata is never promoted to thin.
// The actual input must still be read uniquely after the frozen baseline and
// fenced by the exact terminal/process before and after that read.
func (s AgentState) CanQueryCompletionInput() bool {
	return sameThinCompletion(s, s) || sameCorroboratedCompletion(s, s)
}

func corroboratedCompletion(s AgentState) bool {
	o := s.MetadataObservation
	return !s.ThinMetadata && s.MainAgent != nil && s.State == "done" && s.MainAgent.State == "done" &&
		s.AgentType == "codex" && s.PaneKey != "" && s.ProviderSession.ID != "" && s.ProviderSession.TranscriptPath != "" &&
		s.UpdatedAt >= s.MainAgent.StateStartedAt && s.MainAgent.StateStartedAt > 0 &&
		s.TurnStartedAt > 0 && s.TurnStartedAt <= s.MainAgent.StateStartedAt &&
		o.ValidHookTransition() && o.ObservedAt >= s.MainAgent.StateStartedAt &&
		(s.Observation == nil || *s.Observation == *o) &&
		!s.SessionBoundary && !s.RestoredUnconfirmed && !s.Interrupted && !s.ToolOutput
}

func sameCorroboratedCompletion(a, s AgentState) bool {
	return corroboratedCompletion(a) && corroboratedCompletion(s) &&
		a.PaneKey == s.PaneKey && a.ProviderSession == s.ProviderSession && *a.MainAgent == *s.MainAgent &&
		a.Prompt == s.Prompt && a.UpdatedAt == s.UpdatedAt && a.TurnStartedAt == s.TurnStartedAt && a.LastAssistantMessage == s.LastAssistantMessage &&
		*a.MetadataObservation == *s.MetadataObservation &&
		(a.Observation == nil && s.Observation == nil || a.Observation != nil && s.Observation != nil && *a.Observation == *s.Observation)
}

func sameOriginalCompletion(a, s AgentState) bool {
	return sameThinCompletion(a, s) || sameCorroboratedCompletion(a, s)
}

func queryInputTerminal(ctx context.Context, environment string, p b.ProcessIdentity) (Terminal, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return Terminal{}, stateIdentityError("original native input terminal identity is incomplete")
	}
	t, err := queryTerminal(ctx, environment, p)
	if err != nil {
		return Terminal{}, err
	}
	if t.Handle != p.Terminal || t.IncarnationID != p.Incarnation || t.WorktreeID != p.WorktreeID || !t.Connected || t.Exited() || t.AgentIdentity != "codex" || t.WorktreePath == "" || t.Branch == "" {
		return t, stateIdentityError("original native input terminal was replaced or disconnected")
	}
	return t, nil
}

func queryFencedCompletionInput(ctx context.Context, environment string, p b.ProcessIdentity, pane string, baseline int64, before AgentState) (AgentState, error) {
	if !before.CanQueryCompletionInput() || baseline <= 0 || before.MainAgent.StateStartedAt <= baseline {
		return before, errors.New("Orca original main-agent metadata does not corroborate native input")
	}
	terminal, err := queryInputTerminal(ctx, environment, p)
	if err != nil {
		return before, err
	}
	input, err := QueryNativeInput(ctx, environment, p.RuntimeID, before.ProviderSession.ID, before.ProviderSession.TranscriptPath, baseline)
	if err != nil {
		return before, err
	}
	after, err := QueryOriginalAgent(ctx, environment, p, pane)
	if err != nil {
		return after, err
	}
	if !sameOriginalCompletion(before, after) {
		return after, errors.New("Orca original completion changed during native input query")
	}
	confirmed, err := queryInputTerminal(ctx, environment, p)
	if err != nil {
		return after, err
	}
	if terminal.WorktreePath != confirmed.WorktreePath || terminal.Branch != confirmed.Branch {
		return after, stateIdentityError("original native input workspace changed during read")
	}
	frozen := after
	main := *after.MainAgent
	frozen.MainAgent = &main
	if after.MetadataObservation != nil {
		metadata := *after.MetadataObservation
		frozen.MetadataObservation = &metadata
	}
	if after.Observation != nil {
		observation := *after.Observation
		frozen.Observation = &observation
	}
	input.query = &originalInputQuery{process: p, state: frozen, input: *input}
	after.NativeInput = input
	return after, nil
}
