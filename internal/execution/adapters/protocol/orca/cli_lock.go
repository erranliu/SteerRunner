package orca

import (
	"context"
	"os"
	"path/filepath"
)

// SerializeCLI protects Orca's shared environments file from concurrent CLI
// refresh/rename on Windows. It fences command processes, not remote workers or
// state streams. Waiting consumes the original context and never retries a call.
// Wrap the outer CLI invocation only; a diagnostic wrapper's child inherits
// that invocation and must not acquire this lock again.
func SerializeCLI(ctx context.Context, invoke func(context.Context) ([]byte, error)) ([]byte, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "SteerRunner", "orca-cli.lock")
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	release, err := lockCLIFile(ctx, path)
	if err != nil {
		return nil, err
	}
	defer release()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return invoke(ctx)
}
