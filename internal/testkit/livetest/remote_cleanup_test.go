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
	for _, mode := range []string{"settles", "empty-first", "nonempty-deadline", "foreign-runtime", "foreign-worktree", "incomplete", "wrong-total", "transient-read", "permanent-read", "foreign-error-runtime", "permission-denied", "null-inventory", "foreign-host", "changed-incarnation", "local-other-omitted", "local-owner-omitted", "remote-other-omitted"} {
		t.Run(mode, func(t *testing.T) {
			original := b.ProcessIdentity{RuntimeID: "original-runtime", WorktreeID: "repo::C:/worker", Terminal: "original-terminal", Incarnation: "original-inc"}
			if mode == "local-other-omitted" || mode == "local-owner-omitted" {
				original.ExecutionHost = "local"
			}
			calls := 0
			client := native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				calls++
				if len(args) < 4 || args[0] != "terminal" || args[1] != "list" || args[2] != "--worktree" || args[3] != "id:"+original.WorktreeID {
					t.Fatal("cleanup performed a mutation or queried another scope", args)
				}
				if mode == "permanent-read" || (mode == "transient-read" || mode == "foreign-error-runtime" || mode == "permission-denied") && calls == 1 {
					runtime := original.RuntimeID
					code := "remote_runtime_unavailable"
					if mode == "foreign-error-runtime" {
						runtime = "foreign"
					}
					if mode == "permission-denied" {
						code = "permission_denied"
					}
					return json.Marshal(map[string]any{"ok": false, "error": map[string]string{"code": code, "message": "unavailable"}, "_meta": map[string]string{"runtimeId": runtime}})
				}
				runtime := original.RuntimeID
				if mode == "foreign-runtime" {
					runtime = "foreign"
				}
				terminals := []any{}
				if mode != "local-other-omitted" && mode != "empty-first" && mode != "transient-read" && (mode != "settles" || calls == 1) {
					worktree := original.WorktreeID
					if mode == "foreign-worktree" {
						worktree = "other"
					}
					terminals = []any{map[string]any{"handle": original.Terminal, "worktreeId": worktree, "incarnationId": original.Incarnation, "connected": false}}
				}
				if mode == "null-inventory" {
					terminals = nil
				}
				if mode == "changed-incarnation" {
					terminals[0].(map[string]any)["incarnationId"] = "replacement"
				}
				total := len(terminals)
				if mode == "wrong-total" {
					total++
				}
				result := map[string]any{"terminals": terminals, "totalCount": total, "truncated": mode == "incomplete", "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}
				if mode == "local-other-omitted" || mode == "remote-other-omitted" {
					result["hostScope"] = map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{"paired"}}
				}
				if mode == "local-owner-omitted" {
					result["hostScope"] = map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{"local"}}
				}
				if mode == "foreign-host" {
					result["hostScope"] = map[string]any{"hostIds": []string{"other"}, "omittedHostIds": []string{}}
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
			}})
			timeout := time.Second
			if mode == "nonempty-deadline" || mode == "permanent-read" {
				timeout = 20 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			fact, err := WaitRemoteWorkspaceEmpty(ctx, client, original)
			passing := mode == "settles" || mode == "empty-first" || mode == "transient-read" || mode == "local-other-omitted"
			if (err == nil) != passing || fact.Verified != passing {
				t.Fatal("cleanup accepted nonempty, incomplete or foreign evidence", fact, err)
			}
			if mode == "transient-read" && (calls != 2 || fact.TransientReadFailures != 1 || fact.Observations != 1) {
				t.Fatal("original read was not bounded/rechecked", calls, fact)
			}
			if (mode == "foreign-error-runtime" || mode == "permission-denied" || mode == "null-inventory" || mode == "foreign-host" || mode == "changed-incarnation") && calls != 1 {
				t.Fatal("invalid proof was retried", mode, calls)
			}
			if mode == "settles" && calls != 2 {
				t.Fatal("cleanup did not wait for metadata removal", calls)
			}
		})
	}
}
