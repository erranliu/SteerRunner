package orca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// The process gate covers both OS acquisition and invocation. Cancellation
// removes a ticket even when ownership and cancellation arrive together.
var cliFIFO = struct {
	sync.Mutex
	queues map[string][]chan struct{}
}{queues: map[string][]chan struct{}{}}

func enterCLI(ctx context.Context, path string) (func(), error) {
	key := strings.ToLower(filepath.Clean(path))
	ticket := make(chan struct{})
	cliFIFO.Lock()
	cliFIFO.queues[key] = append(cliFIFO.queues[key], ticket)
	if len(cliFIFO.queues[key]) == 1 {
		close(ticket)
	}
	cliFIFO.Unlock()
	leave := func() {
		cliFIFO.Lock()
		defer cliFIFO.Unlock()
		q := cliFIFO.queues[key]
		for i, item := range q {
			if item != ticket {
				continue
			}
			q = append(q[:i], q[i+1:]...)
			if len(q) == 0 {
				delete(cliFIFO.queues, key)
			} else {
				cliFIFO.queues[key] = q
				if i == 0 {
					close(q[0])
				}
			}
			break
		}
	}
	select {
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	case <-ticket:
		if err := ctx.Err(); err != nil {
			leave()
			return nil, err
		}
		return leave, nil
	}
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
	leave, err := enterCLI(ctx, path)
	if err != nil {
		return nil, &CLIQueueError{Cause: err}
	}
	defer leave()
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
