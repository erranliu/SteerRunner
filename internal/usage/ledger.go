// Package usage owns normalization and projections, independently of execution and pricing.
package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"time"

	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func Digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
func Identity(host, namespace, agent, session, response string) string {
	return Digest(host, namespace, agent, session, response)
}
func Validate(t api.Tokens) error {
	for _, v := range []*int64{t.InputTotal, t.InputUncached, t.CacheRead, t.CacheWrite, t.Output, t.Reasoning, t.Total} {
		if v != nil && *v < 0 {
			return errors.New("USAGE_INVALID")
		}
	}
	if t.Reasoning != nil && t.Output != nil && *t.Reasoning > *t.Output {
		return errors.New("USAGE_INVALID")
	}
	if t.InputTotal != nil && t.Output != nil && t.Total != nil {
		if *t.InputTotal > math.MaxInt64-*t.Output || *t.Total != *t.InputTotal+*t.Output {
			return errors.New("USAGE_INVALID")
		}
	}
	knownInput, allInputKnown := int64(0), true
	for _, count := range []*int64{t.InputUncached, t.CacheRead, t.CacheWrite} {
		if count == nil {
			allInputKnown = false
			continue
		}
		if knownInput > math.MaxInt64-*count {
			return errors.New("USAGE_INVALID")
		}
		knownInput += *count
	}
	if t.InputTotal != nil && (knownInput > *t.InputTotal || allInputKnown && knownInput != *t.InputTotal) {
		return errors.New("USAGE_INVALID")
	}
	// Nullable fields cannot hide impossible known lower bounds. Reasoning is
	// part of output, and known input categories are part of input_total; neither
	// becomes an additional category or an inferred value in the public event.
	inputFloor, outputFloor := knownInput, int64(0)
	if t.InputTotal != nil {
		inputFloor = *t.InputTotal
	}
	if t.Reasoning != nil {
		outputFloor = *t.Reasoning
	}
	if t.Output != nil {
		outputFloor = *t.Output
	}
	if inputFloor > math.MaxInt64-outputFloor || t.Total != nil && inputFloor+outputFloor > *t.Total {
		return errors.New("USAGE_INVALID")
	}
	if reflect.DeepEqual(t, api.Tokens{}) {
		return errors.New("USAGE_NOT_RECORDED")
	}
	return nil
}

// Merge never resurrects a quarantined identity. Metadata disputes do not discard tokens.
func Merge(old, next api.Event) api.Event {
	if old.Decision == "quarantined" {
		return old
	}
	if old.RunID != next.RunID || old.SessionID != next.SessionID || old.ResponseID != next.ResponseID || old.Agent != "" && next.Agent != "" && old.Agent != next.Agent || old.Phase != "" && next.Phase != "" && old.Phase != next.Phase || old.CommandID != "" && next.CommandID != "" && old.CommandID != next.CommandID || old.TurnID != "" && next.TurnID != "" && old.TurnID != next.TurnID || !reflect.DeepEqual(old.Tokens, next.Tokens) {
		old.Decision = "quarantined"
		return old
	}
	if old.Phase == "" {
		old.Phase = next.Phase
	}
	for _, issue := range append(append([]api.Issue{}, old.MetadataIssues...), next.MetadataIssues...) {
		if issue.Code == "SOURCE_METADATA_CONFLICT" {
			return metadataConflict(old)
		}
	}
	conflict := false
	mergeString := func(dst **string, src *string) {
		if *dst == nil {
			*dst = src
		} else if src != nil && **dst != *src {
			conflict = true
		}
	}
	mergeString(&old.Provider, next.Provider)
	mergeString(&old.Model, next.Model)
	mergeString(&old.Tier, next.Tier)
	if old.CallTime == nil {
		old.CallTime = next.CallTime
		old.TimeBasis = next.TimeBasis
	} else if next.CallTime != nil && (!old.CallTime.Equal(*next.CallTime) || old.TimeBasis != next.TimeBasis) {
		conflict = true
	}
	if old.ContextTokens == nil {
		old.ContextTokens = next.ContextTokens
	} else if next.ContextTokens != nil && *old.ContextTokens != *next.ContextTokens {
		conflict = true
	}
	if old.MetadataBasis == "unknown" && next.MetadataBasis == "observed" {
		old.MetadataBasis = next.MetadataBasis
	} else if old.MetadataBasis != next.MetadataBasis && next.MetadataBasis != "unknown" {
		conflict = true
	}
	if conflict {
		return metadataConflict(old)
	}
	if !conflict && old.Model != nil && old.Provider != nil && old.CallTime != nil {
		issues := []api.Issue{}
		for _, issue := range old.MetadataIssues {
			if issue.Code != "SOURCE_METADATA_INCOMPLETE" {
				issues = append(issues, issue)
			}
		}
		old.MetadataIssues = issues
	}
	return old
}

