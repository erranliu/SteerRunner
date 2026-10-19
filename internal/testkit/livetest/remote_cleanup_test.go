package livetest

import (
	"context"
	"encoding/json"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"testing"
	"time"
)

func TestRemoteWorkspaceCleanupRequiresAuthoritativeEmptyInventory(t *testing.T) {
	for _, mode := range []string{"settles", "empty-first", "nonempty-deadline", "foreign-runtime", "foreign-worktree", "incomplete", "wrong-total", "local-other-host-omitted", "local-owner-omitted", "local-owner-missing"} {
		t.Run(mode, func(t *testing.T) {
			original := b.ProcessIdentity{RuntimeID: "original-runtime", WorktreeID: "repo::C:/worker", Terminal: "original-terminal", Incarnation: "original-inc"}
			if len(mode) > 6 && mode[:6] == "local-" {
				original.ExecutionHost = "local"
			}
			calls := 0
			client := native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				calls++
				if len(args) < 4 || args[0] != "terminal" || args[1] != "list" || args[2] != "--worktree" || args[3] != "id:"+original.WorktreeID {
					t.Fatal("cleanup performed a mutation or queried another scope", args)
				}
				runtime := original.RuntimeID
				if mode == "foreign-runtime" {
					runtime = "foreign"
				}
				terminals := []any{}
				if mode != "empty-first" && (mode != "settles" || calls == 1) {
					worktree := original.WorktreeID
					if mode == "foreign-worktree" {
						worktree = "other"
					}
					terminals = []any{map[string]any{"handle": original.Terminal, "worktreeId": worktree, "incarnationId": original.Incarnation, "connected": false}}
				}
				total := len(terminals)
				if mode == "wrong-total" {
					total++
				}
				result := map[string]any{"terminals": terminals, "totalCount": total, "truncated": mode == "incomplete", "hostScope": map[string]any{"hostIds": []string{"original"}, "omittedHostIds": []string{}}}
				if original.ExecutionHost == "local" {
					result["terminals"], result["totalCount"] = []any{}, 0
					ids, omitted := []string{"local"}, []string{"runtime:unrelated"}
					if mode == "local-owner-omitted" {
						omitted = []string{"local"}
					}
					if mode == "local-owner-missing" {
						ids = []string{"remote"}
					}
					result["hostScope"] = map[string]any{"hostIds": ids, "omittedHostIds": omitted}
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
			}})
			timeout := time.Second
			if mode == "nonempty-deadline" {
				timeout = 20 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			fact, err := WaitRemoteWorkspaceEmpty(ctx, client, original)
			passing := mode == "settles" || mode == "empty-first" || mode == "local-other-host-omitted"
			if (err == nil) != passing || fact.Verified != passing {
				t.Fatal("cleanup accepted nonempty, incomplete or foreign evidence", fact, err)
			}
			if mode == "settles" && calls != 2 {
				t.Fatal("cleanup did not wait for metadata removal", calls)
			}
		})
	}
}
