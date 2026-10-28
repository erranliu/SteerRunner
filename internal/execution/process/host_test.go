package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestProcessHostCommandHelper(t *testing.T) {
	if !slices.Contains(os.Args, "-test.run=^TestProcessHostCommandHelper$") {
		t.Skip("subprocess helper")
	}
	if mode := os.Getenv("STEERRUNNER_TEST_COMMAND_DIAGNOSTIC_MODE"); mode != "" {
		fmt.Fprint(os.Stdout, "完整 stdout\n")
		fmt.Fprint(os.Stderr, "完整 stderr\n")
		if mode == "exit" {
			os.Exit(7)
		}
	}
	time.Sleep(200 * time.Millisecond)
	fmt.Println("command completed")
	fmt.Println("routing=" + os.Getenv("ORCA_ENVIRONMENT"))
	fmt.Println("kept=" + os.Getenv("STEERRUNNER_VIEWER_TEST_KEEP"))
}

func TestProcessHostCommandDiagnostics(t *testing.T) {
	for _, mode := range []string{"success", "exit", "timeout", "cancelled", "start-failed", "intent-unavailable"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("STEERRUNNER_TEST_COMMAND_DIAGNOSTIC_MODE", mode)
			path := filepath.Join(t.TempDir(), "call")
			cmd := executionbackend.LaunchCommand{Executable: os.Args[0], Args: []string{"-test.run=^TestProcessHostCommandHelper$"}, Timeout: 5 * time.Second, DiagnosticPath: path}
			ctx := context.Background()
			if mode == "timeout" {
				cmd.Timeout = 100 * time.Millisecond
			}
			if mode == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if mode == "start-failed" {
				cmd.Executable = filepath.Join(t.TempDir(), "absent.exe")
			}
			if mode == "intent-unavailable" {
				if err := os.WriteFile(path, []byte("block directory"), 0600); err != nil {
					t.Fatal(err)
				}
				cmd.DiagnosticPath = filepath.Join(path, "call")
			}
			out, err := New(0).OutputCommand(ctx, cmd)
			if mode == "intent-unavailable" {
				if err == nil || len(out) != 0 || !strings.Contains(err.Error(), "command not invoked") {
					t.Fatal(out, err)
				}
				return
			}
			var got commandDiagnostic
			raw, readErr := os.ReadFile(path + ".result.json")
			if readErr != nil || json.Unmarshal(raw, &got) != nil || got.Finished.Before(got.Started) {
				t.Fatal(string(raw), readErr)
			}
			if _, e := os.Stat(path + ".intent.json"); e != nil {
				t.Fatal(e)
			}
			if mode == "success" || mode == "exit" {
				code := 0
				if mode == "exit" {
					code = 7
				}
				if got.ExitCode == nil || *got.ExitCode != code || !got.Invoked || !strings.Contains(got.Stdout, "完整 stdout") || got.Stderr != "完整 stderr\n" || (err != nil) != (code != 0) {
					t.Fatal(got, err)
				}
			} else if mode == "timeout" {
				if !errors.Is(err, context.DeadlineExceeded) || got.Cancelled == nil || got.ExitCode == nil || got.CommandDeadline == nil || !got.Invoked {
					t.Fatal(got, err)
				}
			} else if err == nil || got.Invoked || got.ExitCode != nil {
				t.Fatal(got, err)
			}
		})
	}
}

func TestProcessHostBlocksAmbientRoutingOnly(t *testing.T) {
	t.Setenv("ORCA_ENVIRONMENT", "foreign-host")
	t.Setenv("STEERRUNNER_VIEWER_TEST_KEEP", "original")
	out, err := New(5*time.Second).OutputCommand(context.Background(), executionbackend.LaunchCommand{Executable: os.Args[0], Args: []string{"-test.run=^TestProcessHostCommandHelper$"}, BlockedEnvironment: []string{"orca_environment"}})
	if err != nil || strings.Contains(string(out), "foreign-host") || !strings.Contains(string(out), "kept=original") || os.Getenv("ORCA_ENVIRONMENT") != "foreign-host" {
		t.Fatalf("routing filtering changed parent/unrelated environment: %s / %v", out, err)
	}
}

func TestProcessHostOperationBudget(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := executionbackend.LaunchCommand{Executable: executable, Args: []string{"-test.run=^TestProcessHostCommandHelper$"}}
	for _, scenario := range []string{"explicit_budget", "default_budget", "parent_cancel"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			cmd := command
			if scenario != "default_budget" {
				cmd.Timeout = 5 * time.Second
			}
			if scenario == "parent_cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
			}
			out, err := New(50*time.Millisecond).OutputCommand(ctx, cmd)
			if scenario == "explicit_budget" {
				if err != nil || !strings.Contains(string(out), "command completed") {
					t.Fatalf("ordinary host timeout truncated operation budget: %s / %v", out, err)
				}
			} else if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("command ignored default or parent deadline: %s / %v", out, err)
			}
		})
	}
}
