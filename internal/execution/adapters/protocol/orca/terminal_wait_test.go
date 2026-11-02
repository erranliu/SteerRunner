package orca

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestOrcaTerminalWaitCapability(t *testing.T) {
	for _, scenario := range []string{"satisfied", "busy", "missing", "null", "zero-ms", "wrong-runtime", "bad-type", "duplicate-type", "malformed", "native-rejection", "response-lost", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			wait := map[string]any{"satisfied": true, "blockedReason": []any{"ignored extension"}}
			runtime := "runtime"
			switch scenario {
			case "busy":
				wait["satisfied"] = false
			case "missing":
				delete(wait, "satisfied")
			case "null":
				wait["satisfied"] = nil
			case "wrong-runtime":
				runtime = "foreign"
			case "bad-type":
				wait["satisfied"] = "true"
			}
			packet := map[string]any{"ok": true, "result": map[string]any{"wait": wait}, "_meta": map[string]string{"runtimeId": runtime}}
			if scenario == "native-rejection" {
				packet["ok"], packet["error"] = false, map[string]string{"code": "timeout", "message": "wait expired"}
			}
			raw, _ := json.Marshal(packet)
			calls := 0
			lost := errors.New("original wait response lost")
			client := New(Config{Output: func(_ context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				calls++
				millis := "1000"
				if scenario == "zero-ms" {
					millis = "0"
				}
				if timeout != 2*time.Second || !reflect.DeepEqual(args, []string{"terminal", "wait", "--terminal", "terminal", "--for", "tui-idle", "--timeout-ms", millis, "--json"}) {
					t.Fatal("wait changed original argv or invocation budget", args, timeout)
				}
				switch scenario {
				case "malformed":
					return []byte(`{"ok":true,"result":{"wait":"invalid"},"_meta":{"runtimeId":"runtime"}}`), nil
				case "duplicate-type":
					return []byte(`{"ok":true,"result":{"wait":{"satisfied":"bad","satisfied":true}},"_meta":{"runtimeId":"runtime"}}`), nil
				case "response-lost":
					return nil, lost
				case "cancelled":
					return nil, context.Canceled
				}
				return raw, nil
			}})
			input := TerminalWait{Handle: "terminal", Condition: "tui-idle", TimeoutMillis: 1000}
			if scenario == "zero-ms" {
				input.TimeoutMillis = 0
			}
			result, receipt, err := client.WaitTerminal(context.Background(), 2*time.Second, input)
			badType := scenario == "bad-type" || scenario == "duplicate-type" || scenario == "malformed"
			lostResponse := scenario == "response-lost" || scenario == "cancelled"
			if calls != 1 || (err != nil) != (badType || lostResponse || scenario == "native-rejection") || receipt.Decoded() != (!badType && !lostResponse) || result.CallUncertain != lostResponse {
				t.Fatal("wait promoted a malformed result or retried", calls, result, receipt, err)
			}
			if receipt.Decoded() && receipt.Satisfied != (scenario != "busy" && scenario != "missing" && scenario != "null") {
				t.Fatal("native wait fact changed", receipt)
			}
			if !lostResponse && result.RuntimeID != runtime || scenario == "native-rejection" && result.ErrorCode != "timeout" || scenario == "response-lost" && !errors.Is(err, lost) || scenario == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal("wait lost original native/call facts", result, err)
			}
		})
	}
	t.Run("invalid-zero-invocation", func(t *testing.T) {
		calls := 0
		client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) { calls++; return nil, nil }})
		for _, input := range []TerminalWait{{Condition: "tui-idle"}, {Handle: "terminal"}, {Handle: "terminal", Condition: "tui-idle", TimeoutMillis: -1}} {
			_, receipt, err := client.WaitTerminal(context.Background(), time.Second, input)
			if err == nil || calls != 0 || receipt.Decoded() {
				t.Fatal("invalid wait reached an implicit target", calls, receipt, err)
			}
		}
	})
	t.Run("wire-cannot-inject-decode", func(t *testing.T) {
		var receipt TerminalWaitReceipt
		if err := json.Unmarshal([]byte(`{"satisfied":true,"decoded":true}`), &receipt); err != nil || receipt.Decoded() {
			t.Fatal("wire injected decoding fact", receipt, err)
		}
	})
}
