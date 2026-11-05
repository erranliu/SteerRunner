package host

import (
	"os"
	"path/filepath"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Check before writing startup intent, and again under the writer lock before
// invalidating the old service's shutdown proof.
func checkOrcaStartupState(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "runner.db")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	store, err := storage.OpenReader(dir)
	if err != nil {
		return err
	}
	defer store.Close()
	runs, err := store.Runs()
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Request.Execution.Adapter == "orca-terminal" && run.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
			return contracts.Fail(5, "legacy_orca_state", "Drain the original Orca terminal Runs with the original Runner build and start the new build in a fresh state directory.")
		}
	}
	return nil
}
