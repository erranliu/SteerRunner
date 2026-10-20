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

func TestTerminalsStoppedUseOneCompleteOriginalInventory(t *testing.T) {
	for _, mode := range []string{"absent", "exited", "active-reader", "changed-incarnation", "foreign-runtime", "foreign-worktree", "missing-identity", "truncated", "omitted-local", "empty-identities"} {
		t.Run(mode, func(t *testing.T) {
			originals := []b.ProcessIdentity{}
			for _, id := range []string{"business", "reader-one", "reader-two"} {
				originals = append(originals, b.ProcessIdentity{RuntimeID: "original", WorktreeID: "worktree", Terminal: id, Incarnation: id + "-inc"})
			}
			switch mode {
			case "foreign-runtime":
				originals[2].RuntimeID = "other"
			case "foreign-worktree":
				originals[2].WorktreeID = "other"
			case "missing-identity":
				originals[2].Incarnation = ""
			case "empty-identities":
				originals = nil
			}
			calls := 0
			c := New(Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				calls++
				if len(args) != 5 || args[0] != "terminal" || args[1] != "list" || args[3] != "id:worktree" {
					t.Fatal("unexpected effect", args)
				}
				terms := []Terminal{}
				if mode == "exited" || mode == "active-reader" || mode == "changed-incarnation" {
					term := Terminal{Handle: "reader-two", IncarnationID: "reader-two-inc", WorktreeID: "worktree", Status: "exited"}
					if mode == "active-reader" {
						term.Status = "running"
					}
					if mode == "changed-incarnation" {
						term.IncarnationID = "replacement"
					}
					terms = append(terms, term)
				}
				hosts := []string{"local"}
				omitted := []string{}
				if mode == "omitted-local" {
					hosts = []string{"other"}
					omitted = []string{"local"}
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": terms, "totalCount": len(terms), "truncated": mode == "truncated", "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": omitted}}, "_meta": map[string]string{"runtimeId": "original"}})
			}})
			err := c.VerifyTerminalsStopped(context.Background(), originals)
			want := mode == "absent" || mode == "exited"
			wantCalls := 1
			if mode == "foreign-runtime" || mode == "foreign-worktree" || mode == "missing-identity" || mode == "empty-identities" {
				wantCalls = 0
			}
			if (err == nil) != want || calls != wantCalls {
				t.Fatal("batch proof changed identity or repeated inventory", mode, err, calls)
			}
		})
	}
}
