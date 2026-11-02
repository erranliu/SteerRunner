package orca

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestOrcaResourceQueries(t *testing.T) {
	t.Run("repositories", func(t *testing.T) {
		for _, scenario := range []string{"complete", "truncated", "empty", "missing", "bad-field", "malformed", "ignored-extension"} {
			t.Run(scenario, func(t *testing.T) {
				value := map[string]any{"repos": []any{map[string]any{"id": "repo", "path": "D:/project with space", "kind": "git"}, map[string]any{"id": "folder", "kind": "folder", "path": "/other"}}, "truncated": scenario == "truncated"}
				switch scenario {
				case "empty":
					value["repos"] = []any{}
				case "missing":
					delete(value, "repos")
				case "bad-field":
					value["repos"] = []any{map[string]any{"id": 42}}
				case "ignored-extension":
					value["totalCount"] = "not required by the existing reader"
					value["repos"].([]any)[0].(map[string]any)["displayName"] = []any{}
				}
				payload, _ := json.Marshal(value)
				if scenario == "malformed" {
					payload = []byte(`"invalid inventory"`)
				}
				calls := 0
				client := New(Config{CommandTimeout: "7s", Output: func(_ context.Context, timeout time.Duration, args ...string) ([]byte, error) {
					calls++
					if timeout != 7*time.Second || !reflect.DeepEqual(args, []string{"repo", "list", "--json"}) {
						t.Fatal("repository query changed original parameters", timeout, args)
					}
					return json.Marshal(map[string]any{"ok": true, "result": json.RawMessage(payload), "_meta": map[string]string{"runtimeId": "runtime"}})
				}})
				result, err := client.MetaCall(context.Background(), RepositoryListArgs()...)
				inventory := DecodeRepositoryInventory(result)
				if err != nil || calls != 1 || result.RuntimeID != "runtime" || inventory.Decoded() != (scenario != "bad-field" && scenario != "malformed") || inventory.Truncated != (scenario == "truncated") {
					t.Fatal("repository native facts changed", result, inventory, err, calls)
				}
				if scenario == "complete" || scenario == "ignored-extension" || scenario == "truncated" {
					if len(inventory.Repos) != 2 || inventory.Repos[0] != (Repository{ID: "repo", Path: "D:/project with space", Kind: "git"}) || inventory.Repos[1].Kind != "folder" {
						t.Fatal("repository policy leaked into native inventory", inventory)
					}
				}
			})
		}
		args := RepositoryListArgs()
		args[0] = "mutated"
		if RepositoryListArgs()[0] != "repo" {
			t.Fatal("operation arguments alias another caller")
		}
	})
	t.Run("worktree", func(t *testing.T) {
		for _, scenario := range []string{"complete", "wrong-runtime", "malformed", "missing", "lost", "native-error"} {
			t.Run(scenario, func(t *testing.T) {
				want := Worktree{ID: "repo::C:/owned", InstanceID: "incarnation", RepoID: "repo", Path: "C:/owned", Head: "head", Branch: "branch", BaseRef: "base", DisplayName: "name"}
				calls := 0
				lost := errors.New("worktree show response lost")
				client := New(Config{CommandTimeout: "9s", Output: func(_ context.Context, timeout time.Duration, args ...string) ([]byte, error) {
					calls++
					if timeout != 9*time.Second || !reflect.DeepEqual(args, []string{"worktree", "show", "--worktree", "id:repo::C:/owned", "--json"}) {
						t.Fatal("worktree query changed original parameters", timeout, args)
					}
					if scenario == "lost" {
						return nil, lost
					}
					runtime := "runtime"
					if scenario == "wrong-runtime" {
						runtime = "foreign"
					}
					result := map[string]any{"worktree": want}
					if scenario == "malformed" {
						result["worktree"] = "invalid"
					} else if scenario == "missing" {
						delete(result, "worktree")
					}
					packet := map[string]any{"ok": scenario != "native-error", "result": result, "_meta": map[string]string{"runtimeId": runtime}}
					if scenario == "native-error" {
						packet["error"] = map[string]string{"code": "denied", "message": "native rejection"}
					}
					return json.Marshal(packet)
				}})
				result, query, err := client.ShowWorktree(context.Background(), "id:repo::C:/owned")
				if calls != 1 || (err != nil) != (scenario == "lost" || scenario == "native-error") || query.Decoded() != (scenario != "malformed" && scenario != "lost") || result.CallUncertain != (scenario == "lost") {
					t.Fatal("worktree query changed call/parsing facts", calls, result, query, err)
				}
				if query.Decoded() && scenario != "missing" && query.Worktree != want || scenario == "missing" && query.Worktree != (Worktree{}) || scenario == "wrong-runtime" && result.RuntimeID != "foreign" || scenario == "lost" && !errors.Is(err, lost) || scenario == "native-error" && result.ErrorCode != "denied" {
					t.Fatal("worktree query selected a resource or lost native facts", result, query, err)
				}
			})
		}
	})
	t.Run("implicit-selector-zero-call", func(t *testing.T) {
		calls := 0
		client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) { calls++; return nil, nil }})
		if _, _, err := client.ShowWorktree(context.Background(), ""); err == nil || calls != 0 {
			t.Fatal("implicit worktree lookup invoked", err, calls)
		}
	})
	t.Run("wire-cannot-inject-decoded", func(t *testing.T) {
		var inventory RepositoryInventory
		var query WorktreeQuery
		_ = json.Unmarshal([]byte(`{"repos":[],"decoded":true}`), &inventory)
		_ = json.Unmarshal([]byte(`{"worktree":{},"decoded":true}`), &query)
		if inventory.Decoded() || query.Decoded() {
			t.Fatal("wire injected parser authority")
		}
	})
}
