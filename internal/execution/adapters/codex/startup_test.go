package codexworker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func assertStartupHeld(t *testing.T, home string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	release, err := acquireStartup(ctx, home)
	if release != nil {
		release()
		t.Fatal("initialization was not serialized")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestCodexStartupLockScopesAndCancellation(t *testing.T) {
	home := t.TempDir()
	release, err := acquireStartup(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	assertStartupHeld(t, home)
	other, err := acquireStartup(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal("unrelated home was blocked", err)
	}
	other()
	release()
	release()
	next, err := acquireStartup(context.Background(), home)
	if err != nil {
		t.Fatal("initialization lock was not released", err)
	}
	next()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := acquireStartup(ctx, filepath.Join(home, "cancelled")); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled initialization acquired a lock", err)
	}
	if _, err := os.Stat(filepath.Join(home, "cancelled")); !os.IsNotExist(err) {
		t.Fatal("cancelled initialization touched disk")
	}
}

func TestCodexStartupLockProcess(t *testing.T) {
	home := os.Getenv("SR_CODEX_STARTUP_LOCK_HELPER")
	if home == "" {
		return
	}
	release, err := acquireStartup(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err = os.WriteFile(filepath.Join(home, "ready"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	<-time.After(10 * time.Second)
}

func TestCodexStartupLockIsCrossProcessAndCrashReleased(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCodexStartupLockProcess$")
	child.Env = append(os.Environ(), "SR_CODEX_STARTUP_LOCK_HELPER="+home)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	for {
		if _, err := os.Stat(filepath.Join(home, "ready")); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("child never acquired initialization lock")
		case <-time.After(10 * time.Millisecond):
		}
	}
	assertStartupHeld(t, home)
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	release, err := acquireStartup(ctx, home)
	if err != nil {
		t.Fatal("crashed process retained initialization lock", err)
	}
	release()
}
