package orcatransport

import (
	"encoding/json"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// originalInputState keeps execution evidence outside Orca's raw state values.
type originalInputState struct {
	native.AgentState
	NativeInput *nativeInput `json:"-"`
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

// queryOriginalInput cross-checks a bounded native transcript read against the
// exact pane/provider/main record before and after it. A SessionStart caused by
// compaction may change the display marker; it cannot create another user row.

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
