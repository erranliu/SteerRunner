package cost

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/usage/api"
)

type reader struct {
	s         api.Snapshot
	events    []api.Event
	revisions []uint64
}

func (r *reader) GetRunUsage(_ context.Context, _ string, revision uint64) (api.Snapshot, error) {
	if revision != 0 && revision != r.s.Revision {
		return api.Snapshot{}, api.ErrSnapshotNotFound
	}
	return r.s, nil
}
func (r *reader) ListRunUsageEvents(_ context.Context, run string, revision uint64, _ string, _ int) (api.Page, error) {
	r.revisions = append(r.revisions, revision)
	return api.Page{RunID: run, Revision: revision, Events: r.events}, nil
}
func pricingFixture() (*reader, Policy) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	model, provider := "fixture-model", "fixture-provider"
	e := api.Event{RunID: "r", ID: "e", Decision: "counted", Provider: &provider, Model: &model, MetadataBasis: "observed", CallTime: &now, TimeBasis: "request", Tokens: api.Tokens{InputTotal: api.Number(1100000), InputUncached: api.Number(1000000), CacheRead: api.Number(100000), CacheWrite: api.Number(0), Output: api.Number(500000), Reasoning: api.Number(200000), Total: api.Number(1600000)}}
	r := &reader{s: api.Snapshot{RunID: "r", Revision: 7, Status: "available", Scope: "main_session", Tokens: &e.Tokens, Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, CountedResponses: 1, Finalized: true}}, events: []api.Event{e}}
	p := Policy{Schema: "run-pricing-policy/v1", ID: "frozen-test-only", Version: "1", Source: "synthetic arithmetic fixture, not provider pricing", Currency: "USD", Enabled: true, Rules: []Rule{{Provider: provider, Model: model, From: now.Add(-24 * time.Hour), Until: now.Add(24 * time.Hour), Rates: Rates{Input: "2", CacheRead: "0.2", CacheWrite: "3", Output: "4"}}}}
	return r, p
}
func TestCostDeterministicAndIndependent(t *testing.T) {
	r, p := pricingFixture()
	e, err := EstimateRun(context.Background(), r, "r", 7, p)
	if err != nil || e.Status != "estimated" || e.Total == nil || *e.Total != "4.02000000" || e.UsageRevision != 7 || r.revisions[0] != 7 {
		t.Fatal(e, err)
	}
	p.Rules[0].Rates.Output = "8"
	again, err := EstimateRun(context.Background(), r, "r", 7, p)
	if err != nil || again.PolicyDigest == e.PolicyDigest || *again.Total != "6.02000000" || *r.s.Tokens.Total != 1600000 {
		t.Fatal(again, err)
	}
}

func TestCostTokenOnlyScopeAndCompletionAreExplicit(t *testing.T) {
	r, p := pricingFixture()
	r.events[0].TimeBasis = "completion"
	p.AllowCompletionTime = true
	estimate, err := EstimateRun(context.Background(), r, "r", 7, p)
	if err != nil || estimate.Status != "estimated" || estimate.Basis != "api_equivalent" {
		t.Fatal(estimate, err)
	}
	notes := strings.Join(estimate.Assumptions, "\n")
	for _, required := range []string{"Token-only", "subscription/credits", "tool or external API charges", "completion time"} {
		if !strings.Contains(notes, required) {
			t.Fatal("estimate omitted its billing scope or time assumption", required, estimate)
		}
	}
}
func TestCostAvailability(t *testing.T) {
	cases := []struct {
		name         string
		mutate       func(*reader, *Policy)
		status, code string
	}{{"active", func(r *reader, p *Policy) { r.s.Coverage.Finalized = false }, "partial", "USAGE_NOT_FINALIZED"}, {"partial", func(r *reader, p *Policy) { r.s.Status = "partial" }, "partial", "USAGE_PARTIAL"}, {"unknown model", func(r *reader, p *Policy) { r.events[0].Model = nil }, "unavailable", "MODEL_UNKNOWN"}, {"metadata conflict", func(r *reader, p *Policy) {
		r.events[0].MetadataIssues = []api.Issue{{Code: "SOURCE_METADATA_CONFLICT"}}
	}, "unavailable", "PRICING_METADATA_CONFLICT"}, {"missing split", func(r *reader, p *Policy) { r.events[0].Tokens.CacheWrite = nil }, "unavailable", "USAGE_PARTIAL"}, {"completion uncertain", func(r *reader, p *Policy) { r.events[0].TimeBasis = "completion" }, "unavailable", "PRICE_PERIOD_UNKNOWN"}, {"bad policy", func(r *reader, p *Policy) { p.Rules[0].Rates.Input = "NaN" }, "unavailable", "COST_POLICY_INVALID"}, {"disabled", func(r *reader, p *Policy) { p.Enabled = false }, "unavailable", "ESTIMATION_DISABLED"}, {"unknown usage", func(r *reader, p *Policy) { r.s.Tokens = nil }, "unavailable", "USAGE_UNAVAILABLE"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, p := pricingFixture()
			tc.mutate(r, &p)
			e, err := EstimateRun(context.Background(), r, "r", 7, p)
			if err != nil || e.Status != tc.status {
				t.Fatal(e, err)
			}
			found := false
			for _, i := range e.Issues {
				if i == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatal("missing reason", e)
			}
			if e.Status != "estimated" && e.Total != nil {
				t.Fatal("invented total")
			}
			if e.Status == "unavailable" && e.Covered != nil {
				t.Fatal("invented zero")
			}
		})
	}
}
func TestCostSnapshotAndCompletionAssumption(t *testing.T) {
	r, p := pricingFixture()
	if _, err := EstimateRun(context.Background(), r, "r", 6, p); err != api.ErrSnapshotNotFound {
		t.Fatal(err)
	}
	r.events[0].TimeBasis = "completion"
	p.AllowCompletionTime = true
	e, err := EstimateRun(context.Background(), r, "r", 7, p)
	if err != nil || e.Status != "estimated" || len(e.Assumptions) == 0 {
		t.Fatal(e, err)
	}
	p.Rules = append(p.Rules, p.Rules[0])
	e, err = EstimateRun(context.Background(), r, "r", 7, p)
	if err != nil || e.Status != "unavailable" {
		t.Fatal("overlapping rules accepted", e, err)
	}
}

func TestCostUnknownDiscriminantCannotSelectFallback(t *testing.T) {
	for _, field := range []string{"tier", "context", "time"} {
		t.Run(field, func(t *testing.T) {
			r, p := pricingFixture()
			rule := p.Rules[0]
			code := "PRICING_CONTEXT_UNKNOWN"
			switch field {
			case "tier":
				rule.Tier = "priority"
				p.Rules = append(p.Rules, rule)
			case "context":
				rule.MinContext = api.Number(200000)
				p.Rules = append(p.Rules, rule)
			case "time":
				r.events[0].TimeBasis = "unknown"
				p.AllowCompletionTime = true
				code = "PRICE_PERIOD_UNKNOWN"
			}
			got, err := EstimateRun(context.Background(), r, "r", 7, p)
			if err != nil || got.Status != "unavailable" || got.Total != nil || got.Covered != nil {
				t.Fatal(got, err)
			}
			found := false
			for _, issue := range got.Issues {
				found = found || issue == code
			}
			if !found {
				t.Fatal("reason missing", got)
			}
		})
	}
}
