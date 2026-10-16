package livetest

import (
	"context"
	"encoding/json"
	"errors"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"testing"
	"time"
)

func TestDiagnosticReadNeverReplaysMutation(t *testing.T) {
	for _, action := range []string{"show", "create", "send", "close"} {
		calls := 0
		_, err := DiagnosticRead(context.Background(), func(context.Context, ...string) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte(`{"ok":false,"error":{"code":"remote_runtime_unavailable","message":"offline"}}`), errors.New("exit 1")
			}
			return []byte(`{"ok":true,"result":{},"_meta":{"runtimeId":"original"}}`), nil
		}, "terminal", action)
		if action == "show" {
			if err != nil || calls != 2 {
				t.Fatal("bounded read did not recover", calls, err)
			}
		} else if err == nil || calls != 1 {
			t.Fatal("mutation was repeated", action, calls, err)
		}
	}
}

func TestAuxiliaryCloseReconcilesExactIdentityAndCompleteInventory(t *testing.T) {
	for _, mode := range []string{"still-live", "already-absent", "foreign-incarnation", "foreign-runtime", "truncated", "missing-completeness", "persistent-failure"} {
		t.Run(mode, func(t *testing.T) {
			closes := 0
			original := b.ProcessIdentity{RuntimeID: "runtime", Terminal: "original", Incarnation: "incarnation", WorktreeID: "workspace"}
			client := orca.New(orca.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				runtime := "runtime"
				var result any
				tm := map[string]any{"handle": "original", "incarnationId": "incarnation", "worktreeId": "workspace", "connected": true, "status": "running"}
				switch args[1] {
				case "show":
					result = map[string]any{"terminal": tm}
				case "close":
					closes++
					if closes == 1 || mode == "persistent-failure" {
						return []byte(`{"ok":false,"error":{"code":"remote_runtime_unavailable","message":"lost"}}`), errors.New("lost response")
					}
					result = map[string]any{"close": map[string]any{"handle": "original", "ptyKilled": true}}
				case "list":
					terminals := []any{tm}
					if mode == "already-absent" {
						terminals = nil
					}
					if mode == "foreign-incarnation" {
						tm["incarnationId"] = "replacement"
					}
					if mode == "foreign-runtime" {
						runtime = "replacement"
					}
					inventory := map[string]any{"terminals": terminals, "totalCount": len(terminals), "truncated": mode == "truncated", "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}
					if mode == "missing-completeness" {
						delete(inventory, "truncated")
					}
					result = inventory
				default:
					t.Fatal("unexpected primitive", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
			}})
			err := CloseAuxiliaryTerminal(context.Background(), client, original)
			want := map[string]int{"still-live": 2, "already-absent": 1, "persistent-failure": 3}[mode]
			if want == 0 {
				want = 1
			}
			if closes != want || (err == nil) != (mode == "still-live" || mode == "already-absent") {
				t.Fatal("unsafe or unresolved auxiliary cleanup", closes, err)
			}
		})
	}
}
