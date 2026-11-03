package collect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/usage/access"
	"golang.org/x/sys/windows"
)

// Give the parent a hidden real console so an unconfigured child would inherit
// a nonzero console handle. No user-visible console is needed for this test.
func TestUsageCLIQueriesHaveNoConsoleWindow(t *testing.T) {
	console := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
	switch os.Getenv("SR_USAGE_CONSOLE_FIXTURE") {
	case "query":
		handle, _, _ := console.Call()
		if handle != 0 {
			os.Exit(27)
		}
		fmt.Print(`{"ok":true,"_meta":{"runtimeId":"test-runtime"},"result":{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"omittedHostIds":[]}}}`)
		os.Exit(0)
	case "parent":
		handle, _, _ := console.Call()
		if handle == 0 {
			t.Fatal("fixture parent has no console; cannot detect inheritance")
		}
		t.Setenv("SR_USAGE_CONSOLE_FIXTURE", "query")
		p := orca.Profile{Executable: os.Args[0], Args: []string{"-test.run=^TestUsageCLIQueriesHaveNoConsoleWindow$", "--"}}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for range 3 {
			if runtime, err := currentRuntime(ctx, p); err != nil || runtime != "test-runtime" {
				t.Fatal("background status query acquired a console", runtime, err)
			}
		}
		if !safeFolder(ctx, p, access.Lease{FolderID: "original-folder", Runtime: "test-runtime"}) {
			t.Fatal("background terminal census acquired a console")
		}
		cancelled, stop := context.WithCancel(ctx)
		stop()
		if _, err := currentRuntime(cancelled, p); !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled CLI query lost its deadline", err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUsageCLIQueriesHaveNoConsoleWindow$")
	command.Env = append(os.Environ(), "SR_USAGE_CONSOLE_FIXTURE=parent")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_CONSOLE}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("hidden console fixture failed: %v\n%s", err, output)
	}
}
