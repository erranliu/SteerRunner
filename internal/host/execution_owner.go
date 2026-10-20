package host

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
)

// Pin the host image outside the installation directory. Updates must not
// replace or delete an executable still referenced by a native execution.
func launchExecutionOwner(ctx context.Context, stateDir, ownerDir string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	sha, err := executableDigest(executable)
	if err != nil {
		return err
	}
	imageDir := filepath.Join(stateDir, "runtimes", sha)
	if err = os.MkdirAll(imageDir, 0700); err != nil {
		return err
	}
	image := filepath.Join(imageDir, "steerrunner.exe")
	if existing, e := executableDigest(image); e == nil {
		if existing != sha {
			return errors.New("pinned execution owner image changed")
		}
	} else if !os.IsNotExist(e) {
		return e
	} else {
		source, e := os.Open(executable)
		if e != nil {
			return e
		}
		defer source.Close()
		tmp, e := os.CreateTemp(imageDir, "owner-*.tmp")
		if e != nil {
			return e
		}
		name := tmp.Name()
		defer os.Remove(name)
		_, e = io.Copy(tmp, source)
		if e == nil {
			e = tmp.Sync()
		}
		closeErr := tmp.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		if e = replaceFile(name, image); e != nil {
			return e
		}
	}
	for _, name := range []string{"logs", "service"} {
		if err = os.MkdirAll(filepath.Join(ownerDir, name), 0700); err != nil {
			return err
		}
	}
	if err = startDetached(ctx, image, ownerDir, contracts.NewID()); err != nil {
		return err
	}
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err = os.Stat(filepath.Join(ownerDir, "owner-ready.json")); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func executionOwnerDirectory(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, codex.ExecutionOwnerConfigFile))
	return err == nil
}
