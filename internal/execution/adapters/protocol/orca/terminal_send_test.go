package orca

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestOrcaTerminalSendCapability(t *testing.T) {
	identity := TerminalIdentity{RuntimeID: "runtime", Handle: "terminal", IncarnationID: "incarnation"}
	for _, scenario := range []string{"default", "wait", "rejected", "accepted-null", "accepted-missing", "runtime", "handle", "incarnation", "request", "malformed", "partial", "transport-loss", "native-error", "optional-shape", "duplicate-extension"} {
		t.Run(scenario, func(t *testing.T) {
			prompt := map[string]any{"requestId": "request", "processIncarnation": "incarnation", "provider": "codex", "observation": "supported", "stages": []string{"input_accepted", "turn_started"}}
			send := map[string]any{"handle": "terminal", "accepted": true, "prompt": prompt}
			runtime, nativeCode := "runtime", ""
			switch scenario {
			case "rejected":
				send["accepted"] = false
			case "accepted-null":
				send["accepted"] = nil
			case "accepted-missing":
				delete(send, "accepted")
			case "runtime":
				runtime = "foreign"
			case "handle":
				send["handle"] = "foreign"
			case "incarnation":
				prompt["processIncarnation"] = "foreign"
			case "request":
				prompt["requestId"] = "foreign"
			case "native-error":
				nativeCode = "unavailable"
			case "optional-shape":
				prompt["stages"], prompt["provider"], prompt["observation"] = 42, []any{}, map[string]any{}
			}
			nativeResult := map[string]any{"send": send}
			if scenario == "malformed" {
				nativeResult["send"] = "invalid"
			}
			packet := map[string]any{"ok": nativeCode == "", "result": nativeResult, "_meta": map[string]string{"runtimeId": runtime}}
			if nativeCode != "" {
				packet["error"] = map[string]string{"code": nativeCode, "message": "native rejected request"}
			}
			payload, _ := json.Marshal(packet)
			originalError := errors.New("response lost after invocation")
			calls := 0
			client := New(Config{CommandTimeout: "31s", Output: func(_ context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				calls++
				want := []string{"terminal", "send", "--terminal", "terminal", "--text", "中文 'quoted'\r\rnext", "--enter", "--retry-request", "request", "--json"}
				wantTimeout := 31 * time.Second
				if scenario == "wait" {
					want = append(want, "--wait-submit", "20")
					wantTimeout = 50 * time.Second
				}
				if !reflect.DeepEqual(args, want) || timeout != wantTimeout {
					t.Fatal("send changed original argv/timeout", args, timeout)
				}
				if scenario == "transport-loss" {
					return nil, originalError
				}
				if scenario == "partial" {
					return []byte(`{"ok":true,"result":{"send":{"handle":"terminal","accepted":true,"prompt":{"requestId":"request","processIncarnation":"incarnation"},"accepted":"bad-type"}},"_meta":{"runtimeId":"runtime"}}`), nil
				}
				if scenario == "duplicate-extension" {
					return []byte(`{"ok":true,"result":{"send":{"handle":"terminal","accepted":true,"prompt":{"requestId":"request","processIncarnation":"incarnation","stages":42,"stages":["input_accepted","turn_started"],"provider":"codex","observation":"supported"}}},"_meta":{"runtimeId":"runtime"}}`), nil
				}
				return payload, nil
			}})
			input := TerminalSend{Handle: "terminal", Text: "中文 'quoted'\r\rnext", RequestID: "request"}
			var result Result
			var receipt TerminalSendReceipt
			var err error
			if scenario == "wait" {
				input.WaitSubmitSeconds = 20
				result, receipt, err = client.SendTerminalWithTimeout(context.Background(), 50*time.Second, input)
			} else {
				result, receipt, err = client.SendTerminal(context.Background(), input)
			}
			wantMatch := scenario == "default" || scenario == "wait" || scenario == "rejected" || scenario == "accepted-null" || scenario == "accepted-missing" || scenario == "native-error" || scenario == "optional-shape" || scenario == "duplicate-extension"
			wantError := scenario == "malformed" || scenario == "partial" || scenario == "transport-loss" || scenario == "native-error"
			if calls != 1 || receipt.Matches(identity, "request") != wantMatch || (err != nil) != wantError || result.CallUncertain != (scenario == "transport-loss") || result.ErrorCode != nativeCode {
				t.Fatal("send lost facts, accepted a malformed identity, or retried", calls, receipt, result, err)
			}
			if scenario == "transport-loss" && !errors.Is(err, originalError) {
				t.Fatal("original invocation error lost", err)
			}
			if scenario == "optional-shape" || scenario == "duplicate-extension" {
				if receipt.Prompt.Decoded() {
					t.Fatal("malformed optional fields claimed full decoding", receipt)
				}
			} else if wantMatch {
				wantAccepted := scenario != "accepted-null" && scenario != "accepted-missing"
				if (receipt.Accepted != nil) != wantAccepted || wantAccepted && *receipt.Accepted != (scenario != "rejected") ||
					!reflect.DeepEqual(receipt.Prompt.Stages, []string{"input_accepted", "turn_started"}) || receipt.Prompt.Provider != "codex" || receipt.Prompt.Observation != "supported" || !receipt.Prompt.Decoded() {
					t.Fatal("native acceptance/stages were simplified or promoted", receipt)
				}
			}
		})
	}
	t.Run("wire-cannot-inject-parse-proof", func(t *testing.T) {
		var receipt TerminalSendReceipt
		raw := []byte(`{"RuntimeID":"runtime","handle":"terminal","accepted":true,"decoded":true,"Decoded":true,"prompt":{"requestId":"request","processIncarnation":"incarnation"}}`)
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Matches(identity, "request") || receipt.RuntimeID != "" || receipt.decoded {
			t.Fatal("wire injected local envelope/parse authority", receipt)
		}
	})
	t.Run("missing-input-zero-invocation", func(t *testing.T) {
		calls := 0
		client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
			calls++
			return nil, nil
		}})
		for _, send := range []TerminalSend{{Handle: "terminal"}, {RequestID: "request"}, {}} {
			for _, explicit := range []bool{false, true} {
				var result Result
				var receipt TerminalSendReceipt
				var err error
				if explicit {
					result, receipt, err = client.SendTerminalWithTimeout(context.Background(), time.Second, send)
				} else {
					result, receipt, err = client.SendTerminal(context.Background(), send)
				}
				if err == nil || calls != 0 || receipt.Matches(identity, "request") || result.CallUncertain {
					t.Fatal("incomplete send identity reached a CLI or gained authority", calls, result, receipt, err)
				}
			}
		}
	})
}
