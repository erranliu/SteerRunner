package orca

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// TerminalIdentity identifies an exact terminal incarnation on an Orca runtime.
type TerminalIdentity struct {
	RuntimeID, Handle, IncarnationID string
}

// TerminalIdentityError reports missing or contrary native terminal identity.
type TerminalIdentityError struct{}

func (*TerminalIdentityError) Error() string {
	return "original Orca runtime/terminal/incarnation changed"
}

func (*TerminalIdentityError) Unwrap() error { return ErrStateIdentityChanged }

func validateTerminalIdentity(t Terminal, p TerminalIdentity) error {
	if p.RuntimeID == "" || p.Handle == "" || p.IncarnationID == "" || t.RuntimeID != p.RuntimeID || t.Handle != p.Handle || t.IncarnationID != p.IncarnationID {
		return &TerminalIdentityError{}
	}
	return nil
}

// QueryBoundTerminal reads the original terminal over the existing state RPC.
// It owns a short-lived connection, separate from the state feed and CLI cache.
func QueryBoundTerminal(ctx context.Context, environment string, p TerminalIdentity) (Terminal, error) {
	if p.RuntimeID == "" || p.Handle == "" || p.IncarnationID == "" {
		return Terminal{}, &TerminalIdentityError{}
	}
	// Match the lifecycle CLI's 40s budget, including connection/handshake time.
	// A shorter caller deadline or cancellation always wins.
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	t, err := queryTerminal(ctx, environment, p)
	if err != nil {
		return t, err
	}
	return t, validateTerminalIdentity(t, p)
}

func queryTerminal(ctx context.Context, environment string, p TerminalIdentity) (Terminal, error) {
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
	raw, err := callState(ctx, socket, "terminal.show", map[string]string{"terminal": p.Handle})
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
