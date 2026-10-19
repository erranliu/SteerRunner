//go:build windows

package orca

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	// Keep the named object alive so the next acquisition exercises abandoned
	// ownership rather than silently creating a new, unlocked object.
	key := filepath.Join(os.Getenv("LOCALAPPDATA"), "SteerRunner", "orca-cli.lock")
	name, _ := windows.UTF16PtrFromString(cliMutexName(key))
	ref, refErr := windows.CreateMutex(nil, false, name)
	if refErr != nil && !errors.Is(refErr, windows.ERROR_ALREADY_EXISTS) {
		t.Fatal(refErr)
	}
	defer windows.CloseHandle(ref)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_, err = SerializeCLI(ctx, func(context.Context) ([]byte, error) { calls++; return []byte("original-result"), nil })
	if err != nil || calls != 1 {
		t.Fatal("abandoned owner blocked the next exact invocation", calls, err)
	}
}

func TestOrcaCLIPreparationSharesMutexAndRecoversKilledOwner(t *testing.T) {
	root := t.TempDir()
	key := filepath.Join(root, "orca-cli.lock")
	helper, err := filepath.Abs("../../../../../scripts/orca-cli-lock.ps1")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "hold.ps1")
	body := `param([string]$Helper,[string]$Key,[string]$Expected)
. $Helper
$name=Get-SRCLIMutexName $Key
if($name -cne $Expected){throw 'Cross-language mutex name differs'}
$mutex=[Threading.Mutex]::new($false,$name)
try{$null=$mutex.WaitOne();[Console]::WriteLine('LOCKED');$null=[Console]::ReadLine()}finally{$mutex.ReleaseMutex();$mutex.Dispose()}
`
	if err = os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-File", script, "-Helper", helper, "-Key", key, "-Expected", cliMutexName(key))
	input, _ := cmd.StdinPipe()
	output, _ := cmd.StdoutPipe()
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "LOCKED" {
		t.Fatal("preparation did not acquire the same mutex", line, err)
	}
	wait, stopWait := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stopWait()
	if release, err := lockCLIFile(wait, key); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatal("preparation did not exclude Runner CLI", err)
	}
	name, _ := windows.UTF16PtrFromString(cliMutexName(key))
	ref, refErr := windows.CreateMutex(nil, false, name)
	if refErr != nil && !errors.Is(refErr, windows.ERROR_ALREADY_EXISTS) {
		t.Fatal(refErr)
	}
	defer windows.CloseHandle(ref)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	release, err := lockCLIFile(ctx, key)
	if err != nil {
		t.Fatal("killed preparation retained ownership", err)
	}
	release()
}
