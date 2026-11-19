package codexworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestLaunchServerFixesWindowsSandboxBeforeStarting(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows launch contract")
	}
	called := false
	sentinel := errors.New("captured launch")
	config := LaunchConfig{Executable: os.Args[0], Workspace: t.TempDir(), ServerDir: t.TempDir(), Sandbox: "danger-full-access", WindowsSandbox: ""}
	_, _, _, err := LaunchServer(config, func(cmd *exec.Cmd) (int, error) {
		called = true
		if !strings.Contains(strings.Join(cmd.Args, " "), `--config sandbox_mode=danger-full-access`) || strings.Contains(strings.Join(cmd.Args, " "), "windows.sandbox=") {
			t.Fatalf("missing fixed mode: %v", cmd.Args)
		}
		if len(cmd.Args) < 3 || cmd.Args[1] != "--no-daemon" || cmd.Args[2] != "--strict-config" {
			t.Fatal("session can route through an unrelated native daemon", cmd.Args)
		}
		return 0, sentinel
	}, func(env []string) []string { return env })
	if !called || !errors.Is(err, sentinel) {
		t.Fatalf("launch not observed: %v", err)
	}
	config.WindowsSandbox = "elevated"
	called = false
	_, _, _, err = LaunchServer(config, func(cmd *exec.Cmd) (int, error) { called = true; return 0, sentinel }, func(env []string) []string { return env })
	if called || err == nil {
		t.Fatal("unsupported mode reached process start")
	}
}

func TestHistoryUsesOriginalServerSQLiteAndSharedAuthentication(t *testing.T) {
	home, server := t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", home)
	sentinel := errors.New("captured history launch")
	for range 2 {
		_, err := OpenHistory(context.Background(), b.Command{Executable: os.Args[0]}, home, home, server, func(cmd *exec.Cmd) (b.StopHandle, error) {
			if !slices.Contains(cmd.Args, fmt.Sprintf("sqlite_home=%q", filepath.Join(server, "sqlite"))) || !slices.Contains(cmd.Env, "CODEX_HOME="+home) {
				t.Fatal("history changed the original SQLite/authentication binding", cmd.Args)
			}
			return nil, sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatal(err)
		}
	}
}

func TestLifecycleServerDirectorySeparatesAttempts(t *testing.T) {
	root := t.TempDir()
	setup := b.LifecycleSetup{RunDirectory: root, AttemptNumber: 1, Snapshot: b.LifecycleSnapshot{Session: b.SessionContext{WorkflowRunID: "original-run", AttemptID: "original-attempt"}}}
	if got := lifecycleServerDirectory(setup); got != filepath.Join(root, "runs", "original-run", "app-server") {
		t.Fatal(got)
	}
	setup.AttemptNumber, setup.Snapshot.Session.AttemptID = 2, "next-attempt"
	if got := lifecycleServerDirectory(setup); got != filepath.Join(root, "runs", "original-run", "attempts", "next-attempt", "app-server") {
		t.Fatal(got)
	}
}

func TestLaunchServerIsolatesSQLiteWithSharedCodexHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	sentinel := errors.New("captured launch")
	var databases []string
	for range 2 {
		server := t.TempDir()
		config := LaunchConfig{Executable: os.Args[0], Workspace: t.TempDir(), ServerDir: server, Sandbox: "danger-full-access"}
		_, _, _, err := LaunchServer(config, func(cmd *exec.Cmd) (int, error) {
			if !slices.Contains(cmd.Env, "CODEX_HOME="+home) {
				t.Fatal("shared authentication/configuration home changed")
			}
			want := fmt.Sprintf("sqlite_home=%q", filepath.Join(server, "sqlite"))
			if !slices.Contains(cmd.Args, want) {
				t.Fatal("missing resource-owned SQLite directory", cmd.Args)
			}
			databases = append(databases, want)
			info, err := os.Stat(filepath.Join(server, "sqlite"))
			if err != nil || !info.IsDir() {
				t.Fatal("SQLite directory unavailable before launch", err)
			}
			return 0, sentinel
		}, func(env []string) []string { return env })
		if !errors.Is(err, sentinel) {
			t.Fatal(err)
		}
	}
	if databases[0] == databases[1] {
		t.Fatal("independent servers share a SQLite runtime")
	}
}
