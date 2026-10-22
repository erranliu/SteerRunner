// Package api is the price-independent, immutable public usage contract.
package api

import (
	"context"
	"errors"
	"time"
)

const Version = "run-usage/v1"

var ErrSnapshotNotFound = errors.New("snapshot_not_found")
var ErrSnapshotExpired = errors.New("snapshot_expired")
var ErrCursor = errors.New("invalid_usage_cursor")

type Tokens struct {
	InputTotal    *int64 `json:"input_total"`
	InputUncached *int64 `json:"input_uncached"`
	CacheRead     *int64 `json:"cache_read"`
	CacheWrite    *int64 `json:"cache_write"`
	Output        *int64 `json:"output"`
	Reasoning     *int64 `json:"reasoning"`
	Total         *int64 `json:"total"`
}

func Number(n int64) *int64 { return &n }

type Issue struct {
	Category  string `json:"category"`
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	SourceRef string `json:"source_ref,omitempty"`
}
type SourceSummary struct {
	SourceRef string `json:"source_ref"`
	Status    string `json:"status"`
	Finalized bool   `json:"finalized"`
	Watermark string `json:"watermark"`
}
type Coverage struct {
	ExpectedSources       *int64          `json:"expected_sources"`
	VerifiedSources       int64           `json:"verified_sources"`
	CountedResponses      int64           `json:"counted_responses"`
	MissingUsageResponses int64           `json:"missing_usage_responses"`
	ThroughEventID        string          `json:"through_event_id"`
	Finalized             bool            `json:"finalized"`
	SourceSummaries       []SourceSummary `json:"source_summaries"`
}
type Snapshot struct {
	Schema              string    `json:"schema"`
	RunID               string    `json:"run_id"`
	Revision            uint64    `json:"snapshot_revision"`
	Status              string    `json:"status"`
	Scope               string    `json:"scope"`
	Tokens              *Tokens   `json:"tokens"`
	Coverage            Coverage  `json:"coverage"`
	Issues              []Issue   `json:"issues"`
	AccessCleanupStatus string    `json:"access_cleanup_status"`
	ObservedAt          time.Time `json:"observed_at"`
}
type Event struct {
	ID             string     `json:"event_id"`
	SourceRef      string     `json:"source_ref"`
	RunID          string     `json:"run_id"`
	AttemptID      string     `json:"attempt_id"`
	Generation     string     `json:"generation"`
	CommandID      string     `json:"command_id"`
	Phase          string     `json:"phase,omitempty"`
	Agent          string     `json:"agent"`
	SessionID      string     `json:"native_session_id"`
	TurnID         string     `json:"native_turn_id,omitempty"`
	ResponseID     string     `json:"native_response_id"`
	Tokens         Tokens     `json:"tokens"`
	Provider       *string    `json:"provider"`
	Model          *string    `json:"model"`
	MetadataBasis  string     `json:"metadata_basis"`
	CallTime       *time.Time `json:"call_time"`
	TimeBasis      string     `json:"time_basis"`
	Tier           *string    `json:"tier"`
	ContextTokens  *int64     `json:"context_tokens"`
	ObservedAt     time.Time  `json:"observed_at"`
	Format         string     `json:"format"`
	ParserVersion  string     `json:"parser_version"`
	EvidenceRef    string     `json:"evidence_ref"`
	MetadataIssues []Issue    `json:"metadata_issues"`
	Decision       string     `json:"decision"`
}
type Page struct {
	RunID      string  `json:"run_id"`
	Revision   uint64  `json:"snapshot_revision"`
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor"`
}
type Reader interface {
	GetRunUsage(context.Context, string, uint64) (Snapshot, error)
	ListRunUsageEvents(context.Context, string, uint64, string, int) (Page, error)
}
