package usage

import (
	"math"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestUsagePartialFieldsStillValidateKnownRelations(t *testing.T) {
	for _, tokens := range []api.Tokens{
		{InputTotal: api.Number(1), CacheRead: api.Number(2)},
		{InputTotal: api.Number(1), CacheWrite: api.Number(2)},
		{InputTotal: api.Number(1), InputUncached: api.Number(2)},
		{InputTotal: api.Number(3), CacheRead: api.Number(2), CacheWrite: api.Number(2)},
		{InputUncached: api.Number(math.MaxInt64), CacheRead: api.Number(1)},
		{InputUncached: api.Number(math.MaxInt64), CacheRead: api.Number(1), CacheWrite: api.Number(0)},
		{Total: api.Number(1), InputTotal: api.Number(2)},
		{Total: api.Number(1), Output: api.Number(2)},
		{Total: api.Number(1), Reasoning: api.Number(2)},
		{Total: api.Number(3), CacheRead: api.Number(2), Output: api.Number(2)},
		{Total: api.Number(3), InputUncached: api.Number(2), CacheRead: api.Number(2)},
		{InputTotal: api.Number(math.MaxInt64), Output: api.Number(1)},
		{InputUncached: api.Number(math.MaxInt64), Reasoning: api.Number(1)},
	} {
		if Validate(tokens) == nil {
			t.Fatal("invalid known counters accepted", tokens)
		}
	}
	for _, tokens := range []api.Tokens{
		{Total: api.Number(5), CacheRead: api.Number(2), Output: api.Number(2)},
		{Total: api.Number(5), Reasoning: api.Number(2)},
	} {
		if err := Validate(tokens); err != nil {
			t.Fatal("consistent nullable counters rejected", tokens, err)
		}
	}
}

func TestUsageDedupMetadataEnrichment(t *testing.T) {
	old := api.Event{ID: "response", RunID: "r", CommandID: "cmd", Decision: "counted", Tokens: api.Tokens{Total: api.Number(3)}, MetadataBasis: "unknown", MetadataIssues: []api.Issue{{Category: "metadata", Code: "SOURCE_METADATA_INCOMPLETE"}}}
	next := old
	model, provider := "observed-model", "observed-provider"
	now := time.Now().UTC()
	next.Model = &model
	next.Provider = &provider
	next.CallTime = &now
	next.TimeBasis = "completion"
	next.MetadataBasis = "observed"
	enriched := Merge(old, next)
	if enriched.Decision != "counted" || enriched.MetadataBasis != "observed" || enriched.Model == nil || len(enriched.MetadataIssues) != 0 || *enriched.Tokens.Total != 3 {
		t.Fatal(enriched)
	}
	next.CommandID = "another-call"
	if Merge(enriched, next).Decision != "quarantined" {
		t.Fatal("reused response identity crossed commands")
	}
}
func TestUsageAvailability(t *testing.T) {
	for _, tc := range []struct {
		code, status string
		retry        bool
	}{{"SESSION_UNBOUND", "pending", true}, {"HOST_OFFLINE", "unavailable", true}, {"COLLECTION_DISABLED", "unavailable", false}, {"PRODUCTION_BINDING_UNSUPPORTED", "unavailable", false}} {
		s := Project(api.Snapshot{RunID: "r", Issues: []api.Issue{{Category: "usage", Code: tc.code, Retryable: tc.retry}}}, nil)
		if s.Status != tc.status || s.Tokens != nil {
			t.Fatal(s)
		}
	}
}

func TestUsageDedupInitializationPhase(t *testing.T) {
	old := api.Event{ID: "response", RunID: "r", Phase: "initialization", Decision: "counted", Tokens: api.Tokens{Total: api.Number(1)}}
	next := old
	next.Phase = "command"
	next.CommandID = "business"
	if Merge(old, next).Decision != "quarantined" {
		t.Fatal("initialization response reassigned to business command")
	}
}

func TestUsageIncompleteFieldsAndNoImplicitWaiting(t *testing.T) {
	for _, tokens := range []api.Tokens{{Total: api.Number(3)}, {InputTotal: api.Number(3)}} {
		s := Project(api.Snapshot{RunID: "r", Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, Finalized: true}}, []api.Event{{ID: "e", Decision: "counted", Tokens: tokens}})
		if s.Status != "partial" || s.Coverage.Finalized || s.Tokens == nil {
			t.Fatal(s)
		}
	}
	s := Project(api.Snapshot{RunID: "r", Coverage: api.Coverage{Finalized: true}}, nil)
	if s.Status != "unavailable" || s.Tokens != nil || s.Coverage.Finalized || len(s.Issues) == 0 {
		t.Fatal("absence became pending or zero", s)
	}
}

func TestUsageMetadataConflictRemainsUnknownAfterReplay(t *testing.T) {
	model1, model2, provider := "one", "two", "provider"
	old := api.Event{ID: "e", RunID: "r", SessionID: "s", ResponseID: "resp", Model: &model1, Provider: &provider, MetadataBasis: "observed", Decision: "counted", Tokens: api.Tokens{Total: api.Number(1)}}
	next := old
	next.Model = &model2
	conflict := Merge(old, next)
	again := Merge(conflict, old)
	if again.Decision != "counted" || again.Model != nil || again.Provider != nil || again.MetadataBasis != "unknown" || len(again.MetadataIssues) != 1 || again.MetadataIssues[0].Code != "SOURCE_METADATA_CONFLICT" {
		t.Fatal("metadata was chosen after conflict", again)
	}
	next = old
	next.SessionID = "different"
	if Merge(old, next).Decision != "quarantined" {
		t.Fatal("same key hid session change")
	}
}
