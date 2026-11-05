package host

import (
	"context"
	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
)

// InspectOffline reads the original committed snapshot while no service can
// write it, without constructing an execution engine or changing authority.
func InspectOffline(ctx context.Context, dir string, opts Options) contracts.Response {
	reject := func(f *contracts.Fault) contracts.Response { return contracts.Rejected("", f) }
	selectors := 0
	for _, id := range []string{opts.RunID, opts.RequestID, opts.CommandID} {
		if id != "" {
			selectors++
		}
	}
	if selectors != 1 || opts.Command != "inspect" || opts.ExpectedBinding != nil || opts.ExpectedInstanceID != "" {
		return reject(contracts.Fail(2, "invalid_arguments", "Offline inspect only reads persisted original identities."))
	}
	if err := ctx.Err(); err != nil {
		return reject(contracts.Fail(4, "service_unavailable", "Offline query deadline expired."))
	}
	lease, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), false)
	if err != nil {
		return reject(fault(err, contracts.Fail(4, "service_unavailable", "Original writer lease cannot be read.")))
	}
	if busy {
		return reject(contracts.Fail(5, "directory_conflict", "A writer still owns this state directory; use resident inspect."))
	}
	defer lease.Close()
	if _, err = os.Stat(bootstrapPath(dir)); !os.IsNotExist(err) {
		return reject(contracts.Fail(4, "startup_unverified", "An original startup remains unresolved."))
	}
	store, err := storage.OpenReader(dir)
	if err != nil {
		return reject(fault(err, contracts.Fail(4, "service_unavailable", "Original current-format state cannot be read.")))
	}
	defer store.Close()
	response, err := application.InspectPersisted(store, opts.RunID, opts.RequestID, opts.CommandID)
	if err != nil {
		return reject(fault(err, contracts.Fail(4, "service_unavailable", "Original persisted projection is unavailable.")))
	}
	return response
}
