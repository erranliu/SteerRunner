// Package cost prices immutable usage through its public Reader only. It has no
// dependency on execution, collection, Orca, storage, or native log formats.
package cost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/usage/api"
)

type Rates struct {
	Input      string `json:"input_uncached_per_million"`
	CacheRead  string `json:"cache_read_per_million"`
	CacheWrite string `json:"cache_write_per_million"`
	Output     string `json:"output_per_million"`
}
type Rule struct {
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	From       time.Time `json:"effective_from"`
	Until      time.Time `json:"effective_until"`
	Tier       string    `json:"tier,omitempty"`
	MinContext *int64    `json:"min_context_tokens,omitempty"`
	MaxContext *int64    `json:"max_context_tokens,omitempty"`
	Rates      Rates     `json:"rates"`
}
type Policy struct {
	Schema              string `json:"schema"`
	ID                  string `json:"id"`
	Version             string `json:"version"`
	Source              string `json:"source"`
	Currency            string `json:"currency"`
	Enabled             bool   `json:"enabled"`
	AllowCompletionTime bool   `json:"allow_completion_time"`
	Rules               []Rule `json:"rules"`
}
type Estimate struct {
	Schema        string   `json:"schema"`
	RunID         string   `json:"run_id"`
	Status        string   `json:"status"`
	UsageRevision uint64   `json:"usage_revision"`
	Scope         string   `json:"scope"`
	UsageStatus   string   `json:"usage_status"`
	PolicyID      string   `json:"pricing_policy"`
	PolicyVersion string   `json:"pricing_policy_version"`
	PolicyDigest  string   `json:"pricing_policy_digest"`
	Currency      string   `json:"currency"`
	Covered       *string  `json:"covered_estimated_amount"`
	Total         *string  `json:"total_estimated_amount"`
	Priced        int64    `json:"priced_responses"`
	Unpriced      int64    `json:"unpriced_responses"`
	Assumptions   []string `json:"assumptions"`
	Issues        []string `json:"issues"`
	Basis         string   `json:"basis"`
}

