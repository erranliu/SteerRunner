package orca

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestQueryBoundTerminalMissingIdentityDoesNotConnect(t *testing.T) {
	for _, field := range []string{"runtime", "terminal", "incarnation"} {
		t.Run(field, func(t *testing.T) {
			var calls atomic.Int32
			localIPCFixture(t, func(net.Conn, ipcTestRequest) { calls.Add(1) })
			p := b.ProcessIdentity{RuntimeID: "runtime", Terminal: "terminal", Incarnation: "incarnation"}
			switch field {
			case "runtime":
				p.RuntimeID = ""
			case "terminal":
				p.Terminal = ""
			case "incarnation":
				p.Incarnation = ""
			}
			_, err := QueryBoundTerminal(context.Background(), "", p)
			var identity *b.BackendError
			if !errors.As(err, &identity) || identity.Code != b.BackendIdentityMismatch || calls.Load() != 0 {
				t.Fatal(err, calls.Load())
			}
		})
	}
}

func TestQueryBoundTerminalHandshakeCancellation(t *testing.T) {
	for _, scenario := range []string{"cancel", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			entered := make(chan struct{}, 1)
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method != "status.get" {
					t.Errorf("unexpected dispatch %s", req.Method)
				}
				entered <- struct{}{} // leave the bootstrap reply pending
			})
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if scenario == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := QueryBoundTerminal(ctx, "", b.ProcessIdentity{RuntimeID: "runtime", Terminal: "terminal", Incarnation: "incarnation"})
				result <- err
			}()
			if scenario == "cancel" {
				select {
				case <-entered:
					cancel()
				case <-time.After(2 * time.Second):
					t.Fatal("handshake did not start")
				}
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("want %v got %v", want, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("handshake did not cancel")
			}
		})
	}
}

// Cancel exactly when the query inspects cancellation after bootstrap has
// already proved a different runtime. Err still follows the parent context.
type cancelOnSecondErrContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *cancelOnSecondErrContext) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestTerminalQueryCancellationCannotHideKnownIdentityConflict(t *testing.T) {
	var calls atomic.Int32
	localIPCFixture(t, func(net.Conn, ipcTestRequest) { calls.Add(1) })
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &cancelOnSecondErrContext{Context: parent, cancel: cancel}
	// queryTerminal is also the existing native-input evidence path. Contrary
	// runtime evidence must survive cancellation, so callers can retain it.
	_, err := queryTerminal(ctx, "", b.ProcessIdentity{RuntimeID: "different-runtime", Terminal: "terminal", Incarnation: "incarnation"})
	if !errors.Is(parent.Err(), context.Canceled) || !errors.Is(err, ErrStateIdentityChanged) || calls.Load() != 0 {
		t.Fatalf("identity conflict lost to cancellation: %v, context=%v, calls=%d", err, parent.Err(), calls.Load())
	}
}
