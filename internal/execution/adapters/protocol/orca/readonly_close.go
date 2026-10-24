package orca

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// CloseReadonlyTerminal reconciles one already-owned auxiliary facility. A
// complete original-runtime inventory can prove absence; only the same live
// incarnation permits continuing its close. It never creates or sends input.
func (c Client) CloseReadonlyTerminal(ctx context.Context, p b.ProcessIdentity) (b.StopReceipt, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return b.StopReceipt{}, errors.New("readonly close identity missing")
	}
	inventory := func() (bool, error) {
		t, e := c.originalTerminalInventory(ctx, p)
		if e != nil {
			return false, e
		}
		if t == nil {
			return true, nil
		}
		if !t.Connected || t.Exited() {
			return false, errors.New("readonly terminal liveness changed")
		}
		return false, nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		if e := ctx.Err(); e != nil {
			return b.StopReceipt{}, e
		}
		t, e := c.InspectBoundTerminal(ctx, b.ExecutionTarget{}, p)
		if e != nil {
			var identity *b.BackendError
			if errors.As(e, &identity) && identity.Code == b.BackendIdentityMismatch {
				return b.StopReceipt{}, e
			}
			absent, ie := inventory()
			if ie != nil {
				return b.StopReceipt{}, errors.Join(e, ie)
			}
			if absent {
				return b.StopReceipt{Disposition: "confirmed"}, nil
			}
			continue
		}
		if t.WorktreeID != p.WorktreeID {
			return b.StopReceipt{}, errors.New("readonly original terminal is not verifiably live")
		}
		if !t.Connected || t.Exited() {
			// Orca retains disconnected display cards after a PTY close. The
			// card alone proves neither liveness nor absence; reconcile the
			// same identity against the complete original worktree inventory.
			absent, ie := inventory()
			if ie != nil {
				return b.StopReceipt{}, ie
			}
			if absent {
				return b.StopReceipt{Disposition: "confirmed"}, nil
			}
			return b.StopReceipt{}, errors.New("readonly original terminal is not verifiably live")
		}
		proof, ce := c.CloseTerminalChecked(ctx, p.Terminal, p.RuntimeID)
		if ce == nil && proof.Disposition == "confirmed" {
			return proof, nil
		}
		absent, ie := inventory()
		if ie != nil {
			return b.StopReceipt{}, errors.Join(ce, ie)
		}
		if absent {
			return b.StopReceipt{Disposition: "confirmed"}, nil
		}
	}
	return b.StopReceipt{}, errors.New("original readonly close remains unverified")
}
