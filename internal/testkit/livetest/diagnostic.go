package livetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"time"
)

// DiagnosticRead retries only explicit connection failures of public read APIs.
// Mutations, malformed envelopes and identity changes are never replayed here.
func DiagnosticRead(ctx context.Context, call func(context.Context, ...string) ([]byte, error), args ...string) ([]byte, error) {
	read := len(args) >= 2 && (args[0] == "terminal" && (args[1] == "show" || args[1] == "read" || args[1] == "list") || args[0] == "repo" && args[1] == "list" || args[0] == "worktree" && (args[1] == "list" || args[1] == "show"))
	for attempt := 0; ; attempt++ {
		raw, err := call(ctx, args...)
		v, parseErr := orca.ParseEnvelope(raw)
		if err == nil && parseErr == nil {
			return raw, nil
		}
		if !read || attempt == 2 || v.Error == nil || (v.Error.Code != "remote_runtime_unavailable" && v.Error.Code != "runtime_unavailable") {
			return raw, errors.Join(err, parseErr)
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return raw, ctx.Err()
		case <-timer.C:
		}
	}
}

// CloseAuxiliaryTerminal closes only an original test-control PTY. After a lost
// response it requires a complete original-runtime inventory: absence confirms
// cleanup, while the same still-live incarnation permits continuing this exact
// facility-close intent. This never sends model input or creates a resource.
func CloseAuxiliaryTerminal(ctx context.Context, client orca.Client, original b.ProcessIdentity) error {
	if original.RuntimeID == "" || original.Terminal == "" || original.Incarnation == "" || original.WorktreeID == "" {
		return errors.New("missing auxiliary terminal identity")
	}
	for attempt := 0; attempt < 3; attempt++ {
		terminal, err := client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, original)
		if err != nil || terminal.WorktreeID != original.WorktreeID || !terminal.Connected || terminal.Exited() {
			return errors.Join(err, errors.New("auxiliary original PTY is not verifiably live"))
		}
		proof, closeErr := client.CloseTerminalChecked(ctx, original.Terminal, original.RuntimeID)
		if closeErr == nil && proof.Disposition == "confirmed" {
			return nil
		}
		result, err := client.MetaCall(ctx, "terminal", "list", "--worktree", "id:"+original.WorktreeID, "--json")
		var inventory struct {
			Terminals []orca.Terminal `json:"terminals"`
			Truncated *bool           `json:"truncated"`
			Total     *int            `json:"totalCount"`
			HostScope *struct {
				Hosts   []string `json:"hostIds"`
				Omitted []string `json:"omittedHostIds"`
			} `json:"hostScope"`
		}
		if err != nil || result.RuntimeID != original.RuntimeID || json.Unmarshal(result.Payload, &inventory) != nil || inventory.Truncated == nil || *inventory.Truncated || inventory.Total == nil || *inventory.Total != len(inventory.Terminals) || inventory.HostScope == nil || len(inventory.HostScope.Hosts) == 0 || len(inventory.HostScope.Omitted) != 0 {
			return errors.Join(closeErr, err, errors.New("auxiliary close outcome unknown; inventory incomplete or foreign"))
		}
		present := false
		for _, current := range inventory.Terminals {
			if current.Handle == original.Terminal {
				present = true
				if current.IncarnationID != original.Incarnation || current.WorktreeID != original.WorktreeID || !current.Connected || current.Exited() {
					return errors.New("auxiliary incarnation/liveness changed; no close repeated")
				}
			}
		}
		if !present {
			return nil
		}
	}
	return fmt.Errorf("original auxiliary close remained unconfirmed after three identity-bound attempts: %s", original.Terminal)
}
