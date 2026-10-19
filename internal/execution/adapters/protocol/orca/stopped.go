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
	if p.RuntimeID == "" || p.Terminal == "" || p.Incarnation == "" || p.WorktreeID == "" {
		return errors.New("original terminal stop identity is incomplete")
	}
	t, err := c.originalTerminalInventory(ctx, p)
	if err != nil {
		return err
	}
	if t != nil && !t.Exited() {
		return errors.New("original terminal is still active")
	}
	return nil
}

// Both offline verification and readonly close use the same original-host
// inventory proof. A nil terminal means absence, not native writer-tree exit.
func (c Client) originalTerminalInventory(ctx context.Context, p b.ProcessIdentity) (*Terminal, error) {
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
	var original *Terminal
	for _, t := range v.Terminals {
		if t.Handle == "" || t.IncarnationID == "" || t.WorktreeID != p.WorktreeID || seen[t.Handle] {
			return nil, errors.New("original terminal stop inventory scope changed")
		}
		seen[t.Handle] = true
		if t.Handle == p.Terminal {
			if t.IncarnationID != p.Incarnation {
				return nil, errors.New("original terminal incarnation changed")
			}
			copy := t
			original = &copy
		}
	}
	return original, ctx.Err()
}
