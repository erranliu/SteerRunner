package orca

import (
	"context"
	"encoding/json"
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
		r, e := c.MetaCall(ctx, "terminal", "list", "--worktree", "id:"+p.WorktreeID, "--json")
		var v struct {
			Terminals []Terminal `json:"terminals"`
			Truncated *bool      `json:"truncated"`
			Total     *int       `json:"totalCount"`
			Hosts     *struct {
				IDs     []string `json:"hostIds"`
				Omitted []string `json:"omittedHostIds"`
			} `json:"hostScope"`
		}
		if e != nil || r.RuntimeID != p.RuntimeID || json.Unmarshal(r.Payload, &v) != nil || v.Truncated == nil || *v.Truncated || v.Total == nil || *v.Total != len(v.Terminals) || v.Hosts == nil || len(v.Hosts.IDs) == 0 || len(v.Hosts.Omitted) != 0 {
			return false, errors.Join(e, errors.New("readonly close inventory incomplete or foreign"))
		}
		seen := map[string]bool{}
		for _, t := range v.Terminals {
			if t.Handle == "" || t.WorktreeID != p.WorktreeID || seen[t.Handle] {
				return false, errors.New("readonly close inventory scope changed")
			}
			seen[t.Handle] = true
			if t.Handle == p.Terminal {
				if t.IncarnationID != p.Incarnation || t.WorktreeID != p.WorktreeID || !t.Connected || t.Exited() {
					return false, errors.New("readonly terminal incarnation or liveness changed")
				}
				return false, nil
			}
		}
		return true, nil
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
		if t.WorktreeID != p.WorktreeID || !t.Connected || t.Exited() {
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
