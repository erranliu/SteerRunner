package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// originalInputState keeps execution evidence outside Orca's raw state values.
type originalInputState struct {
	native.AgentState
	NativeInput *nativeInput `json:"-"`
}

func originalInputIdentityError(message string) error {
	return fmt.Errorf("%w: %s", native.ErrStateIdentityChanged, message)
}

// nativeInput is read from Orca's existing transcript RPC, never from a screen
// or hook preview. It corroborates input attribution, not turn completion.
type nativeInput struct {
	// Transient public pages for diagnostics; never added to persisted input
	// identity or model text. The original bounded query owns their lifecycle.
	Pages       []json.RawMessage `json:"-"`
	ID          string
	Text        string
	Timestamp   int64
	Baseline    int64
	Observation *native.StateObservation
	query       *originalInputQuery
}

// queryNativeInput applies the original baseline/unique-user contract to raw pages.
func queryNativeInput(ctx context.Context, environment, runtime, provider, path string, baseline int64) (*nativeInput, error) {
	if runtime == "" || provider == "" || path == "" || baseline <= 0 {
		return nil, errors.New("Orca original native input identity unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	reader, err := native.OpenSessionReader(ctx, environment, runtime, native.SessionReadOptions{Agent: "codex", SessionID: provider, TranscriptPath: path, Limit: 100, MaxPages: 16})
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var input *nativeInput
	pages := []json.RawMessage{}
	for page := 0; page < 16; page++ {
		value, err := reader.Next()
		if err != nil {
			return nil, err
		}
		pages = append(pages, value.Raw)
		messages := value.Messages
		for i := len(messages) - 1; i >= 0; i-- {
			m := messages[i]
			if m.Role != "user" {
				continue
			}
			if m.Source != "transcript" || m.ID == "" || m.Timestamp <= 0 || m.Shortened || len(m.Blocks) != 1 || m.Blocks[0].Type != "text" || strings.TrimSpace(m.Blocks[0].Text) == "" {
				return nil, errors.New("Orca latest native input is incomplete")
			}
			if m.Timestamp <= baseline {
				if input == nil {
					return nil, errors.New("Orca original native input absent after baseline")
				}
				input.Pages = pages
				return input, nil
			}
			if input != nil {
				return nil, errors.New("Orca another native input follows the original baseline")
			}
			input = &nativeInput{ID: m.ID, Text: m.Blocks[0].Text, Timestamp: m.Timestamp, Baseline: baseline}
		}
		if !value.HasMore {
			if input != nil {
				input.Pages = pages
				return input, nil
			}
			return nil, errors.New("Orca original native input absent")
		}
	}
	return nil, errors.New("Orca original native input exceeds query budget")
}

// queryOriginalInput cross-checks a bounded native transcript read against the
// exact pane/provider/main record before and after it. A SessionStart caused by
// compaction may change the display marker; it cannot create another user row.
func queryOriginalInput(ctx context.Context, environment string, process b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (originalInputState, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if corroboratedWorkingInput(before) {
		return queryFencedWorkingInput(ctx, environment, process, pane, baseline, before)
	}
	if before.MetadataObservation == nil || canQueryCompletionInput(before) {
		return queryFencedCompletionInput(ctx, environment, process, pane, baseline, before)
	}
	input, err := queryNativeInput(ctx, environment, process.RuntimeID, before.ProviderSession.ID, before.ProviderSession.TranscriptPath, baseline)
	if err != nil {
		return originalInputState{AgentState: before}, err
	}
	current, err := native.QueryOriginalAgent(ctx, environment, orcaStateResource(process), pane)
	after := originalInputState{AgentState: current}
	if err != nil {
		return after, err
	}
	if before.PaneKey != after.PaneKey || before.AgentType != after.AgentType || before.ProviderSession.ID != after.ProviderSession.ID {
		return after, originalInputIdentityError("original native input pane/agent/provider changed during read")
	}
	if after.MetadataObservation != nil && (before.MetadataObservation.AuthorityID != after.MetadataObservation.AuthorityID || before.MetadataObservation.Incarnation != after.MetadataObservation.Incarnation) {
		return after, originalInputIdentityError("original native input authority changed during read")
	}
	if after.MetadataObservation != nil && after.MetadataObservation.Revision < before.MetadataObservation.Revision {
		return after, errors.New("Orca original native input observation moved backwards during read")
	}
	if after.MetadataObservation == nil || before.ProviderSession != after.ProviderSession || before.MainAgent == nil || after.MainAgent == nil || *before.MainAgent != *after.MainAgent || before.TurnStartedAt != after.TurnStartedAt || after.SessionBoundary || after.RestoredUnconfirmed || after.Interrupted || after.ToolOutput || after.Prompt != before.Prompt {
		return after, errors.New("Orca original input changed during native transcript query")
	}
	after.NativeInput = input
	input.Observation = after.MetadataObservation
	return after, nil
}

// This is query evidence, not a fabricated hook observation. It is transient,
// cannot be decoded from the wire, and never supplies an assistant result.
type originalInputQuery struct {
	process b.ProcessIdentity
	state   native.AgentState
	input   nativeInput
	working bool
}

// MatchesOriginalQuery binds a unique user row to the exact independently
// queried native completion. A copied proof cannot authorize another record.
func (i *nativeInput) MatchesOriginalQuery(s native.AgentState) bool {
	return i != nil && i.query != nil && !i.query.working && i.ID == i.query.input.ID && i.Text == i.query.input.Text &&
		i.Timestamp == i.query.input.Timestamp && i.Baseline == i.query.input.Baseline && sameOriginalCompletion(i.query.state, s)
}

func (i *nativeInput) MatchesOriginalProcess(p b.ProcessIdentity) bool {
	return i != nil && i.query != nil && i.query.process == p
}

func sameThinCompletion(a, s native.AgentState) bool {
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
func canQueryCompletionInput(s native.AgentState) bool {
	return sameThinCompletion(s, s) || sameCorroboratedCompletion(s, s)
}

func corroboratedCompletion(s native.AgentState) bool {
	o := s.MetadataObservation
	return !s.ThinMetadata && s.MainAgent != nil && s.State == "done" && s.MainAgent.State == "done" &&
		s.AgentType == "codex" && s.PaneKey != "" && s.ProviderSession.ID != "" && s.ProviderSession.TranscriptPath != "" &&
		s.UpdatedAt >= s.MainAgent.StateStartedAt && s.MainAgent.StateStartedAt > 0 &&
		s.TurnStartedAt > 0 && s.TurnStartedAt <= s.MainAgent.StateStartedAt &&
		o.ValidHookTransition() && o.ObservedAt >= s.MainAgent.StateStartedAt &&
		(s.Observation == nil || *s.Observation == *o) &&
		!s.SessionBoundary && !s.RestoredUnconfirmed && !s.Interrupted && !s.ToolOutput
}

func sameCorroboratedCompletion(a, s native.AgentState) bool {
	return corroboratedCompletion(a) && corroboratedCompletion(s) &&
		a.PaneKey == s.PaneKey && a.ProviderSession == s.ProviderSession && *a.MainAgent == *s.MainAgent &&
		a.Prompt == s.Prompt && a.UpdatedAt == s.UpdatedAt && a.TurnStartedAt == s.TurnStartedAt && a.LastAssistantMessage == s.LastAssistantMessage &&
		*a.MetadataObservation == *s.MetadataObservation &&
		(a.Observation == nil && s.Observation == nil || a.Observation != nil && s.Observation != nil && *a.Observation == *s.Observation)
}

func sameOriginalCompletion(a, s native.AgentState) bool {
	return sameThinCompletion(a, s) || sameCorroboratedCompletion(a, s)
}

func queryInputTerminal(ctx context.Context, environment string, p b.ProcessIdentity) (native.Terminal, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return native.Terminal{}, originalInputIdentityError("original native input terminal identity is incomplete")
	}
	t, err := native.QueryBoundTerminal(ctx, environment, native.TerminalIdentity{RuntimeID: p.RuntimeID, Handle: p.Terminal, IncarnationID: p.Incarnation})
	if err != nil {
		return native.Terminal{}, err
	}
	if t.Handle != p.Terminal || t.IncarnationID != p.Incarnation || t.WorktreeID != p.WorktreeID || !t.Connected || t.Exited() || t.AgentIdentity != "codex" || t.WorktreePath == "" || t.Branch == "" {
		return t, originalInputIdentityError("original native input terminal was replaced or disconnected")
	}
	return t, nil
}

func queryFencedCompletionInput(ctx context.Context, environment string, p b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (originalInputState, error) {
	if !canQueryCompletionInput(before) || baseline <= 0 || before.MainAgent.StateStartedAt <= baseline {
		return originalInputState{AgentState: before}, errors.New("Orca original main-agent metadata does not corroborate native input")
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
	if !sameOriginalCompletion(before, after.AgentState) {
		return after, errors.New("Orca original completion changed during native input query")
	}
	confirmed, err := queryInputTerminal(ctx, environment, p)
	if err != nil {
		return after, err
	}
	if terminal.WorktreePath != confirmed.WorktreePath || terminal.Branch != confirmed.Branch {
		return after, originalInputIdentityError("original native input workspace changed during read")
	}
	frozen := after.AgentState
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
