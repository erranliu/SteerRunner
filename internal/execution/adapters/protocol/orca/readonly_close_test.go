package orca

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestReadonlyCloseStaleDisconnectedCardUsesOriginalInventory(t *testing.T) {
	p := b.ProcessIdentity{Kind: "orca_native_control", RuntimeID: "original", Terminal: "control", Incarnation: "inc", WorktreeID: "worktree"}
	for _, mode := range []string{"absent", "still-listed", "truncated", "missing-local", "foreign-runtime", "replacement", "foreign-worktree"} {
		t.Run(mode, func(t *testing.T) {
			queries, closes := 0, 0
			c := New(Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				term := Terminal{Handle: p.Terminal, IncarnationID: p.Incarnation, WorktreeID: p.WorktreeID, Connected: false}
				var result any
				runtime := p.RuntimeID
				switch args[0] + " " + args[1] {
				case "terminal show":
					if mode == "replacement" {
						term.IncarnationID = "replacement"
					}
					if mode == "foreign-worktree" {
						term.WorktreeID = "foreign"
					}
					result = map[string]any{"terminal": term}
				case "terminal list":
					queries++
					if len(args) != 5 || args[3] != "id:"+p.WorktreeID {
						t.Fatal("query escaped original worktree", args)
					}
					terms := []Terminal{}
					if mode == "still-listed" {
						terms = append(terms, term)
					}
					hosts := []string{"local"}
					if mode == "missing-local" {
						hosts = []string{"other"}
					}
					if mode == "foreign-runtime" {
						runtime = "foreign"
					}
					result = map[string]any{"terminals": terms, "totalCount": len(terms), "truncated": mode == "truncated", "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": []string{}}}
				default:
					closes++
					t.Fatal("stale closed card repeated an effect", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}})
			}})
			proof, err := c.CloseReadonlyTerminal(context.Background(), p)
			if (err == nil) != (mode == "absent") || mode == "absent" && proof.Disposition != "confirmed" || closes != 0 {
				t.Fatal("stale card reconciliation", proof, err, queries, closes)
			}
			if mode == "absent" && queries != 1 {
				t.Fatal("absence requires complete original inventory", queries)
			}
		})
	}
}
