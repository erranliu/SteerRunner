//go:build windows

package codexworker

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// This read precedes any owner request. A transient discovery-file sharing
// collision must not strand an unsent intent on the receipt-only recovery path.
// Retry only that read, within both the original context and a fixed 500ms cap.
func readOwnerDispatchFile(ctx context.Context, path string) ([]byte, error) {
	until := time.Now().Add(500 * time.Millisecond)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := os.ReadFile(path)
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || !time.Now().Before(until) {
			return raw, err
		}
		timer := time.NewTimer(min(5*time.Millisecond, time.Until(until)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
