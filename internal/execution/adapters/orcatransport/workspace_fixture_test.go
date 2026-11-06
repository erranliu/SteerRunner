package orcatransport

import (
	"context"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Shared workspace/discovery tests use the supported execution protocol. The
// new workspace has no Agent terminal; Agent creation is a separate operation.
func workspaceFixtureOn(t *testing.T, environment string) (*Lifecycle, *fakeHost, b.LifecycleSnapshot) {
	t.Helper()
	l, f, s := structuredFixtureOn(t, environment)
	original := l.client
	l.client = native.New(native.Config{Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "terminal" && args[1] == "list" {
			// Keep the shared host's full command history, with a current empty workspace.
			if _, err := original.Raw(ctx, d, args...); err != nil {
				return nil, err
			}
			return []byte(`{"ok":true,"result":{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"hostIds":["local"],"omittedHostIds":[]}},"_meta":{"runtimeId":"runtime"}}`), nil
		}
		return original.Raw(ctx, d, args...)
	}})
	return l, f, s
}
