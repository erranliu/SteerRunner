package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestObserverQueueCancellationKeepsOriginalObservationWithoutResend(t *testing.T) {
	l, f, snapshot := fixture(t)
	receipt, _, err := l.Execute(context.Background(), request(snapshot, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &snapshot.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&snapshot, receipt)
	f.writeReply = true
	snapshot, input := prepared(t, l, snapshot, strings.Repeat("c", 32))
	receipt, err = l.submit(context.Background(), request(snapshot, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&snapshot, receipt)
	original := l.client
	queued := 0
	l.client = native.New(native.Config{Output: func(ctx context.Context, _ time.Duration, args ...string) ([]byte, error) {
		if args[0] == "terminal" && args[1] == "show" && queued == 0 {
			queued++
			return nil, &native.CLIQueueError{Cause: context.DeadlineExceeded}
		}
		result, err := original.MetaCall(ctx, args...)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"ok": true, "result": json.RawMessage(result.Payload), "_meta": map[string]any{"runtimeId": result.RuntimeID}})
	}})
	observer := l.observe(snapshot)
	defer observer.Close(context.Background())
	completed := false
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case event := <-observer.Events():
			if event.Fact.Kind == b.EventCompleted {
				completed = true
			}
			if event.Persisted != nil {
				event.Persisted <- nil
			}
			if event.Processed != nil {
				event.Processed <- nil
			}
		case err := <-observer.Done():
			if err != nil || !completed || queued != 1 {
				t.Fatal("original observation failed after a non-dispatched read", err, completed, queued)
			}
			sends, creates := 0, 0
			for _, call := range f.calls {
				if call[0] == "terminal" && call[1] == "send" {
					sends++
				}
				if call[0] == "worktree" && call[1] == "create" {
					creates++
				}
			}
			if sends != 1 || creates != 1 {
				t.Fatal("queue recovery repeated input or replaced the worker", sends, creates)
			}
			return
		case <-timeout.C:
			t.Fatal("original observation did not resume after queue cancellation")
		}
	}
}
