package livetest

import (
	"context"
	"encoding/json"
	"errors"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"slices"
	"time"
)

type RemoteWorkspaceCleanup struct {
	Runtime               string    `json:"runtime"`
	Worktree              string    `json:"worktree"`
	Verified              bool      `json:"verified"`
	TransientReadFailures int       `json:"transient_read_failures"`
	Observations          int       `json:"observations"`
	CheckedAt             time.Time `json:"checked_at"`
}

// A killed PTY can remain in Orca's metadata briefly. Only read the exact
// original workspace until an authoritative complete empty inventory arrives;
// a malformed/foreign inventory or deadline never becomes successful cleanup.
func WaitRemoteWorkspaceEmpty(ctx context.Context, client native.Client, original b.ProcessIdentity) (RemoteWorkspaceCleanup, error) {
	fact := RemoteWorkspaceCleanup{Runtime: original.RuntimeID, Worktree: original.WorktreeID}
	if original.RuntimeID == "" || original.WorktreeID == "" || original.Terminal == "" || original.Incarnation == "" {
		return fact, errors.New("remote cleanup original resource identity missing")
	}
	for {
		if err := ctx.Err(); err != nil {
			return fact, err
		}
		result, err := client.MetaCall(ctx, "terminal", "list", "--worktree", "id:"+original.WorktreeID, "--json")
		if err != nil {
			if result.RuntimeID != "" && result.RuntimeID != original.RuntimeID {
				return fact, err
			}
			if result.ErrorCode != "remote_runtime_unavailable" && result.ErrorCode != "runtime_unavailable" {
				return fact, err
			}
			fact.TransientReadFailures++
			select {
			case <-ctx.Done():
				return fact, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}
		fact.Observations++
		var inventory struct {
			Terminals []native.Terminal `json:"terminals"`
			Total     *int              `json:"totalCount"`
			Truncated *bool             `json:"truncated"`
			Hosts     *struct {
				IDs     []string `json:"hostIds"`
				Omitted []string `json:"omittedHostIds"`
			} `json:"hostScope"`
		}
		if result.RuntimeID != original.RuntimeID || json.Unmarshal(result.Payload, &inventory) != nil || inventory.Terminals == nil || inventory.Total == nil || *inventory.Total != len(inventory.Terminals) || inventory.Truncated == nil || *inventory.Truncated || inventory.Hosts == nil || len(inventory.Hosts.IDs) == 0 || inventory.Hosts.Omitted == nil || len(inventory.Hosts.Omitted) > 0 && original.ExecutionHost != "local" {
			return fact, errors.New("remote cleanup inventory incomplete or foreign")
		}
		hosts := map[string]bool{}
		for _, host := range inventory.Hosts.IDs {
			if host == "" || hosts[host] {
				return fact, errors.New("remote cleanup host scope invalid")
			}
			hosts[host] = true
		}
		if !hosts["local"] || slices.Contains(inventory.Hosts.Omitted, "local") {
			return fact, errors.New("remote cleanup original host missing")
		}
		seen := map[string]bool{}
		for _, term := range inventory.Terminals {
			if term.Handle == "" || term.IncarnationID == "" || seen[term.Handle] || term.WorktreeID != original.WorktreeID || term.Handle == original.Terminal && term.IncarnationID != original.Incarnation {
				return fact, errors.New("remote cleanup inventory resource scope changed")
			}
			seen[term.Handle] = true
		}
		if len(inventory.Terminals) == 0 {
			fact.Verified = true
			fact.CheckedAt = time.Now().UTC()
			return fact, nil
		}
		select {
		case <-ctx.Done():
			return fact, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