func metadataConflict(event api.Event) api.Event {
	event.Model = nil
	event.Provider = nil
	event.Tier = nil
	event.CallTime = nil
	event.ContextTokens = nil
	event.MetadataBasis = "unknown"
	event.TimeBasis = "unknown"
	event.MetadataIssues = []api.Issue{{Category: "metadata", Code: "SOURCE_METADATA_CONFLICT"}}
	return event
}

// Project derives availability from trustworthy data and explicit coverage, never retryability.
func Project(s api.Snapshot, events []api.Event) api.Snapshot {
	s.Schema = api.Version
	s.Scope = "main_session"
	s.Tokens = nil
	s.Coverage.CountedResponses = 0
	s.Coverage.ThroughEventID = ""
	if s.Issues == nil {
		s.Issues = []api.Issue{}
	}
	if s.Coverage.SourceSummaries == nil {
		s.Coverage.SourceSummaries = []api.SourceSummary{}
	}
	var sum api.Tokens
	unknown := [7]bool{}
	gap, failure, waiting := false, false, false
	for _, issue := range s.Issues {
		if issue.Category == "usage" {
			gap = true
			if (issue.Code == "SESSION_UNBOUND" || issue.Code == "LOG_TAIL_INCOMPLETE") && issue.Retryable {
				waiting = true
			} else {
				failure = true
			}
		}
	}
	for _, e := range events {
		if e.Decision != "counted" {
			gap = true
			failure = true
			s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "USAGE_CONFLICT", SourceRef: e.SourceRef})
			continue
		}
		if Validate(e.Tokens) != nil {
			gap = true
			failure = true
			s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "USAGE_INVALID", SourceRef: e.SourceRef})
			continue
		}
		if e.Tokens.InputTotal == nil || e.Tokens.Output == nil || e.Tokens.Total == nil {
			gap = true
			failure = true
			s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "USAGE_NOT_RECORDED", SourceRef: e.SourceRef})
		}
		s.Coverage.CountedResponses++
		s.Coverage.ThroughEventID = e.ID
		vals := []*int64{e.Tokens.InputTotal, e.Tokens.InputUncached, e.Tokens.CacheRead, e.Tokens.CacheWrite, e.Tokens.Output, e.Tokens.Reasoning, e.Tokens.Total}
		dsts := []**int64{&sum.InputTotal, &sum.InputUncached, &sum.CacheRead, &sum.CacheWrite, &sum.Output, &sum.Reasoning, &sum.Total}
		for i, v := range vals {
			if v == nil {
				unknown[i] = true
				continue
			}
			if *dsts[i] == nil {
				*dsts[i] = api.Number(0)
			}
			if **dsts[i] > math.MaxInt64-*v {
				unknown[i] = true
				gap = true
				failure = true
				s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "USAGE_INVALID"})
				continue
			}
			**dsts[i] += *v
		}
	}
	dsts := []**int64{&sum.InputTotal, &sum.InputUncached, &sum.CacheRead, &sum.CacheWrite, &sum.Output, &sum.Reasoning, &sum.Total}
	for i, u := range unknown {
		if u {
			*dsts[i] = nil
		}
	}
	if s.Coverage.MissingUsageResponses > 0 {
		gap = true
		failure = true
		s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "USAGE_NOT_RECORDED"})
	}
	if s.Coverage.ExpectedSources == nil || s.Coverage.VerifiedSources != *s.Coverage.ExpectedSources {
		gap = true
		if !waiting || failure {
			s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "COVERAGE_INCOMPLETE"})
		}
	}
	if s.Coverage.CountedResponses > 0 {
		s.Tokens = &sum
		s.Status = "available"
		if gap {
			s.Status = "partial"
		}
	} else {
		s.Status = "unavailable"
		if waiting && !failure {
			s.Status = "pending"
		}
		if !waiting && !failure {
			s.Issues = append(s.Issues, api.Issue{Category: "usage", Code: "USAGE_NOT_RECORDED"})
		}
	}
	if gap || s.Status != "available" {
		s.Coverage.Finalized = false
	}
	// Sort and coalesce issues so repeated observation does not create spurious revisions.
	sort.Slice(s.Issues, func(i, j int) bool {
		a, b := s.Issues[i], s.Issues[j]
		return a.Category+a.Code+a.SourceRef < b.Category+b.Code+b.SourceRef
	})
	issues := s.Issues[:0]
	for _, v := range s.Issues {
		if len(issues) == 0 || issues[len(issues)-1] != v {
			issues = append(issues, v)
		}
	}
	s.Issues = issues
	return s
}
func Equivalent(a, b api.Snapshot) bool {
	a.Revision = 0
	b.Revision = 0
	a.ObservedAt = time.Time{}
	b.ObservedAt = time.Time{}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
