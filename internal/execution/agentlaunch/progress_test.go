package agentlaunch

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProgressDisabledLaunchDoesNotInheritAnotherRunPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	root := t.TempDir()
	command := ProgressCommand(`if($env:STEERLINE_PROGRESS_FILE) { throw 'foreign progress path' }; [Console]::Write('unset')`, "")
	command += `; [Console]::Write('|' + $env:STEERLINE_PROGRESS_FILE)`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "STEERLINE_PROGRESS_FILE=C:/another-run/progress.bin")
	raw, err := cmd.CombinedOutput()
	if err != nil || string(raw) != "unset|C:/another-run/progress.bin" {
		t.Fatal("disabled launch leaked or failed to restore inherited path", err, string(raw))
	}
	if _, err := os.Stat(filepath.Join(root, ".steerrunner")); !os.IsNotExist(err) {
		t.Fatal("disabled channel prepared a runtime directory", err)
	}
}

func TestResultFileLaunchCombinationsAndExactUnwrap(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	for _, progress := range []bool{false, true} {
		for _, result := range []bool{false, true} {
			root := t.TempDir()
			if progress || result {
				if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
					t.Fatal(err, string(out))
				}
			}
			run := strings.Repeat("a", 32)
			progressRun, resultRun := "", ""
			if progress {
				progressRun = run
			}
			if result {
				resultRun = run
			}
			inner := `[Console]::Write((@{progress=$env:STEERLINE_PROGRESS_FILE;result=$env:STEERLINE_RESULT_FILE}|ConvertTo-Json -Compress))`
			command := FileCommands(inner, progressRun, resultRun)
			if got, ok := UnwrapFileCommands(command, run); !ok || got != inner {
				t.Fatal("new launch lost", got, ok)
			}
			if _, ok := UnwrapFileCommands(command+"; other", run); ok {
				t.Fatal("arbitrary suffix accepted")
			}
			if result {
				if _, ok := UnwrapFileCommands(command, strings.Repeat("b", 32)); ok {
					t.Fatal("foreign Run accepted")
				}
			}
			command += `; [Console]::Write('|' + $env:STEERLINE_PROGRESS_FILE + '|' + $env:STEERLINE_RESULT_FILE)`
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "STEERLINE_PROGRESS_FILE=old-progress", "STEERLINE_RESULT_FILE=old-result")
			raw, err := cmd.CombinedOutput()
			cancel()
			if err != nil {
				t.Fatal(err, string(raw))
			}
			parts := strings.Split(string(raw), "|")
			if len(parts) != 3 || parts[1] != "old-progress" || parts[2] != "old-result" {
				t.Fatal("outer environment not restored", string(raw))
			}
			var env map[string]string
			if err := json.Unmarshal([]byte(parts[0]), &env); err != nil {
				t.Fatal(err, string(raw))
			}
			expectedProgress, expectedResult := "", ""
			if progress {
				expectedProgress = filepath.Join(root, ".steerrunner", "progress", run, "progress.bin")
			}
			if result {
				expectedResult = filepath.Join(root, ".steerrunner", "result", run, "result.bin")
			}
			if env["progress"] != expectedProgress || env["result"] != expectedResult {
				t.Fatal("channel environment mixed", env)
			}
			if !progress && !result {
				if _, err := os.Stat(filepath.Join(root, ".steerrunner")); !os.IsNotExist(err) {
					t.Fatal("disabled channels prepared files", err)
				}
			}
		}
	}
}

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
