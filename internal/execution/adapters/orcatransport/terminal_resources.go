package orcatransport

import (
	"context"
	"errors"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func InspectBoundTerminal(ctx context.Context, client native.Client, p b.ProcessIdentity) (native.Terminal, error) {
	t, err := client.InspectTerminal(ctx, native.TerminalIdentity{RuntimeID: p.RuntimeID, Handle: p.Terminal, IncarnationID: p.Incarnation})
	var identity *native.TerminalIdentityError
	if errors.As(err, &identity) {
		err = &b.BackendError{Code: b.BackendIdentityMismatch, Diagnostic: err.Error(), Cause: err}
	}
	return t, err
}

func CloseTerminalChecked(ctx context.Context, client native.Client, handle, runtime string) (b.StopReceipt, error) {
	_, closed, err := client.CloseTerminal(ctx, handle, runtime)
	var identity *native.TerminalCloseIdentityError
	if errors.As(err, &identity) {
		return b.StopReceipt{}, &b.BackendError{Code: b.BackendIdentityMismatch, Diagnostic: identity.Error()}
	}
	if err != nil {
		return b.StopReceipt{}, err
	}
	if closed.PtyKilled {
		return b.StopReceipt{Disposition: "confirmed"}, nil
	}
	return b.StopReceipt{Disposition: "accepted"}, nil
}

func VerifyTerminalStopped(ctx context.Context, client native.Client, p b.ProcessIdentity) error {
	return VerifyTerminalsStopped(ctx, client, []b.ProcessIdentity{p})
}

// Scope and stopped interpretation belong to execution; the inventory itself
// is an Orca resource fact and does not prove helper/root or descendant exit.
func VerifyTerminalsStopped(ctx context.Context, client native.Client, originals []b.ProcessIdentity) error {
	if len(originals) == 0 {
		return errors.New("original terminal stop identities missing")
	}
	first := originals[0]
	for _, p := range originals {
		if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" || p.RuntimeID != first.RuntimeID || p.WorktreeID != first.WorktreeID {
			return errors.New("original terminal stop identities have incomplete or different scopes")
		}
	}
	inventory, err := client.CompleteWorktreeInventory(ctx, native.WorktreeInventoryScope{RuntimeID: first.RuntimeID, WorktreeID: first.WorktreeID})
	if err != nil {
		return err
	}
	for _, p := range originals {
		t, err := originalInventoryTerminal(inventory, p)
		if err != nil {
			return err
		}
		if t != nil && !t.Exited() {
			return errors.New("original terminal is still active")
		}
	}
	return ctx.Err()
}

func originalTerminalInventory(ctx context.Context, client native.Client, p b.ProcessIdentity) (*native.Terminal, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return nil, errors.New("original terminal identity is incomplete")
	}
	inventory, err := client.CompleteWorktreeInventory(ctx, native.WorktreeInventoryScope{RuntimeID: p.RuntimeID, WorktreeID: p.WorktreeID})
	if err != nil {
		return nil, err
	}
	return originalInventoryTerminal(inventory, p)
}

func originalInventoryTerminal(inventory map[string]native.Terminal, p b.ProcessIdentity) (*native.Terminal, error) {
	t, found := inventory[p.Terminal]
	if !found {
		return nil, nil
	}
	if t.IncarnationID != p.Incarnation {
		return nil, errors.New("original terminal incarnation changed")
	}
	return &t, nil
}
