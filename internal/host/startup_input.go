package host

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
)

func pendingStartPath(dir string) string { return filepath.Join(dir, "service", "pending-stops.json") }

func savePendingStart(dir string, input *contracts.ServiceStartInput) error {
	if input == nil {
		return nil
	}
	// Never silently remove a previously saved cancellation during a retry.
	if raw, err := os.ReadFile(pendingStartPath(dir)); err == nil {
		old, fault := contracts.ReadServiceStartInput(raw)
		if fault != nil {
			return fault
		}
		for _, prior := range old.PendingStops {
			found := false
			for _, next := range input.PendingStops {
				if next.RunID == prior.RunID && next.Request.RequestID == prior.Request.RequestID {
					if storage.RequestDigest("stop", next.RunID, next.Request) != storage.RequestDigest("stop", prior.RunID, prior.Request) {
						return contracts.Fail(5, "request_conflict", "Pending stop retry changed its original input.")
					}
					found = true
				}
			}
			if !found {
				return contracts.Fail(5, "request_conflict", "Startup must preserve unresolved original stop requests.")
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, _ := json.Marshal(input)
	tmp, err := os.CreateTemp(filepath.Dir(pendingStartPath(dir)), "pending-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, err = tmp.Write(raw)
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(name, pendingStartPath(dir))
}

func loadPendingStart(dir string) (*contracts.ServiceStartInput, error) {
	raw, err := os.ReadFile(pendingStartPath(dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	input, fault := contracts.ReadServiceStartInput(raw)
	if fault != nil {
		return nil, fault
	}
	return input, nil
}
