package orca

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// NativeInput is read from Orca's existing transcript RPC, never from a screen
// or hook preview. It corroborates input attribution, not turn completion.
type NativeInput struct {
	// Transient public pages for diagnostics; never added to persisted input
	// identity or model text. The original bounded query owns their lifecycle.
	Pages       []json.RawMessage `json:"-"`
	ID          string
	Text        string
	Timestamp   int64
	Baseline    int64
	Observation *StateObservation
	query       *originalInputQuery
}

// QueryNativeInput walks backwards to the frozen baseline and requires exactly
// one user input since it. A repeated identical prompt is another input too.
func QueryNativeInput(ctx context.Context, environment, runtime, provider, path string, baseline int64) (*NativeInput, error) {
	if runtime == "" || provider == "" || path == "" || baseline <= 0 {
		return nil, errors.New("Orca original native input identity unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return nil, err
	}
	defer socket.close()
	params := map[string]any{"agent": "codex", "sessionId": provider, "transcriptPath": path, "limit": 100}
	previous := int64(-1)
	var input *NativeInput
	pages := []json.RawMessage{}
	for page := 0; page < 16; page++ {
		raw, err := callState(ctx, socket, "nativeChat.readSession", params)
		if err != nil {
			return nil, err
		}
		pages = append(pages, append(json.RawMessage(nil), raw...))
		var value struct {
			Messages *[]struct {
				ID, Role, Source string
				Timestamp        int64
				Shortened        bool
				Blocks           []struct{ Type, Text string }
			}
			HasMore      *bool
			BeforeOffset *int64
			Error        string
		}
		if json.Unmarshal(raw, &value) != nil || value.Messages == nil || value.HasMore == nil || value.BeforeOffset == nil || value.Error != "" {
			return nil, errors.New("Orca original native input page unavailable")
		}
		// Validate the page even when it contains the sought input/baseline;
		// an early successful match cannot repair an invalid pagination boundary.
		if *value.BeforeOffset < 0 || *value.HasMore && (*value.BeforeOffset == 0 || previous >= 0 && *value.BeforeOffset >= previous) {
			return nil, errors.New("Orca original native input pagination did not advance")
		}
		messages := *value.Messages
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
			input = &NativeInput{ID: m.ID, Text: m.Blocks[0].Text, Timestamp: m.Timestamp, Baseline: baseline}
		}
		if !*value.HasMore {
			if input != nil {
				input.Pages = pages
				return input, nil
			}
			return nil, errors.New("Orca original native input absent")
		}
		previous = *value.BeforeOffset
		params["beforeOffset"] = previous
	}
	return nil, errors.New("Orca original native input exceeds query budget")
}

// QueryOriginalInput cross-checks a bounded native transcript read against the
// exact pane/provider/main record before and after it. A SessionStart caused by
// compaction may change the display marker; it cannot create another user row.
func QueryOriginalInput(ctx context.Context, environment string, process b.ProcessIdentity, pane string, baseline int64, before AgentState) (AgentState, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if before.MetadataObservation == nil || before.CanQueryCompletionInput() {
		return queryFencedCompletionInput(ctx, environment, process, pane, baseline, before)
	}
	input, err := QueryNativeInput(ctx, environment, process.RuntimeID, before.ProviderSession.ID, before.ProviderSession.TranscriptPath, baseline)
	if err != nil {
		return before, err
	}
	after, err := QueryOriginalAgent(ctx, environment, process, pane)
	if err != nil {
		return after, err
	}
	if before.PaneKey != after.PaneKey || before.AgentType != after.AgentType || before.ProviderSession.ID != after.ProviderSession.ID {
		return after, stateIdentityError("original native input pane/agent/provider changed during read")
	}
	if after.MetadataObservation != nil && (before.MetadataObservation.AuthorityID != after.MetadataObservation.AuthorityID || before.MetadataObservation.Incarnation != after.MetadataObservation.Incarnation) {
		return after, stateIdentityError("original native input authority changed during read")
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
