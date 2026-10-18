package orca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// CLIQueueError proves the callback never ran. A command's unknown transport
// outcome is different and must never authorize a resend.
type CLIQueueError struct{ Cause error }

func (e *CLIQueueError) Error() string { return "Orca CLI queue ended before command invocation" }
func (e *CLIQueueError) Unwrap() error { return e.Cause }
func IsCLIQueueError(err error) bool {
	var queued *CLIQueueError
	return errors.As(err, &queued)
}

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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, &CLIQueueError{Cause: err}
		}
		return nil, err
	}
	defer release()
	if err = ctx.Err(); err != nil {
		return nil, &CLIQueueError{Cause: err}
	}
	return invoke(ctx)
}
