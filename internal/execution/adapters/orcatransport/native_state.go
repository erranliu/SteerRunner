package orcatransport

import (
	"strings"
	"time"
	"unicode/utf16"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

type NativeProof struct {
	Agent         string    `json:"agent,omitempty"`
	Pane          string    `json:"pane"`
	Provider      string    `json:"provider,omitempty"`
	BaseAuthority string    `json:"base_authority,omitempty"`
	BaseRevision  int64     `json:"base_revision,omitempty"`
	BaseRootAt    int64     `json:"base_root_at,omitempty"`
	Authority     string    `json:"authority,omitempty"`
	Incarnation   int64     `json:"incarnation,omitempty"`
	Revision      int64     `json:"revision,omitempty"`
	WorkingAt     int64     `json:"working_at,omitempty"`
	RootWorkingAt int64     `json:"root_working_at,omitempty"`
	EndedAt       int64     `json:"ended_at,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
	ReplyDigest   string    `json:"reply_digest,omitempty"`
	ReplyBody     string    `json:"reply_body,omitempty"`
	LastSignal    time.Time `json:"last_signal,omitempty"`
	LastQuery     time.Time `json:"last_query,omitempty"`
	Hint          string    `json:"hint,omitempty"`
	HintPending   bool      `json:"hint_pending,omitempty"`
	// Conflict is durable contrary evidence; Waiting is a recoverable evidence gap.
	Conflict string `json:"conflict,omitempty"`
	Waiting  string `json:"waiting,omitempty"`
}

func (p NativeProof) diagnostic() string {
	if p.Conflict != "" {
		return p.Conflict
	}
	return p.Waiting
}

func (p *NativeProof) reject(message string) {
	if p.Conflict == "" {
		p.Conflict = message
	}
	p.Waiting = ""
}

func (p NativeProof) hasFinal() bool {
	return p.EndedAt > 0 && p.ReplyDigest != ""
}

func (p *NativeProof) incomplete(s native.AgentState, turn *Turn, linked bool, input *nativeInput) bool {
	// Missing display fields remain recoverable. Empty end previews need either
	// current observer coverage or an independently queried retained input stamp.
	if s.PaneKey == "" || s.AgentType == "" || s.ProviderSession.ID == "" || s.MainAgent == nil {
		// Missing display metadata cannot revoke an already verified final.
		// Positive conflicts are checked independently before availability.
		if !p.hasFinal() {
			p.Waiting = "Native original input metadata is incomplete; awaiting original evidence"
		}
		return true
	}
	issue := p.attribution(s, turn, linked, input)
	if issue.kind == provenanceMissing && !p.hasFinal() {
		p.Waiting = issue.diagnostic
		return true
	}
	return false
}

func (p *NativeProof) rejectIdentity(s native.AgentState) bool {
	if s.PaneKey != "" && s.PaneKey != p.Pane || s.AgentType != "" && s.AgentType != p.agentName() || p.Provider != "" && s.ProviderSession.ID != "" && s.ProviderSession.ID != p.Provider {
		p.reject("Native original agent/pane/provider changed")
		return true
	}
	if o := s.MetadataObservation; o != nil && canQueryCompletionInput(s) {
		if p.BaseAuthority != "" && o.AuthorityID != p.BaseAuthority {
			p.reject("Native baseline authority changed")
			return true
		}
		if p.Authority != "" && (o.AuthorityID != p.Authority || o.Incarnation != p.Incarnation) {
			p.reject("Native main-agent authority changed")
			return true
		}
	}
	return false
}

func trustedState(s native.AgentState) bool {
	o := s.Observation
	return s.AgentType != "" && s.MainAgent != nil && o.ValidHookTransition()
}

func (p NativeProof) agentName() string {
	if p.Agent == "" {
		return "codex"
	}
	return p.Agent
}
func preview(text string) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	count, end := 0, 0
	for end < len(runes) {
		n := 1
		if runes[end] > 0xffff {
			n = 2
		}
		if count+n > 200 {
			break
		}
		count += n
		end++
	}
	return string(runes[:end])
}

func (p *NativeProof) body(message string) {
	// The public Orca hook field is capped at 8000 UTF-16 code units. Reject
	// the surrogate-safe truncation boundary too; never treat a prefix as full.
	if strings.TrimSpace(message) == "" || len(utf16.Encode([]rune(message))) >= 7999 {
		return
	}
	body := message
	hash := digest([]byte(body))
	// The first complete native original is immutable; no display normalization.
	if p.ReplyDigest != "" && (p.ReplyDigest != hash || p.ReplyBody != body) {
		p.reject("Native ended reply changed; current result is unverified")
		return
	}
	p.ReplyDigest = hash
	p.ReplyBody = body
}

func (p *NativeProof) rejectProvenance(s native.AgentState, turn *Turn) bool {
	if s.MainAgent != nil && s.MainAgent.StateStartedAt > p.BaseRootAt && (s.MainAgent.State == "working" || s.MainAgent.State == "done") &&
		p.attribution(s, turn, false, nil).kind == provenanceConflict {
		p.reject("Native original completion/input provenance is unverified")
		return true
	}
	if s.MainAgent != nil && s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > p.BaseRootAt &&
		(s.Interrupted || s.MainAgent.Outcome != "" && s.MainAgent.Outcome != "success" || p.agentName() != "codex" && s.MainAgent.Outcome != "success") {
		p.reject("Native interruption/outcome cannot verify ordinary completion")
		return true
	}
	return false
}
func (p *NativeProof) end(s native.AgentState, turn *Turn, now time.Time, linked bool, input *nativeInput) bool {
	if !turn.Accepted || !turn.Started || p.Provider == "" || s.MainAgent == nil || s.MainAgent.State != "done" || s.MainAgent.StateStartedAt <= p.WorkingAt || s.MainAgent.StateStartedAt <= p.BaseRootAt || p.WorkingAt == 0 && p.BaseRootAt <= 0 {
		return false
	}
	if p.rejectIdentity(s) || p.rejectProvenance(s, turn) {
		return false
	}
	if p.incomplete(s, turn, linked, input) {
		return false
	}
	if s.ToolOutput {
		p.Waiting = "Native original final body is tool output; awaiting original assistant evidence"
		return false
	}
	if p.EndedAt != 0 && p.EndedAt != s.MainAgent.StateStartedAt {
		p.reject("Native original completion record changed")
		return false
	}
	if p.EndedAt == 0 {
		p.EndedAt = s.MainAgent.StateStartedAt
		p.LastSignal = now
		// Do not reinterpret any already persisted outcome, including unknown.
		p.Outcome = "unreported"
		if s.MainAgent.Outcome != "" {
			p.Outcome = s.MainAgent.Outcome
		}
	}
	p.Waiting = ""
	p.body(s.LastAssistantMessage)
	return true
}
func (p *NativeProof) query(s native.AgentState, turn *Turn, now time.Time, linked bool, input *nativeInput) bool {
	if p.Conflict != "" {
		return false
	}
	if s.MainAgent != nil && s.UpdatedAt < s.MainAgent.StateStartedAt {
		return false
	}
	if p.rejectIdentity(s) {
		return false
	}
	if p.rejectProvenance(s, turn) {
		return false
	}
	if s.MainAgent == nil {
		p.incomplete(s, turn, linked, input)
		return false
	}
	if s.MainAgent.State == "done" {
		return p.end(s, turn, now, linked || p.retainsOriginalInput(s), input)
	}
	if p.hasFinal() {
		return false
	}
	if p.incomplete(s, turn, linked, input) {
		return false
	}
	if s.MainAgent.State == "waiting" || s.MainAgent.State == "blocked" {
		p.Waiting = "Native main agent is waiting; no next input was sent"
	} else {
		p.Waiting = ""
	}
	return false
}

// A reconnect loses notification coverage, but not the persisted, prompt-matched
// working root. Orca's positive turnStartedAt stamp is set by the original input
// hook, retained through tools/done, and reset by another input. The independent
// query cross-checks this stamp against the same main-agent state/time in session
// metadata. It may therefore rebind that exact root without claiming continuity.
// Initial snapshots and notifications never acquire this query-only permission.
func (p NativeProof) retainsOriginalInput(s native.AgentState) bool {
	if p.agentName() != "codex" || p.Authority == "" || p.WorkingAt <= 0 || p.RootWorkingAt <= p.BaseRootAt ||
		p.RootWorkingAt <= 0 || s.TurnStartedAt != p.RootWorkingAt || s.PaneKey != p.Pane ||
		s.AgentType != p.agentName() || p.Provider == "" || s.ProviderSession.ID != p.Provider ||
		s.MainAgent == nil || s.MainAgent.State != "done" || s.MainAgent.StateStartedAt <= p.RootWorkingAt {
		return false
	}
	// Older hook-store projections omit observation metadata. When present, it
	// must corroborate the persisted authority rather than mask a replacement.
	return s.Observation == nil || trustedState(s) && s.Observation.AuthorityID == p.Authority &&
		s.Observation.Incarnation == p.Incarnation && s.Observation.Revision >= p.Revision && s.Observation.Revision > p.BaseRevision
}
func (p NativeProof) recordedInputMatches(s native.AgentState, turn *Turn, input *nativeInput) bool {
	i := input
	if i == nil || !turn.Accepted || !turn.Started || p.agentName() != "codex" || p.BaseRootAt <= 0 ||
		i.Baseline != p.BaseRootAt || i.ID == "" || i.Timestamp <= p.BaseRootAt || i.Timestamp > s.UpdatedAt ||
		i.Text != nativePromptText(turn.Prompt) ||
		s.PaneKey != p.Pane || s.ProviderSession.ID == "" || s.ProviderSession.ID != p.Provider || s.AgentType != p.agentName() ||
		s.MainAgent == nil || s.SessionBoundary || s.RestoredUnconfirmed || s.Interrupted || s.ToolOutput || p.Conflict != "" {
		return false
	}
	o := i.Observation
	if i.MatchesOriginalQuery(s) {
		if o := s.MetadataObservation; o != nil && (o.Revision <= p.BaseRevision || p.Authority != "" && o.Revision < p.Revision) {
			return false
		}
		// A separate terminal-fenced, unique-input query corroborates this
		// exact canonical done record, with either absent metadata or fully
		// matching metadata. It cannot authorize starts, streams or another end.
		// The independently fenced original-input read can also corroborate a
		// done record whose streamed working notification was never received.
		return s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > p.BaseRootAt && i.Timestamp <= s.MainAgent.StateStartedAt &&
			(p.WorkingAt == 0 || p.RootWorkingAt > p.BaseRootAt && s.MainAgent.StateStartedAt > p.RootWorkingAt)
	}
	if p.Authority == "" || p.WorkingAt <= 0 || p.RootWorkingAt <= p.BaseRootAt ||
		!o.ValidHookTransition() || o.ObservedAt < s.MainAgent.StateStartedAt || o.AuthorityID != p.Authority || o.Incarnation != p.Incarnation || o.Revision < p.Revision || o.Revision <= p.BaseRevision {
		return false
	}
	return s.MainAgent.State == "working" && s.MainAgent.StateStartedAt == p.RootWorkingAt ||
		s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > p.RootWorkingAt
}
