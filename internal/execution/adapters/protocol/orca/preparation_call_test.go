package orca

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOrcaCLIFIFOAndCanceledTicket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue")
	first, err := enterCLI(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if first != nil {
			first()
		}
	}()
	order := make(chan int, 3)
	cancelCtx, cancel := context.WithCancel(context.Background())
	for i := 0; i < 3; i++ {
		ctx := context.Background()
		if i == 1 {
			ctx = cancelCtx
		}
		go func(i int, ctx context.Context) {
			leave, err := enterCLI(ctx, path)
			if err != nil {
				order <- -1
				return
			}
			order <- i
			leave()
		}(i, ctx)
		// Wait for the actual ticket rather than depending on goroutine timing.
		for until := time.Now().Add(time.Second); ; {
			cliFIFO.Lock()
			count := len(cliFIFO.queues[strings.ToLower(filepath.Clean(path))])
			cliFIFO.Unlock()
			if count == i+2 {
				break
			}
			if time.Now().After(until) {
				t.Fatal("ticket was not enqueued")
			}
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if got := <-order; got != -1 {
		t.Fatal("canceled waiter ran", got)
	}
	first()
	first = nil
	for _, want := range []int{0, 2} {
		if got := <-order; got != want {
			t.Fatal("FIFO order", got, want)
		}
	}
}

func TestOrcaPreparationQueueBeyondOldExecutionLimit(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, release, ownerDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := SerializeCLI(ctx, func(context.Context) ([]byte, error) { close(locked); <-release; return nil, nil })
		ownerDone <- err
	}()
	<-locked
	defer func() { close(release); <-ownerDone }()
	calls, checkpoints := 0, 0
	client := New(Config{Output: func(callCtx context.Context, timeout time.Duration, _ ...string) ([]byte, error) {
		calls++
		deadline, ok := callCtx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 15*time.Second || timeout != 15*time.Second {
			t.Error("execution budget did not start after queue")
		}
		return []byte(`{"ok":true,"result":{},"_meta":{"runtimeId":"original"}}`), nil
	}})
	done := make(chan error, 1)
	go func() {
		_, err := client.PreparationCall(ctx, true, func() error { checkpoints++; return nil }, "status", "--json")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatal("query ended within old queue limit", err)
	case <-time.After(16 * time.Second):
	}
	// Release without closing the channel twice in cleanup.
	release <- struct{}{}
	if err := <-done; err != nil || calls != 1 || checkpoints != 1 {
		t.Fatal("original query did not continue once", calls, checkpoints, err)
	}
}

func TestOrcaQueueErrorDoesNotParseEmptyOutput(t *testing.T) {
	queued := &CLIQueueError{Cause: context.DeadlineExceeded}
	client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) { return nil, queued }})
	result, err := client.MetaCall(context.Background(), "status", "--json")
	if err != queued || result.CallUncertain || strings.Contains(err.Error(), "JSON") {
		t.Fatal(result, err)
	}
	real := errors.New("actual transport failed")
	client = New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) { return []byte("{"), real }})
	result, err = client.MetaCall(context.Background(), "status", "--json")
	if !errors.Is(err, real) || !result.CallUncertain || err == real {
		t.Fatal("real malformed output lost original boundary", result, err)
	}
}
