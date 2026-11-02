package orca

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func TestOrcaCLICommandConstruction(t *testing.T) {
	for _, scenario := range []string{"local", "remote", "empty", "prefix"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("ORCA_ENVIRONMENT", "ambient-must-not-route")
			var prefix []string
			args := []string{"terminal", "send", "--text", "中文 'quote'\nsecond line", "--json"}
			environment := ""
			want := []string{"terminal", "send", "--text", "中文 'quote'\nsecond line", "--json"}
			switch scenario {
			case "remote":
				environment = "paired host"
				want = []string{"terminal", "send", "--text", "中文 'quote'\nsecond line", "--json", "--environment", "paired host"}
			case "empty":
				args, want = nil, nil
			case "prefix":
				prefix = []string{"prefix", "with space"}
				want = []string{"prefix", "with space", "terminal", "send", "--text", "中文 'quote'\nsecond line", "--json"}
			}
			command := CompileCLICommand("C:/path with space/orca.exe", prefix, environment, args...)
			if command.Executable != "C:/path with space/orca.exe" || !reflect.DeepEqual(command.Args, want) ||
				!reflect.DeepEqual(command.BlockedEnvironment, []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}) {
				t.Fatal("compiled command differs from legacy argv", command)
			}
			if len(args) != 0 {
				args[0] = "caller mutation"
				if command.Args[len(prefix)] != "terminal" {
					t.Fatal("operation args alias compiled command")
				}
			}
			if len(prefix) != 0 {
				command.Args[0] = "command mutation"
				if prefix[0] != "prefix" {
					t.Fatal("compiled command aliases profile args")
				}
			}
			command.BlockedEnvironment[0] = "mutated"
			if CompileCLICommand("orca", nil, "").BlockedEnvironment[0] != "ORCA_ENVIRONMENT" {
				t.Fatal("one command changed another command's routing protection")
			}
		})
	}
}

func TestOrcaCLIResultDecoding(t *testing.T) {
	for _, tc := range []struct {
		name, raw, runtime, payload, code, message string
		bad                                        bool
	}{
		{"success", `{"ok":true,"result":{"handle":"original"},"_meta":{"runtimeId":"runtime"}}`, "runtime", `{"handle":"original"}`, "", "", false},
		{"bom", "\xef\xbb\xbf" + `{"ok":true,"result":{},"_meta":{"runtimeId":"runtime"}}`, "runtime", `{}`, "", "", false},
		{"rejected", `{"ok":false,"result":{"partial":true},"error":{"code":"denied","message":"native rejection"},"_meta":{"runtimeId":"runtime"}}`, "runtime", `{"partial":true}`, "denied", "native rejection", true},
		{"rejected-no-error", `{"ok":false}`, "", "", "", "", true},
		{"malformed", `{`, "", "", "", "", true},
		{"empty", ``, "", "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, uncertain := range []bool{false, true} {
				result, err := DecodeCLIResult([]byte(tc.raw), uncertain)
				if (err != nil) != tc.bad || result.RuntimeID != tc.runtime || string(result.Payload) != tc.payload || result.ErrorCode != tc.code || result.ErrorMessage != tc.message || result.CallUncertain != uncertain {
					t.Fatal("decoding changed native facts or dispatch uncertainty", uncertain, result, err)
				}
			}
		})
	}
	t.Run("invocation-stage", func(t *testing.T) {
		// A real exited test process supplies the same typed exit error used by
		// the executor. It fails argument parsing before executing any tests.
		exited := exec.Command(os.Args[0], "-not-a-test-flag").Run()
		var exit *exec.ExitError
		if !errors.As(exited, &exit) || !exit.Exited() {
			t.Fatal("no exited CLI fixture", exited)
		}
		for _, original := range []error{nil, exited, errors.New("transport outcome unknown"), &CLIQueueError{Cause: context.Canceled}} {
			calls := 0
			client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
				calls++
				if IsCLIQueueError(original) {
					return nil, original
				}
				return []byte(`{"ok":true,"result":{},"_meta":{"runtimeId":"runtime"}}`), original
			}})
			result, err := client.MetaCall(context.Background(), "terminal", "show", "--json")
			if (err != nil) != (original != nil) || original != nil && !errors.Is(err, original) || calls != 1 ||
				result.CallUncertain != (original != nil && original != exited && !IsCLIQueueError(original)) || IsCLIQueueError(original) && err != original {
				t.Fatal("caller invocation stage changed or retried", original, result, err, calls)
			}
		}
	})
}
