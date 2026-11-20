package codexworker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Only initialization is serialized. Each execution still owns its app-server,
// environment, endpoint and session. The OS releases the lock if a caller exits.
func acquireStartup(ctx context.Context, home string) (func(), error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if home == "" {
		home = EnvironmentHome()
	}
	if !filepath.IsAbs(home) {
		return nil, fmt.Errorf("Codex initialization requires an absolute home")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".steerrunner-startup.lock")
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("Codex initialization is busy: %w", err)
		}
		file, busy, err := tryStartupLock(path)
		if err != nil {
			return nil, fmt.Errorf("lock Codex initialization: %w", err)
		}
		if !busy {
			var once sync.Once
			return func() { once.Do(func() { _ = file.Close() }) }, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("Codex initialization is busy: %w", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}
