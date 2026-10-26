package orca

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// QueryBoundTerminal reads the original terminal over the existing state RPC.
// It owns a short-lived connection, separate from the state feed and CLI cache.
func QueryBoundTerminal(ctx context.Context, environment string, p b.ProcessIdentity) (Terminal, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" {
		return Terminal{}, &b.BackendError{Code: b.BackendIdentityMismatch, Diagnostic: "original Orca runtime/terminal/incarnation changed"}
	}
	// Match the lifecycle CLI's 40s budget, including connection/handshake time.
	// A shorter caller deadline or cancellation always wins.
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	t, err := queryTerminal(ctx, environment, p)
	if err != nil {
		return t, err
	}
	return t, validateBoundTerminal(t, p)
}

func queryTerminal(ctx context.Context, environment string, p b.ProcessIdentity) (Terminal, error) {
	if err := ctx.Err(); err != nil {
		return Terminal{}, err
	}
	socket, err := openStateConnection(ctx, environment, p.RuntimeID)
	if err != nil {
		// A cancellation cannot erase already observed contrary identity facts.
		if ctx.Err() != nil && !errors.Is(err, ErrStateIdentityChanged) {
			return Terminal{}, ctx.Err()
		}
		return Terminal{}, err
	}
	defer socket.close()
	raw, err := callState(ctx, socket, "terminal.show", map[string]string{"terminal": p.Terminal})
	if ctx.Err() != nil && !errors.Is(err, ErrStateIdentityChanged) {
		return Terminal{}, ctx.Err()
	}
	if err != nil {
		return Terminal{}, err
	}
	var result struct{ Terminal Terminal }
	if err := json.Unmarshal(raw, &result); err != nil {
		return Terminal{}, errors.New("Orca original terminal query unavailable")
	}
	// The state envelope has already verified this exact runtime on every reply.
	result.Terminal.RuntimeID = p.RuntimeID
	return result.Terminal, nil
}
