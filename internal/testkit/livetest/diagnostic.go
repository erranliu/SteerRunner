package livetest

import (
	"context"
	"errors"

	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"time"
)

// DiagnosticRead retries only explicit connection failures of public read APIs.
// Mutations, malformed envelopes and identity changes are never replayed here.
func DiagnosticRead(ctx context.Context, call func(context.Context, ...string) ([]byte, error), args ...string) ([]byte, error) {
	read := len(args) >= 2 && (args[0] == "terminal" && (args[1] == "show" || args[1] == "read" || args[1] == "list") || args[0] == "repo" && args[1] == "list" || args[0] == "worktree" && (args[1] == "list" || args[1] == "show"))
	for attempt := 0; ; attempt++ {
		raw, err := call(ctx, args...)
		v, parseErr := orca.ParseEnvelope(raw)
		if err == nil && parseErr == nil {
			return raw, nil
		}
		if !read || attempt == 2 || v.Error == nil || (v.Error.Code != "remote_runtime_unavailable" && v.Error.Code != "runtime_unavailable") {
			return raw, errors.Join(err, parseErr)
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return raw, ctx.Err()
		case <-timer.C:
		}
	}
}

// CloseAuxiliaryTerminal closes only an original test-control PTY. After a lost
// response it requires a complete original-runtime inventory: absence confirms
// cleanup, while the same still-live incarnation permits continuing this exact
// facility-close intent. This never sends model input or creates a resource.
func CloseAuxiliaryTerminal(ctx context.Context, client orca.Client, original b.ProcessIdentity) error {
	_, err := client.CloseReadonlyTerminal(ctx, original)
	return err
}
