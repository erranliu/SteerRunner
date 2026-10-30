package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// SerializeCLI first inspects Done after enqueuing its ticket. This observation
// is a deterministic queue barrier, not a timer or a replacement lock.
type queuedPromptContext struct {
	context.Context
	once   sync.Once
	queued chan struct{}
}

func (c *queuedPromptContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.queued) })
	return c.Context.Done()
}

// Adapter-level shared mutex regression; real service/SQLite isolation is
// exercised separately by TestSRMultipleRunsIsolation and TestSROrcaCLIWaitRetry.
func TestNativePromptSharedQueueWithStopAndCancellation(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	first, h1, s1, input := promptFixture(t, "paired-worker", "Original\n多行正文")
	stop, h2, s2, _ := promptFixture(t, "paired-worker", "Other run")
	third, h3, s3, _ := promptFixture(t, "", "Local run")
	// Give the three adapter fixtures distinct durable Run and resource identities.
	for i, row := range []struct {
		l *Lifecycle
		h *fakeHost
		s *b.LifecycleSnapshot
	}{{first, h1, &s1}, {stop, h2, &s2}, {third, h3, &s3}} {
		state, err := row.l.state(*row.s)
		if err != nil {
			t.Fatal(err)
		}
		row.h.terminalHandle = []string{"submit-terminal", "stop-terminal", "observe-terminal"}[i]
		row.s.Ref.RunID = strings.Repeat([]string{"a", "b", "c"}[i], 32)
		row.s.Session.ExecutionID = row.s.Ref.RunID
		row.s.Session.WorkflowRunID = row.s.Ref.RunID
		row.s.Ref.External.Terminal = row.h.terminalHandle
		state.Process = row.s.Ref.External
		row.s.Session.SessionID = SessionID(state.Process)
		row.l.setup.Snapshot = *row.s
		row.s.Implementation, err = row.l.envelope(*row.s, state)
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	first.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		return native.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
			if args[0] == "terminal" && args[1] == "send" {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return h1.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
		})
	}})
	submitDone := make(chan error, 1)
	go func() {
		r, e := first.submit(ctx, request(s1, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
		if e == nil && !r.State.InputAccepted {
			t.Error("unaccepted submit")
		}
		submitDone <- e
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stopQueued := make(chan struct{})
	var stopDispatched atomic.Int32
	closed := false
	stop.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		q := &queuedPromptContext{Context: ctx, queued: stopQueued}
		// Only the first invocation announces queueing; subsequent calls use ctx.
		queueCtx := context.Context(ctx)
		if stopDispatched.Load() == 0 {
			queueCtx = q
		}
		return native.SerializeCLI(queueCtx, func(ctx context.Context) ([]byte, error) {
			stopDispatched.Add(1)
			if args[0] == "terminal" && args[1] == "close" {
				closed = true
			}
			if args[0] == "terminal" && args[1] == "list" && closed {
				return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "runtime"}, "result": map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}})
			}
			return h2.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
		})
	}})
	type stopped struct {
		r b.LifecycleReceipt
		e error
	}
	stopDone := make(chan stopped, 1)
	go func() {
		r, e := stop.StopLifecycle(ctx, request(s2, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
		stopDone <- stopped{r, e}
	}()
	select {
	case <-stopQueued:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if stopDispatched.Load() != 0 {
		t.Fatal("stop bypassed held CLI lock")
	}
	thirdCtx, cancelThird := context.WithCancel(ctx)
	defer cancelThird()
	thirdQueued := make(chan struct{})
	var thirdDispatched atomic.Int32
	third.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		return native.SerializeCLI(&queuedPromptContext{Context: ctx, queued: thirdQueued}, func(ctx context.Context) ([]byte, error) {
			thirdDispatched.Add(1)
			return h3.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
		})
	}})
	thirdDone := make(chan error, 1)
	go func() { _, e := third.original(thirdCtx, s3); thirdDone <- e }()
	select {
	case <-thirdQueued:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cancelThird()
	select {
	case e := <-thirdDone:
		if !native.IsCLIQueueError(e) {
			t.Fatal("lost queue cancellation", e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if thirdDispatched.Load() != 0 {
		t.Fatal("canceled observation dispatched")
	}
	close(release)
	select {
	case e := <-submitDone:
		if e != nil {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case v := <-stopDone:
		if v.e != nil || !v.r.State.Stopped {
			t.Fatal("original stop unverified", v.r, v.e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if h1.nativeSends != 1 || thirdDispatched.Load() != 0 || stopDispatched.Load() == 0 {
		t.Fatal("duplicate input or blocked cleanup")
	}
	// Lock was released after all three original operations; no deadline reset.
	if _, e := native.SerializeCLI(ctx, func(context.Context) ([]byte, error) { return nil, nil }); e != nil {
		t.Fatal(e)
	}
}