func decimal(s string) (*big.Rat, bool) {
	if len(s) == 0 || len(s) > 64 || strings.ContainsAny(s, "/eE+-") {
		return nil, false
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return nil, false
	}
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return nil, false
			}
		}
	}
	v, ok := new(big.Rat).SetString(s)
	return v, ok && v.Sign() >= 0
}
func (p Policy) Validate() error {
	if p.Schema != "run-pricing-policy/v1" || p.ID == "" || p.Version == "" || p.Source == "" || len(p.Currency) != 3 || len(p.Rules) == 0 || len(p.Rules) > 1000 {
		return errors.New("COST_POLICY_INVALID")
	}
	for _, c := range p.Currency {
		if c < 'A' || c > 'Z' {
			return errors.New("COST_POLICY_INVALID")
		}
	}
	for _, r := range p.Rules {
		if r.Model == "" || r.Provider == "" || r.From.IsZero() || !r.Until.After(r.From) {
			return errors.New("COST_POLICY_INVALID")
		}
		if r.MinContext != nil && *r.MinContext < 0 || r.MaxContext != nil && *r.MaxContext < 0 || r.MinContext != nil && r.MaxContext != nil && *r.MaxContext < *r.MinContext {
			return errors.New("COST_POLICY_INVALID")
		}
		for _, s := range []string{r.Rates.Input, r.Rates.CacheRead, r.Rates.CacheWrite, r.Rates.Output} {
			if _, ok := decimal(s); !ok {
				return errors.New("COST_POLICY_INVALID")
			}
		}
	}
	return nil
}
func Unavailable(run string, revision uint64, code string) Estimate {
	return Estimate{Schema: "run-cost-estimate/v1", RunID: run, UsageRevision: revision, Status: "unavailable", Basis: "api_equivalent", Assumptions: []string{"Token-only API-equivalent estimate; excludes subscription/credits settlement and tool or external API charges."}, Issues: []string{code}}
}
func EstimateRun(ctx context.Context, reader api.Reader, run string, revision uint64, p Policy) (Estimate, error) {
	s, err := reader.GetRunUsage(ctx, run, revision)
	if err != nil {
		return Estimate{}, err
	}
	e := Unavailable(run, s.Revision, "COST_POLICY_INVALID")
	e.Scope = s.Scope
	e.UsageStatus = s.Status
	e.PolicyID = p.ID
	e.PolicyVersion = p.Version
	e.Currency = p.Currency
	raw, _ := json.Marshal(p)
	digest := sha256.Sum256(raw)
	e.PolicyDigest = hex.EncodeToString(digest[:])
	if p.Validate() != nil {
		return e, nil
	}
	e.Issues = []string{}
	addIssue := func(code string) {
		for _, old := range e.Issues {
			if old == code {
				return
			}
		}
		e.Issues = append(e.Issues, code)
	}
	if !p.Enabled {
		addIssue("ESTIMATION_DISABLED")
		return e, nil
	}
	if s.Tokens == nil || s.Status == "unavailable" || s.Status == "pending" {
		addIssue("USAGE_UNAVAILABLE")
		return e, nil
	}
	amount := new(big.Rat)
	cursor := ""
	for {
		page, err := reader.ListRunUsageEvents(ctx, run, s.Revision, cursor, 200)
		if err != nil {
			return Estimate{}, err
		}
		if page.Revision != s.Revision || page.RunID != run {
			return Estimate{}, api.ErrSnapshotNotFound
		}
		for _, event := range page.Events {
			if event.Decision != "counted" {
				continue
			}
			value, code, assumption := price(event, p)
			if code != "" {
				e.Unpriced++
				addIssue(code)
				continue
			}
			e.Priced++
			amount.Add(amount, value)
			if assumption != "" && !slices.Contains(e.Assumptions, assumption) {
				e.Assumptions = append(e.Assumptions, assumption)
			}
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor {
			return Estimate{}, api.ErrCursor
		}
		cursor = page.NextCursor
	}
	if e.Priced != s.Coverage.CountedResponses-e.Unpriced {
		addIssue("USAGE_PARTIAL")
		return e, nil
	}
	if e.Priced == 0 {
		if s.Status == "available" && s.Coverage.Finalized && s.Coverage.CountedResponses == 0 && s.Coverage.ExpectedSources != nil && s.Coverage.VerifiedSources == *s.Coverage.ExpectedSources && s.Tokens.Total != nil && *s.Tokens.Total == 0 {
			zero := "0.00000000"
			e.Covered = &zero
			e.Total = &zero
			e.Status = "estimated"
			return e, nil
		}
		addIssue("PRICE_NOT_FOUND")
		return e, nil
	}
	value := amount.FloatString(8)
	e.Covered = &value
	e.Status = "partial"
	if s.Status != "available" {
		addIssue("USAGE_PARTIAL")
	}
	if !s.Coverage.Finalized {
		addIssue("USAGE_NOT_FINALIZED")
	}
	if s.Status == "available" && s.Coverage.Finalized && s.Coverage.ExpectedSources != nil && *s.Coverage.ExpectedSources == s.Coverage.VerifiedSources && e.Unpriced == 0 {
		e.Status = "estimated"
		e.Total = &value
	}
	return e, nil
}
func price(e api.Event, p Policy) (*big.Rat, string, string) {
	for _, i := range e.MetadataIssues {
		if i.Code == "SOURCE_METADATA_CONFLICT" {
			return nil, "PRICING_METADATA_CONFLICT", ""
		}
	}
	if e.Model == nil || e.Provider == nil || e.MetadataBasis != "observed" {
		return nil, "MODEL_UNKNOWN", ""
	}
	if e.CallTime == nil {
		return nil, "PRICE_PERIOD_UNKNOWN", ""
	}
	assumption := ""
	if e.TimeBasis != "request" {
		if e.TimeBasis != "completion" || !p.AllowCompletionTime {
			return nil, "PRICE_PERIOD_UNKNOWN", ""
		}
		assumption = "Policy explicitly uses native completion time as the request-time proxy."
	}
	var matches []Rule
	uncertain := false
	code := "PRICE_NOT_FOUND"
	for _, r := range p.Rules {
		if r.Model != *e.Model || r.Provider != *e.Provider {
			continue
		}
		if e.CallTime.Before(r.From) || !e.CallTime.Before(r.Until) {
			code = "PRICE_PERIOD_UNKNOWN"
			continue
		}
		if r.Tier != "" && e.Tier != nil && *e.Tier != r.Tier {
			continue
		}
		if e.ContextTokens != nil && (r.MinContext != nil && *e.ContextTokens < *r.MinContext || r.MaxContext != nil && *e.ContextTokens > *r.MaxContext) {
			continue
		}
		if r.Tier != "" && e.Tier == nil || (r.MinContext != nil || r.MaxContext != nil) && e.ContextTokens == nil {
			uncertain = true
			continue
		}
		matches = append(matches, r)
	}
	if uncertain {
		return nil, "PRICING_CONTEXT_UNKNOWN", ""
	}
	if len(matches) != 1 {
		if len(matches) > 1 {
			code = "COST_POLICY_INVALID"
		}
		return nil, code, ""
	}
	r := matches[0]
	sum := new(big.Rat)
	counts := []*int64{e.Tokens.InputUncached, e.Tokens.CacheRead, e.Tokens.CacheWrite, e.Tokens.Output}
	rates := []string{r.Rates.Input, r.Rates.CacheRead, r.Rates.CacheWrite, r.Rates.Output}
	for i, count := range counts {
		if count == nil || *count < 0 {
			return nil, "USAGE_PARTIAL", ""
		}
		rate, _ := decimal(rates[i])
		sum.Add(sum, new(big.Rat).Mul(rate, new(big.Rat).SetFrac(big.NewInt(*count), big.NewInt(1000000))))
	}
	return sum, "", assumption
}
