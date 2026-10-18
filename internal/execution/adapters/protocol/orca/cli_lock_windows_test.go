//go:build windows

package orca

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestOrcaCLISerializesProcessesWithoutRetryOrDispatchAfterCancellation(t *testing.T) {
	const helper = "SR_ORCA_CLI_LOCK_HELPER"
	if os.Getenv(helper) == "1" {
		_, err := SerializeCLI(context.Background(), func(context.Context) ([]byte, error) {
			fmt.Println("LOCKED")
			_, err := io.Copy(io.Discard, os.Stdin)
			return nil, err
		})
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOrcaCLISerializesProcessesWithoutRetryOrDispatchAfterCancellation$")
	cmd.Env = append(os.Environ(), helper+"=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "LOCKED\n" {
		t.Fatal("child did not own original lock", line, err)
	}
	wait, cancelWait := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelWait()
	calls := 0
	_, err = SerializeCLI(wait, func(context.Context) ([]byte, error) { calls++; return nil, nil })
	if !errors.Is(err, context.DeadlineExceeded) || !IsCLIQueueError(err) || calls != 0 {
		t.Fatal("canceled queued invocation dispatched or retried", calls, err)
	}
	// A killed owner releases the kernel lock without deleting a shared path.
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_, err = SerializeCLI(ctx, func(context.Context) ([]byte, error) { calls++; return []byte("original-result"), nil })
	if err != nil || calls != 1 {
		t.Fatal("abandoned owner blocked the next exact invocation", calls, err)
	}
}
