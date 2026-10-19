package orca

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestTerminalStoppedRequiresCompleteOriginalHost(t *testing.T) {
	p := b.ProcessIdentity{Kind: "orca_terminal", RuntimeID: "original", Terminal: "terminal", Incarnation: "inc", WorktreeID: "worktree"}
	for _, name := range []string{"absent", "exited", "unrelated-omitted", "active", "disconnected", "incarnation", "runtime", "worktree", "duplicate", "truncated", "count", "missing-count", "missing-host", "omitted-original", "missing-terminals", "offline", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			runtime := p.RuntimeID
			term := Terminal{Handle: p.Terminal, IncarnationID: p.Incarnation, WorktreeID: p.WorktreeID, Status: "exited"}
			v := map[string]any{"terminals": []Terminal{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}
			switch name {
			case "exited", "active", "disconnected", "incarnation", "worktree", "duplicate":
				if name == "active" || name == "disconnected" {
					term.Status = "running"
				}
				if name == "incarnation" {
					term.IncarnationID = "replacement"
				}
				if name == "worktree" {
					term.WorktreeID = "other"
				}
				terms := []Terminal{term}
				if name == "duplicate" {
					terms = append(terms, term)
				}
				v["terminals"] = terms
				v["totalCount"] = len(terms)
			case "runtime":
				runtime = "replacement"
			case "truncated":
				v["truncated"] = true
			case "count":
				v["totalCount"] = 1
			case "missing-count":
				delete(v, "totalCount")
			case "missing-host":
				v["hostScope"] = map[string]any{"hostIds": []string{"runtime:other"}, "omittedHostIds": []string{}}
			case "omitted-original":
				v["hostScope"] = map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{"local"}}
			case "unrelated-omitted":
				v["hostScope"] = map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{"runtime:other"}}
			case "missing-terminals":
				delete(v, "terminals")
			}
			calls := 0
			c := New(Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				calls++
				if len(args) != 5 || args[0] != "terminal" || args[1] != "list" || args[3] != "id:"+p.WorktreeID {
					t.Fatalf("unexpected effect: %v", args)
				}
				if name == "offline" {
					return nil, errors.New("unreachable")
				}
				return json.Marshal(map[string]any{"ok": true, "result": v, "_meta": map[string]string{"runtimeId": runtime}})
			}})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "cancelled" {
				cancel()
			}
			err := c.VerifyTerminalStopped(ctx, p)
			want := name == "absent" || name == "exited" || name == "unrelated-omitted"
			if (err == nil) != want || calls != 1 {
				t.Fatalf("%s: %v calls=%d", name, err, calls)
			}
			// The recovery close must apply exactly the same completeness/identity
			// checks before using an absent original PTY as cleanup evidence.
			calls = 0
			original, inventoryErr := c.originalTerminalInventory(ctx, p)
			inventoryPass := want || name == "active" || name == "disconnected"
			if (inventoryErr == nil) != inventoryPass || calls != 1 {
				t.Fatal("inventory consumers diverged", original, inventoryErr, calls)
			}
		})
	}
}
