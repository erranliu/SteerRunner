package host

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/cost"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func (s *server) usageQuery(o Options) contracts.Response {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reject := func(err error) contracts.Response {
		var fault *contracts.Fault
		if errors.As(err, &fault) {
			return contracts.Rejected("", fault)
		}
		switch {
		case errors.Is(err, api.ErrSnapshotNotFound):
			return contracts.Rejected("", contracts.Fail(5, "snapshot_not_found", "The requested usage snapshot has not been committed."))
		case errors.Is(err, api.ErrSnapshotExpired):
			return contracts.Rejected("", contracts.Fail(5, "snapshot_expired", "The requested usage snapshot has expired."))
		case errors.Is(err, api.ErrCursor):
			return contracts.Rejected("", contracts.Fail(5, "invalid_usage_cursor", "Cursor does not belong to this Run and snapshot."))
		}
		return contracts.Rejected("", contracts.Fail(1, "internal_error", "Cannot read usage projection."))
	}
	// Target existence is distinct from domain unavailability and snapshot absence.
	if _, err := s.engine.HandleUsageTarget(o.RunID); err != nil {
		return reject(err)
	}
	switch o.Command {
	case "run-usage":
		v, err := s.store.GetRunUsage(ctx, o.RunID, o.UsageRevision)
		if err != nil {
			return reject(err)
		}
		return contracts.Success("", v)
	case "run-usage-events":
		v, err := s.store.ListRunUsageEvents(ctx, o.RunID, o.UsageRevision, o.After, o.Limit)
		if err != nil {
			return reject(err)
		}
		return contracts.Success("", v)
	case "estimate-run-cost":
		// Optional pricing initialization is isolated from host and usage startup.
		policy, err := readPricingPolicy(o.PricingPolicy)
		if err != nil {
			snapshot, e := s.store.GetRunUsage(ctx, o.RunID, o.UsageRevision)
			if e != nil {
				return reject(e)
			}
			v := cost.Unavailable(o.RunID, snapshot.Revision, "COST_POLICY_INVALID")
			v.Scope = snapshot.Scope
			v.UsageStatus = snapshot.Status
			return contracts.Success("", v)
		}
		v, err := cost.EstimateRun(ctx, s.store, o.RunID, o.UsageRevision, policy)
		if err != nil {
			return reject(err)
		}
		return contracts.Success("", v)
	}
	return reject(api.ErrSnapshotNotFound)
}
func readPricingPolicy(file string) (cost.Policy, error) {
	var policy cost.Policy
	if !filepath.IsAbs(file) {
		return policy, errors.New("absolute pricing policy path required")
	}
	f, err := os.Open(file)
	if err != nil {
		return policy, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil || len(raw) > 1<<20 {
		return policy, errors.New("pricing policy limit")
	}
	if err = contracts.DecodeStrict(raw, &policy); err != nil {
		return policy, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return policy, err
	}
	for _, name := range []string{"schema", "id", "version", "source", "currency", "enabled", "allow_completion_time", "rules"} {
		if fields[name] == nil || string(fields[name]) == "null" {
			return policy, errors.New("missing pricing policy field")
		}
	}
	// Optional predicates may be omitted, but explicit null is invalid in the
	// published policy schema and must not silently erase a pricing condition.
	var rules []map[string]json.RawMessage
	if err = json.Unmarshal(fields["rules"], &rules); err != nil {
		return policy, err
	}
	for _, rule := range rules {
		for _, name := range []string{"tier", "min_context_tokens", "max_context_tokens"} {
			if value, present := rule[name]; present && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return policy, errors.New("null pricing predicate")
			}
		}
	}
	return policy, policy.Validate()
}
