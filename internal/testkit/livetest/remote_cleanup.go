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
	Runtime      string    `json:"runtime"`
	Worktree     string    `json:"worktree"`
	Verified     bool      `json:"verified"`
	Observations int       `json:"observations"`
	CheckedAt    time.Time `json:"checked_at"`
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
			return fact, err
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
		decoded := json.Unmarshal(result.Payload, &inventory) == nil
		hostsComplete := inventory.Hosts != nil && len(inventory.Hosts.IDs) > 0 && len(inventory.Hosts.Omitted) == 0
		if original.ExecutionHost == "local" {
			// Local IPC reports paired but unqueried remote hosts as omitted.
			// Only the explicitly frozen local owner is this Run's cleanup scope.
			hostsComplete = inventory.Hosts != nil && slices.Contains(inventory.Hosts.IDs, "local") && !slices.Contains(inventory.Hosts.Omitted, "local")
		}
		if result.RuntimeID != original.RuntimeID || !decoded || inventory.Total == nil || *inventory.Total != len(inventory.Terminals) || inventory.Truncated == nil || *inventory.Truncated || !hostsComplete {
			return fact, errors.New("remote cleanup inventory incomplete or foreign")
		}
		seen := map[string]bool{}
		for _, term := range inventory.Terminals {
			if term.Handle == "" || seen[term.Handle] || term.WorktreeID != original.WorktreeID {
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
