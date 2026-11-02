package orca

import (
	"context"
	"encoding/json"
	"errors"
)

type InventoryHosts struct {
	IDs     []string `json:"hostIds"`
	Omitted []string `json:"omittedHostIds"`
}

type TerminalInventory struct {
	Terminals             []Terminal      `json:"terminals"`
	Truncated             *bool           `json:"truncated"`
	Total                 *int            `json:"totalCount"`
	Hosts                 *InventoryHosts `json:"hostScope"`
	decoded, basicDecoded bool
}

func (i TerminalInventory) Decoded() bool      { return i.decoded }
func (i TerminalInventory) BasicDecoded() bool { return i.basicDecoded }
func WorktreeTerminalListArgs(selector string) []string {
	return []string{"terminal", "list", "--worktree", selector, "--json"}
}
func DecodeTerminalInventory(result Result) TerminalInventory {
	var value TerminalInventory
	err := json.Unmarshal(result.Payload, &value)
	value.decoded = err == nil
	// The sole-terminal consumer historically read only these two fields.
	// Unconsumed total/host extensions must not change that reader's contract.
	var basic struct {
		Terminals []Terminal
		Truncated bool
	}
	value.basicDecoded = json.Unmarshal(result.Payload, &basic) == nil
	return value
}

type WorktreeInventoryScope struct{ RuntimeID, WorktreeID string }

// CompleteWorktreeInventory returns complete original-host resource facts,
// without deciding which identities satisfy an execution's stopped contract.
func (c Client) CompleteWorktreeInventory(ctx context.Context, scope WorktreeInventoryScope) (map[string]Terminal, error) {
	if scope.RuntimeID == "" || scope.WorktreeID == "" {
		return nil, errors.New("original terminal identity is incomplete")
	}
	r, err := c.MetaCall(ctx, WorktreeTerminalListArgs("id:"+scope.WorktreeID)...)
	v := DecodeTerminalInventory(r)
	if err != nil || r.RuntimeID != scope.RuntimeID || !v.Decoded() || v.Terminals == nil || v.Truncated == nil || *v.Truncated || v.Total == nil || *v.Total != len(v.Terminals) || v.Hosts == nil || len(v.Hosts.IDs) == 0 || v.Hosts.Omitted == nil {
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
		if t.Handle == "" || t.IncarnationID == "" || t.WorktreeID != scope.WorktreeID || seen[t.Handle] {
			return nil, errors.New("original terminal stop inventory scope changed")
		}
		seen[t.Handle] = true
		inventory[t.Handle] = t
	}
	return inventory, ctx.Err()
}
