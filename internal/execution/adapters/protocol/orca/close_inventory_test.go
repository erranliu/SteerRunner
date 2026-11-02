package orca

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestOrcaCloseInventoryCapability(t *testing.T) {
	for _, scenario := range []string{"killed", "accepted", "runtime", "handle", "bad-close", "native-error", "lost", "queue", "missing-identity"} {
		t.Run("close-"+scenario, func(t *testing.T) {
			calls, invoked := 0, 0
			lost := errors.New("close response lost")
			client := New(Config{CommandTimeout: "4s", Output: func(_ context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				calls++
				if timeout != 4*time.Second || !reflect.DeepEqual(args, []string{"terminal", "close", "--terminal", "original", "--json"}) {
					t.Fatal("close wire changed", timeout, args)
				}
				if scenario == "queue" {
					return nil, &CLIQueueError{Cause: context.Canceled}
				}
				invoked++
				if scenario == "lost" {
					return nil, lost
				}
				runtime, handle := "runtime", "original"
				if scenario == "runtime" {
					runtime = "foreign"
				}
				if scenario == "handle" {
					handle = "foreign"
				}
				var close any = TerminalClosure{Handle: handle, PtyKilled: scenario != "accepted"}
				if scenario == "bad-close" {
					close = true
				}
				packet := map[string]any{"ok": scenario != "native-error", "result": map[string]any{"close": close}, "_meta": map[string]string{"runtimeId": runtime}}
				if scenario == "native-error" {
					packet["error"] = map[string]string{"code": "denied", "message": "native rejection"}
				}
				return json.Marshal(packet)
			}})
			handle := "original"
			if scenario == "missing-identity" {
				handle = ""
			}
			result, closed, err := client.CloseTerminal(context.Background(), handle, "runtime")
			if (err == nil) != (scenario == "killed" || scenario == "accepted") || calls > 1 || invoked > 1 || scenario == "missing-identity" && calls != 0 {
				t.Fatal("single close semantics changed", calls, invoked, result, closed, err)
			}
			if scenario == "lost" && (!result.CallUncertain || !errors.Is(err, lost) || invoked != 1) || scenario == "queue" && (result.CallUncertain || !IsCLIQueueError(err) || invoked != 0) || scenario == "native-error" && result.ErrorCode != "denied" || scenario == "killed" && !closed.PtyKilled || scenario == "accepted" && closed.PtyKilled {
				t.Fatal("dispatch/native facts lost", result, closed, err)
			}
		})
	}
	for _, scenario := range []string{"empty", "active", "truncated", "missing-count", "missing-host", "omitted-local", "duplicate-host", "duplicate-terminal", "wrong-worktree", "wrong-runtime"} {
		t.Run("inventory-"+scenario, func(t *testing.T) {
			calls := 0
			client := New(Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				calls++
				if !reflect.DeepEqual(args, []string{"terminal", "list", "--worktree", "id:original-worktree", "--json"}) {
					t.Fatal("inventory selector changed", args)
				}
				terms := []Terminal{}
				if scenario == "active" || scenario == "duplicate-terminal" || scenario == "wrong-worktree" {
					term := Terminal{Handle: "terminal", IncarnationID: "incarnation", WorktreeID: "original-worktree", Status: "running"}
					if scenario == "wrong-worktree" {
						term.WorktreeID = "foreign"
					}
					terms = append(terms, term)
					if scenario == "duplicate-terminal" {
						terms = append(terms, term)
					}
				}
				hosts := []string{"local"}
				if scenario == "omitted-local" {
					hosts = []string{"other"}
				} else if scenario == "duplicate-host" {
					hosts = append(hosts, "local")
				}
				value := map[string]any{"terminals": terms, "totalCount": len(terms), "truncated": scenario == "truncated", "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": []string{}}}
				if scenario == "missing-count" {
					delete(value, "totalCount")
				} else if scenario == "missing-host" {
					delete(value, "hostScope")
				}
				runtime := "runtime"
				if scenario == "wrong-runtime" {
					runtime = "foreign"
				}
				return json.Marshal(map[string]any{"ok": true, "result": value, "_meta": map[string]string{"runtimeId": runtime}})
			}})
			inventory, err := client.CompleteWorktreeInventory(context.Background(), WorktreeInventoryScope{RuntimeID: "runtime", WorktreeID: "original-worktree"})
			if calls != 1 || (err == nil) != (scenario == "empty" || scenario == "active") || scenario == "active" && inventory["terminal"].Status != "running" {
				t.Fatal("inventory integrity or raw active fact changed", calls, inventory, err)
			}
		})
	}
	for _, field := range []string{"totalCount", "hostScope"} {
		t.Run("basic-scope-"+field, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]any{"terminals": []Terminal{{Handle: "original"}}, "truncated": false, field: "invalid extension type"})
			inventory := DecodeTerminalInventory(Result{Payload: payload})
			if !inventory.Basic().Decoded() || inventory.Decoded() || len(inventory.Basic().Terminals) != 1 {
				t.Fatal("consumer field scopes collapsed", inventory)
			}
		})
	}
	for _, truncated := range []string{"true", "false"} {
		t.Run("basic-duplicate-null-"+truncated, func(t *testing.T) {
			inventory := DecodeTerminalInventory(Result{Payload: []byte(`{"terminals":[{"handle":"original"}],"truncated":` + truncated + `,"truncated":null}`)})
			basic := inventory.Basic()
			if !basic.Decoded() || basic.Truncated != (truncated == "true") || len(basic.Terminals) != 1 {
				t.Fatal("basic bool scope lost its own duplicate/null semantics", basic)
			}
		})
	}
	t.Run("missing-scope-zero-call", func(t *testing.T) {
		calls := 0
		client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) { calls++; return nil, nil }})
		if _, _, err := client.CloseTerminal(context.Background(), "original", ""); err == nil {
			t.Fatal("close without runtime admitted")
		}
		for _, scope := range []WorktreeInventoryScope{{RuntimeID: "runtime"}, {WorktreeID: "worktree"}} {
			if _, err := client.CompleteWorktreeInventory(context.Background(), scope); err == nil || calls != 0 {
				t.Fatal("incomplete scope invoked", scope, calls, err)
			}
		}
	})
}
