package host

import (
	"context"
	"errors"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
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

	}
	return reject(api.ErrSnapshotNotFound)
}
