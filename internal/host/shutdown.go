package host

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func shutdownPath(dir string) string { return filepath.Join(dir, "service", "shutdown.json") }

func writeShutdown(dir string, value contracts.ServiceShutdown) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Join(dir, "service"), "shutdown-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(file.Name(), shutdownPath(dir))
}

func readShutdown(dir string) (*contracts.ServiceShutdown, error) {
	data, err := os.ReadFile(shutdownPath(dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result contracts.ServiceShutdown
	if len(data) > 1<<20 || contracts.DecodeStrict(data, &result) != nil || result.StateDir != dir || result.InstanceID == "" || result.ExecutableSHA256 == "" {
		return nil, contracts.Fail(4, "service_unavailable", "Original service shutdown evidence is invalid.")
	}
	return &result, nil
}
