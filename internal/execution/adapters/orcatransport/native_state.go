package orcatransport

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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

func (p *NativeProof) incomplete(s native.AgentState, turn *Turn, linked bool) bool {
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
	issue := p.attribution(s, turn, linked)
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
	if o := s.MetadataObservation; o != nil && s.CanQueryCompletionInput() {
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

func stateHint(s native.AgentState) string {
	return s.PaneKey + ":" + s.State + ":" + strconv.FormatInt(s.StateStartedAt, 10)
}
func nativeBaseline(s native.AgentState, now time.Time) NativeProof {
	p := NativeProof{Pane: s.PaneKey, Provider: s.ProviderSession.ID, LastSignal: now, LastQuery: now, Hint: stateHint(s)}
	if s.AgentType != "codex" {
		p.Agent = s.AgentType
	}
	if trustedState(s) {
		p.BaseAuthority = s.Observation.AuthorityID
		p.BaseRevision = s.Observation.Revision
	}
	if s.MainAgent != nil {
		p.BaseRootAt = s.MainAgent.StateStartedAt
	}
	return p
}
func trustedState(s native.AgentState) bool {
	o := s.Observation
	return s.AgentType != "" && s.MainAgent != nil && o != nil && o.Origin == "hook" && o.Kind == "transition" && o.AuthorityID != "" && o.Revision > 0 && o.ObservedAt > 0
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
func (p *NativeProof) signal(s native.AgentState, turn *Turn, now time.Time, linked bool) {
	if p.Conflict != "" {
		return
	}
	if p.rejectIdentity(s) {
		return
	}
	if stateHint(s) != p.Hint {
		p.Hint = stateHint(s)
		if s.State == "done" {
			p.HintPending = true
		}
	} // hint only; require a main-agent query
	if !trustedState(s) || !turn.Accepted {
		if turn.Accepted {
			p.incomplete(s, turn, linked)
		}
		return
	}
	o := s.Observation
	if p.Authority != "" && (o.AuthorityID != p.Authority || o.Incarnation != p.Incarnation) {
		p.reject("Native main-agent authority changed")
		return
	}
	if p.BaseAuthority != "" && o.AuthorityID != p.BaseAuthority {
		p.reject("Native baseline authority changed")
		return
	}
	if o.Revision <= p.BaseRevision || o.Revision <= p.Revision {
		return
	}
	if p.rejectProvenance(s, turn) {
		return
	}
	if p.WorkingAt > 0 && o.Boundary && s.MainAgent.State == "working" && s.MainAgent.StateStartedAt != p.RootWorkingAt {
		p.reject("Another native input boundary makes the original turn unverified")
		return
	}
	if p.hasFinal() && s.MainAgent.State != "done" {
		return // Later display availability cannot revoke the committed body.
	}
	if p.incomplete(s, turn, linked) {
		return
	}
	if p.WorkingAt == 0 {
		if s.MainAgent.State == "done" {
			// Without a streamed start, confirm the retained main-agent record
			// through the independent query; never promote a tab snapshot alone.
			return
		}
		if s.MainAgent.State != "working" || s.MainAgent.StateStartedAt <= p.BaseRootAt || !turn.Started || s.ProviderSession.ID == "" || s.Prompt != preview(turn.Prompt) {
			return
		}
		p.Authority, p.Incarnation, p.Provider = o.AuthorityID, o.Incarnation, s.ProviderSession.ID
		p.WorkingAt = o.ObservedAt
		p.RootWorkingAt = s.MainAgent.StateStartedAt
	}
	p.Revision, p.LastSignal = o.Revision, now
	if s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > p.WorkingAt {
		p.end(s, turn, now, linked)
	} else if s.MainAgent.State == "waiting" || s.MainAgent.State == "blocked" {
		// Waiting remains in the current turn; it is not a completion.
		p.Waiting = "Native main agent is waiting; no next input was sent"
	} else {
		p.Waiting = ""
	}
}
func (p *NativeProof) rejectProvenance(s native.AgentState, turn *Turn) bool {
	if s.MainAgent != nil && s.MainAgent.StateStartedAt > p.BaseRootAt && (s.MainAgent.State == "working" || s.MainAgent.State == "done") &&
		p.attribution(s, turn, false).kind == provenanceConflict {
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
func (p *NativeProof) end(s native.AgentState, turn *Turn, now time.Time, linked bool) bool {
	if !turn.Accepted || !turn.Started || p.Provider == "" || s.MainAgent == nil || s.MainAgent.State != "done" || s.MainAgent.StateStartedAt <= p.WorkingAt || s.MainAgent.StateStartedAt <= p.BaseRootAt || p.WorkingAt == 0 && p.BaseRootAt <= 0 {
		return false
	}
	if p.rejectIdentity(s) || p.rejectProvenance(s, turn) {
		return false
	}
	if p.incomplete(s, turn, linked) {
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
func (p *NativeProof) query(s native.AgentState, turn *Turn, now time.Time, linked bool) bool {
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
		p.incomplete(s, turn, linked)
		return false
	}
	if s.MainAgent.State == "done" {
		return p.end(s, turn, now, linked || p.retainsOriginalInput(s))
	}
	if p.hasFinal() {
		return false
	}
	if p.incomplete(s, turn, linked) {
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
func (p NativeProof) recordedInputMatches(s native.AgentState, turn *Turn) bool {
	i := s.NativeInput
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
		o == nil || o.Origin != "hook" || o.Kind != "transition" || o.ObservedAt < s.MainAgent.StateStartedAt || o.AuthorityID != p.Authority || o.Incarnation != p.Incarnation || o.Revision < p.Revision || o.Revision <= p.BaseRevision {
		return false
	}
	return s.MainAgent.State == "working" && s.MainAgent.StateStartedAt == p.RootWorkingAt ||
		s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > p.RootWorkingAt
}

func (p NativeProof) wantsQuery(now time.Time) bool {
	return (p.EndedAt == 0 || p.ReplyDigest == "") && (p.HintPending || !now.Before(p.LastQuery.Add(30*time.Second)))
}

func (l *Lifecycle) stateFeed(ctx context.Context, process b.ProcessIdentity) (native.StateFeed, error) {
	l.feedMu.Lock()
	defer l.feedMu.Unlock()
	if l.feed != nil {
		if l.feedProcess != process {
			return nil, errors.New("native state feed original resource changed")
		}
		return l.feed, nil
	}
	feed, err := l.stateFactory(ctx, l.profile.Environment, process)
	if err != nil {
		return nil, err
	}
	l.feed, l.feedProcess = feed, process
	return feed, nil
}
func (l *Lifecycle) closeStateFeed() {
	l.feedMu.Lock()
	defer l.feedMu.Unlock()
	if l.feed != nil {
		l.feed.Close()
		l.feed = nil
	}
}

func (l *Lifecycle) releaseStateFeed(feed native.StateFeed) {
	if feed == nil {
		return
	}
	l.feedMu.Lock()
	if l.feed == feed {
		l.feed = nil
	}
	l.feedMu.Unlock()
	feed.Close()
}

func (l *Lifecycle) observeNative(ctx context.Context, snapshot b.LifecycleSnapshot, state *State, turn *Turn) (diagnostic string) {
	l.observationMu.Lock()
	defer l.observationMu.Unlock()
	l.feedMu.Lock()
	feed := l.feed
	l.feedMu.Unlock()
	var streamErr error
	if feed == nil && !l.clock().Before(l.feedRetryAt) {
		feed, streamErr = l.stateFeed(ctx, state.Process)
		if streamErr != nil {
			l.feedRetryAt = l.clock().Add(30 * time.Second)
			if errors.Is(streamErr, native.ErrStateIdentityChanged) {
				turn.Native.reject(streamErr.Error())
			}
		} else {
			initial := feed.Initial().AgentState
			if stateHint(initial) != turn.Native.Hint && initial.State == "done" {
				turn.Native.Hint, turn.Native.HintPending = stateHint(initial), true
			}
		}
	}
	observation := l.observation(snapshot, feed)
	diagnostic = observation.diagnostic
	defer func() { observation.diagnostic = diagnostic }()
	drain := func() {
		if feed == nil {
			return
		}
		samples, err := feed.Drain()
		streamErr = err
		if err != nil {
			observation.invalidate()
		}
		for _, sample := range samples {
			s := sample.AgentState
			if trustedState(s) && s.Observation.AuthorityID == turn.Native.Authority && s.Observation.Incarnation == turn.Native.Incarnation && s.Observation.Revision > observation.observedRevision {
				observation.observedRevision = s.Observation.Revision
			}
			if sample.Kind == native.SampleInitial {
				observation.reattach(s, turn.Native, turn)
				if s.MainAgent != nil && s.MainAgent.StateStartedAt > turn.Native.BaseRootAt && (s.MainAgent.State == "working" || s.MainAgent.State == "done") {
					if issue := turn.Native.attribution(s, turn, false); issue.kind != provenanceMatched {
						diagnostic = issue.diagnostic
					}
				}
				if stateHint(s) != turn.Native.Hint && s.State == "done" {
					turn.Native.Hint, turn.Native.HintPending = stateHint(s), true
				}
				continue
			}
			prior := turn.Native.Revision
			hadStart := turn.Native.WorkingAt > 0
			observation.bindInput(&s, turn.Native)
			sample.AgentState = s
			linked := l.profile.Environment == "" && turn.Native.WorkingAt > 0 || err == nil && observation.linked(sample, turn.Native)
			turn.Native.signal(s, turn, received(sample, l.clock()), linked)
			issue := turn.Native.diagnostic()
			if issue != "" {
				diagnostic = issue
			} else if turn.Native.Revision > prior {
				diagnostic = ""
			}
			if sample.Kind == native.SampleNotification && trustedState(s) && !hadStart && turn.Native.WorkingAt > 0 && err == nil {
				observation.continuous = true
				observation.turnStartedAt = s.TurnStartedAt
				observation.lastDigest = digest([]byte(s.LastAssistantMessage))
				observation.pending = nil
			}
			if s.SessionBoundary || s.RestoredUnconfirmed || s.ToolOutput || s.Interrupted ||
				s.TurnStartedAt > 0 && observation.turnStartedAt > 0 && s.TurnStartedAt != observation.turnStartedAt {
				observation.invalidate()
			}
			if err == nil && (observation.continuous || observation.input != nil || turn.Native.EndedAt > 0) {
				observation.offer(sample, turn.Native, turn, turn.Native.Revision > prior && hadStart)
			}
		}
		if errors.Is(err, native.ErrStateIdentityChanged) {
			turn.Native.reject(err.Error())
		}
	}
	drain()
	if turn.Native.Conflict != "" {
		observation.invalidate()
		return turn.Native.diagnostic()
	}
	if turn.Native.wantsQuery(l.clock()) {
		turn.Native.LastQuery, turn.Native.HintPending = l.clock(), false
		if !turn.Accepted || !turn.Started {
			return "Original input has no verified start receipt; no input was resent"
		}
		var sample native.StateSample
		var err error
		if l.queryNative != nil {
			sample, err = l.queryNative(ctx, state.Process, turn.Native.Pane)
		} else if feed != nil {
			sample, err = feed.Query(ctx)
		} else {
			err = streamErr
			if err == nil {
				err = errors.New("native original query source unavailable")
			}
		}
		if err != nil {
			if errors.Is(err, native.ErrStateIdentityChanged) {
				turn.Native.reject(err.Error())
				observation.invalidate()
			}
			return "Native main-agent query is unverified: " + err.Error()
		}
		drain()
		if turn.Native.Conflict != "" {
			observation.invalidate()
			return turn.Native.diagnostic()
		}
		if _, err := l.original(ctx, snapshot); err != nil {
			observation.invalidate()
			return err.Error()
		}
		s := sample.AgentState
		// Preserve the ordinary query's contrary-evidence checks before an
		// optional supplemental read can fail or make these facts unavailable.
		if (s.MainAgent == nil || s.UpdatedAt >= s.MainAgent.StateStartedAt) && (turn.Native.rejectIdentity(s) || turn.Native.rejectProvenance(s, turn)) {
			observation.invalidate()
			return turn.Native.diagnostic()
		}
		if l.queryInput != nil && s.Prompt == "" && turn.Accepted && turn.Started && turn.Native.agentName() == "codex" &&
			s.MainAgent != nil && s.UpdatedAt >= s.MainAgent.StateStartedAt &&
			turn.Native.BaseRootAt > 0 && turn.Native.Conflict == "" &&
			(turn.Native.WorkingAt > 0 || s.CanQueryCompletionInput() && s.MainAgent.StateStartedAt > turn.Native.BaseRootAt) &&
			turn.Native.attribution(s, turn, false).kind == provenanceMissing && !turn.Native.retainsOriginalInput(s) {
			var inputErr error
			s, inputErr = l.queryInput(ctx, state.Process, turn.Native.Pane, turn.Native.BaseRootAt, s)
			if inputErr != nil {
				if errors.Is(inputErr, native.ErrStateIdentityChanged) {
					turn.Native.reject(inputErr.Error())
				} else if s.MainAgent == nil || s.UpdatedAt >= s.MainAgent.StateStartedAt {
					// A failed post-read cross-check may still carry positive
					// contradictory facts; retain them just like an ordinary query.
					turn.Native.rejectIdentity(s)
					turn.Native.rejectProvenance(s, turn)
				}
				if turn.Native.Conflict != "" {
					observation.invalidate()
					return turn.Native.diagnostic()
				}
				return "Native original input query is unverified: " + inputErr.Error()
			}
			sample.AgentState = s
			if s.NativeInput.MatchesOriginalQuery(s) {
				if !s.NativeInput.MatchesOriginalProcess(state.Process) {
					turn.Native.reject("Native original input query belongs to another process")
					observation.invalidate()
					return turn.Native.diagnostic()
				}
				if _, err := l.original(ctx, snapshot); err != nil {
					observation.invalidate()
					return err.Error()
				}
			}
			// The extra read can span more notifications. A later contradictory
			// input/identity must win over the earlier query just as in the
			// ordinary independent-query path.
			drain()
			if turn.Native.Conflict != "" {
				observation.invalidate()
				return turn.Native.diagnostic()
			}
			if s.NativeInput != nil && s.NativeInput.Observation != nil && s.NativeInput.Observation.Revision < observation.observedRevision {
				return "Native original input changed during native transcript query; awaiting current original evidence"
			}
		}
		verified := false
		if observation.supplements(sample, turn.Native) && turn.Native.Conflict == "" {
			verified = turn.Native.end(s, turn, received(sample, l.clock()), true)
		} else {
			verified = turn.Native.query(s, turn, received(sample, l.clock()), l.profile.Environment == "" && turn.Native.WorkingAt > 0)
		}
		if turn.Native.Waiting != "" {
			diagnostic = turn.Native.Waiting
		}
		if verified || s.MainAgent != nil && s.MainAgent.StateStartedAt > turn.Native.BaseRootAt && turn.Native.attribution(s, turn, false).kind == provenanceMatched {
			diagnostic = ""
		}
		if turn.Native.recordedInputMatches(s, turn) && s.TurnStartedAt > 0 {
			observation.input, observation.inputStamp = s.NativeInput, s.TurnStartedAt
			observation.continuous, observation.turnStartedAt = false, 0
		}
		if s.MainAgent != nil && s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > turn.Native.BaseRootAt {
			if issue := turn.Native.attribution(s, turn, false); issue.kind == provenanceMissing && turn.Native.EndedAt == 0 {
				diagnostic = issue.diagnostic
			}
		}
		observation.offer(sample, turn.Native, turn, false)
	}
	if turn.Native.diagnostic() != "" {
		return turn.Native.diagnostic()
	}
	if turn.Native.EndedAt == 0 && streamErr != nil {
		return "Native state subscription is unverified; original query remains available: " + streamErr.Error()
	}
	if turn.Native.EndedAt > 0 {
		return ""
	}
	return diagnostic
}
