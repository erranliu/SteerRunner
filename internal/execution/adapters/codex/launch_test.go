package codexworker

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
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
