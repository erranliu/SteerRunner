package host

import (
	"context"
	"os"
	"path/filepath"

	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// VerifyStopped is an explicit offline query, separate from discovery status.
// It never creates missing state, starts a writer, or alters shutdown receipts.
func VerifyStopped(ctx context.Context, dir string) (contracts.StoppedVerification, *contracts.Fault) {
	empty := contracts.StoppedVerification{}
	if err := ctx.Err(); err != nil {
		return empty, contracts.Fail(4, "service_unavailable", "Verification deadline expired.")
	}
	lock, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), false)
	if err != nil {
		return empty, fault(err, contracts.Fail(4, "service_unavailable", "Original writer lease cannot be verified."))
	}
	if busy {
		return empty, contracts.Fail(5, "directory_conflict", "A writer still owns this directory; resources were not verified.")
	}
	defer lock.Close()
	if _, err = os.Stat(bootstrapPath(dir)); !os.IsNotExist(err) {
		return empty, contracts.Fail(4, "startup_unverified", "An original startup remains unresolved.")
	}
	store, err := storage.OpenReader(dir)
	if err != nil {
		return empty, fault(err, contracts.Fail(4, "service_unavailable", "Current execution evidence cannot be read."))
	}
	defer store.Close()
	runs, err := store.Runs()
	if err != nil {
		return empty, contracts.Fail(4, "service_unavailable", "Current execution evidence cannot be read.")
	}
	checks, err := store.Checks()
	if err != nil {
		return empty, contracts.Fail(4, "service_unavailable", "Current check evidence cannot be read.")
	}
	result := application.VerifyStopped(ctx, dir, runs, checks...)
	if err := ctx.Err(); err != nil {
		return empty, contracts.Fail(4, "service_unavailable", "Verification deadline expired.")
	}
	return result, nil
}
