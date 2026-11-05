package orcatransport

import (
	"context"

	"errors"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestTerminalResourceErrorMapping(t *testing.T) {
	p := b.ProcessIdentity{RuntimeID: "runtime", Terminal: "original", Incarnation: "incarnation"}
	client := native.New(native.Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
		return []byte(`{"ok":true,"result":{"terminal":{"handle":"replacement","incarnationId":"incarnation"},"close":{"handle":"replacement","ptyKilled":true}},"_meta":{"runtimeId":"runtime"}}`), nil
	}})
	_, inspectErr := InspectBoundTerminal(context.Background(), client, p)
	_, closeErr := CloseTerminalChecked(context.Background(), client, p.Terminal, p.RuntimeID)
	for _, err := range []error{inspectErr, closeErr} {
		var mapped *b.BackendError
		if !errors.As(err, &mapped) || mapped.Code != b.BackendIdentityMismatch {
			t.Fatal("original execution identity mapping lost", err)
		}
	}
}
