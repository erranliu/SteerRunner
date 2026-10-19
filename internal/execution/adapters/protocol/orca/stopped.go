package orca

import (
	"context"
	"encoding/json"
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// VerifyTerminalStopped only queries the original runtime's complete inventory.
// Missing/disconnected hosts and replacement incarnations never prove absence.
// It must not create, close, adopt, or send input to a terminal.
func (c Client) VerifyTerminalStopped(ctx context.Context, p b.ProcessIdentity) error {
	return c.VerifyTerminalsStopped(ctx, []b.ProcessIdentity{p})
}

// VerifyTerminalsStopped checks all original identities against one complete
// inventory from their original runtime/worktree. It never creates resources.
func (c Client) VerifyTerminalsStopped(ctx context.Context, originals []b.ProcessIdentity) error {
	if len(originals) == 0 {
		return errors.New("original terminal stop identities missing")
	}
	first := originals[0]
	for _, p := range originals {
		if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" || p.RuntimeID != first.RuntimeID || p.WorktreeID != first.WorktreeID {
			return errors.New("original terminal stop identities have incomplete or different scopes")
		}
	}
	inventory, err := c.originalWorktreeInventory(ctx, first)
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

// Both offline verification and readonly close use the same original-host
// inventory proof. A nil terminal means absence, not native writer-tree exit.
func (c Client) originalTerminalInventory(ctx context.Context, p b.ProcessIdentity) (*Terminal, error) {
	inventory, err := c.originalWorktreeInventory(ctx, p)
	if err != nil {
		return nil, err
	}
	return originalInventoryTerminal(inventory, p)
}

func originalInventoryTerminal(inventory map[string]Terminal, p b.ProcessIdentity) (*Terminal, error) {
	t, found := inventory[p.Terminal]
	if !found {
		return nil, nil
	}
	if t.IncarnationID != p.Incarnation {
		return nil, errors.New("original terminal incarnation changed")
	}
	return &t, nil
}

func (c Client) originalWorktreeInventory(ctx context.Context, p b.ProcessIdentity) (map[string]Terminal, error) {
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return nil, errors.New("original terminal identity is incomplete")
	}
	r, err := c.MetaCall(ctx, "terminal", "list", "--worktree", "id:"+p.WorktreeID, "--json")
	var v struct {
		Terminals []Terminal `json:"terminals"`
		Truncated *bool      `json:"truncated"`
		Total     *int       `json:"totalCount"`
		Hosts     *struct {
			IDs     []string `json:"hostIds"`
			Omitted []string `json:"omittedHostIds"`
		} `json:"hostScope"`
	}
	if err != nil || r.RuntimeID != p.RuntimeID || json.Unmarshal(r.Payload, &v) != nil || v.Terminals == nil || v.Truncated == nil || *v.Truncated || v.Total == nil || *v.Total != len(v.Terminals) || v.Hosts == nil || len(v.Hosts.IDs) == 0 || v.Hosts.Omitted == nil {
		return nil, errors.Join(err, errors.New("original terminal stop inventory is incomplete or foreign"))
	}
	seen := map[string]bool{}
	for _, host := range v.Hosts.IDs {
		if host == "" || seen[host] {
			return nil, errors.New("original terminal host scope is invalid")
		}
		seen[host] = true
	}
	// Environment routing executes on the selected runtime, whose own PTYs
	// are host "local". Its unrelated paired hosts may be omitted; they cannot
	// own an identity bound to this runtime. Never infer absence without local.
	if !seen["local"] {
		return nil, errors.New("original terminal host was not observed")
	}
	for _, host := range v.Hosts.Omitted {
		if host == "" || seen[host] {
			return nil, errors.New("original terminal host scope is inconsistent")
		}
	}
	seen = map[string]bool{}
	inventory := map[string]Terminal{}
	for _, t := range v.Terminals {
		if t.Handle == "" || t.IncarnationID == "" || t.WorktreeID != p.WorktreeID || seen[t.Handle] {
			return nil, errors.New("original terminal stop inventory scope changed")
		}
		seen[t.Handle] = true
		inventory[t.Handle] = t
	}
	return inventory, ctx.Err()
}
