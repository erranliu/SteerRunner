package orcatransport

import (
	"testing"

	// Wrap the public CLI wire, retaining the same fixture host and command history.
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)
import (
	"context"
	"encoding/json"

	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

func workspaceCalls(l *Lifecycle, intercept func([]string) ([]byte, bool)) *[][]string {
	original := l.client
	calls := new([][]string)
	l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		*calls = append(*calls, append([]string(nil), args...))
		if intercept != nil {
			if raw, override := intercept(args); override {
				return raw, nil
			}
		}
		return original.Raw(ctx, timeout, args...)
	}})
	return calls
}

func workspaceQueryError(message string) []byte {
	raw, _ := json.Marshal(map[string]any{"ok": false, "error": map[string]any{"code": "runtime_error", "message": message}, "_meta": map[string]any{"runtimeId": "runtime"}})
	return raw
}

func assertWorkspaceReadonly(t *testing.T, calls [][]string) {
	t.Helper()
	for _, call := range calls {
		if call[1] != "list" && call[1] != "show" {
			t.Fatal("workspace reconciliation mutated resources", call)
		}
	}
}
func TestWorkspaceCreationUsesConfiguredTimeout(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, f, s := workspaceFixtureOn(t, environment)
			l.setup.AgentLaunchCommand, f.commandMode = "opaque-start", true
			l.setup.CommandTimeout = 2 * time.Minute
			original := l.client
			creations := 0
			l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				want := 40 * time.Second
				if args[0] == "status" || args[0] == "repo" && args[1] == "list" {
					want = 15 * time.Second
				}
				if args[0] == "worktree" && args[1] == "create" {
					for _, arg := range args {
						if arg == "--activate" {
							t.Fatal("background execution changed the active Orca view")
						}
					}
					want = 2 * time.Minute
					creations++
				}
				if timeout != want {
					t.Fatalf("%v timeout = %s, want %s", args[:2], timeout, want)
				}
				return original.Raw(ctx, timeout, args...)
			}})
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil || receipt.Disposition != b.OperationConfirmed || creations != 1 {
				t.Fatal(receipt, err, creations)
			}
		})
	}
}
