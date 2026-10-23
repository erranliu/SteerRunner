package agentlaunch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProgressLaunchUsesActualExecutionDirectory(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	root := t.TempDir()
	work := filepath.Join(root, "owner's workspace")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", work, "init", "-q").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	command := ProgressCommand(`if(-not $env:STEERLINE_PROGRESS_FILE) { throw 'missing' }; [Console]::Write($env:STEERLINE_PROGRESS_FILE)`, strings.Repeat("a", 32))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	cmd.Dir = work
	raw, err := cmd.CombinedOutput()
	want := filepath.Join(work, ".steerrunner", "progress", strings.Repeat("a", 32), "progress.bin")
	if err != nil || string(raw) != want {
		t.Fatal(err, string(raw), want)
	}
	ignore, err := os.ReadFile(filepath.Join(work, ".git", "info", "exclude"))
	if err != nil || !strings.Contains(string(ignore), "/.steerrunner/progress/") {
		t.Fatal(err, string(ignore))
	}
}
