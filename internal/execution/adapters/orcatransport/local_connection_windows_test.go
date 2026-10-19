package orcatransport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Unary access qualifies local observation even when subscriptions are absent.
// A failed native query still blocks before business resource creation.
func TestLocalIPCQueryCapabilityBeforeCreation(t *testing.T) {
	var rejectQuery atomic.Bool
	var subscriptions atomic.Int32
	endpoint := fmt.Sprintf(`\\.\pipe\sr-orca-adapter-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var req struct{ ID, Method, AuthToken string }
					if json.Unmarshal(scanner.Bytes(), &req) != nil || req.AuthToken != "test-token" {
						return
					}
					reply := map[string]any{"id": req.ID, "ok": true, "result": map[string]any{}, "_meta": map[string]string{"runtimeId": "runtime"}}
					if req.Method == "session.tabs.subscribe" {
						subscriptions.Add(1)
						reply["ok"] = false
						delete(reply, "result")
						reply["error"] = map[string]string{"code": "method_not_supported", "message": "requires a streaming transport"}
					}
					if req.Method == "session.tabs.list" && rejectQuery.Load() {
						reply["ok"] = false
						delete(reply, "result")
						reply["error"] = map[string]string{"code": "unavailable", "message": "query unavailable"}
					}
					raw, _ := json.Marshal(reply)
					_, _ = conn.Write(append(raw, '\n'))
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	raw, _ := json.Marshal(map[string]any{"runtimeId": "runtime", "authToken": "test-token", "transports": []any{map[string]string{"kind": "named-pipe", "endpoint": endpoint}}})
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	l, host, s := fixture(t)
	l.nativeAccess = true
	// Only named-pipe observation is native here. Keep the fixture CLI on its
	// fake host, outside the shared queue used by real Orca invocations.
	l.client = native.New(native.Config{CommandTimeout: "40s", Target: l.profile.Target(), Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: l.profile.Executable, Args: args, WorkingDirectory: l.setup.RepositoryPath, Timeout: timeout})
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := l.verifyLocalObservation(ctx, "runtime"); err != nil || subscriptions.Load() != 0 {
		t.Fatal("local observation required a subscription", err, subscriptions.Load())
	}
	rejectQuery.Store(true)
	receipt, _, err := l.Execute(ctx, request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	var rpc *native.StateRPCError
	if !errors.As(err, &rpc) || rpc.Code != "unavailable" || receipt.Disposition != b.OperationRejected {
		t.Fatal(receipt, err)
	}
	if host.workspace != nil || host.nativeSends != 0 || host.launched {
		t.Fatal("unavailable IPC created business resources")
	}
	for _, call := range host.calls {
		if len(call) > 1 && call[1] == "create" {
			t.Fatal("creation crossed unsupported gate", call)
		}
	}
}
