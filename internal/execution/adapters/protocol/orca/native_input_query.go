package orca

import (
	"context"
	"encoding/json"
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
		i.Timestamp == i.query.input.Timestamp && i.Baseline == i.query.input.Baseline && sameThinCompletion(i.query.state, s)
}

func (i *NativeInput) MatchesOriginalProcess(p b.ProcessIdentity) bool {
	return i != nil && i.query != nil && i.query.process == p
}

func sameThinCompletion(a, s AgentState) bool {
	return a.ThinMetadata && s.ThinMetadata && a.MainAgent != nil && s.MainAgent != nil &&
		a.State == "done" && s.State == "done" && a.MainAgent.State == "done" && *a.MainAgent == *s.MainAgent && a.AgentType == "codex" && s.AgentType == a.AgentType &&
		a.PaneKey != "" && a.PaneKey == s.PaneKey && a.ProviderSession == s.ProviderSession &&
		a.ProviderSession.ID != "" && a.ProviderSession.TranscriptPath != "" &&
		a.Prompt == "" && s.Prompt == "" && a.UpdatedAt == s.UpdatedAt && s.UpdatedAt >= s.MainAgent.StateStartedAt &&
		a.LastAssistantMessage == s.LastAssistantMessage && a.TurnStartedAt == 0 && s.TurnStartedAt == 0 &&
		a.Observation == nil && s.Observation == nil && a.MetadataObservation == nil && s.MetadataObservation == nil &&
		!a.SessionBoundary && !s.SessionBoundary && !a.RestoredUnconfirmed && !s.RestoredUnconfirmed &&
		!a.Interrupted && !s.Interrupted && !a.ToolOutput && !s.ToolOutput
}

func queryInputTerminal(ctx context.Context, environment string, p b.ProcessIdentity) (Terminal, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return Terminal{}, stateIdentityError("original native input terminal identity is incomplete")
	}
	socket, err := openStateConnection(ctx, environment, p.RuntimeID)
	if err != nil {
		return Terminal{}, err
	}
	defer socket.close()
	raw, err := callState(ctx, socket, "terminal.show", map[string]string{"terminal": p.Terminal})
	if err != nil {
		return Terminal{}, err
	}
	var result struct{ Terminal Terminal }
	if json.Unmarshal(raw, &result) != nil {
		return Terminal{}, errors.New("Orca original native input terminal query unavailable")
	}
	t := result.Terminal
	if t.Handle != p.Terminal || t.IncarnationID != p.Incarnation || t.WorktreeID != p.WorktreeID || !t.Connected || t.Exited() || t.AgentIdentity != "codex" || t.WorktreePath == "" || t.Branch == "" {
		return t, stateIdentityError("original native input terminal was replaced or disconnected")
	}
	return t, nil
}

func queryThinOriginalInput(ctx context.Context, environment string, p b.ProcessIdentity, pane string, baseline int64, before AgentState) (AgentState, error) {
	if !sameThinCompletion(before, before) || baseline <= 0 || before.MainAgent.StateStartedAt <= baseline {
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
	if !sameThinCompletion(before, after) {
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
	input.query = &originalInputQuery{process: p, state: frozen, input: *input}
	after.NativeInput = input
	return after, nil
}
