package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestLocalPreparationOriginalStream(t *testing.T) {
	for _, path := range []string{"control", "readiness"} {
		t.Run(path, func(t *testing.T) {
			for _, fault := range []string{"complete", "empty", "screen", "screen-unavailable", "missing-source", "truncated", "missing-truncated", "limited", "missing-limited", "dropped-prefix", "missing-oldest", "wrong-runtime", "wrong-terminal", "invalid-encoding", "partial-line", "changed-binding"} {
				t.Run(fault, func(t *testing.T) {
					l, host, snapshot, control := fixtureFixedControl(t)
					state, err := l.state(snapshot)
					if err != nil {
						t.Fatal(err)
					}
					state.Control = &ControlTerminal{Process: b.ProcessIdentity{Kind: "orca_native_control", RuntimeID: state.Process.RuntimeID, WorktreeID: state.Process.WorktreeID, Terminal: "control", Incarnation: "control-inc"}}
					control.setup = l.controlSetup(snapshot, state)
					snapshot.Implementation, err = l.envelope(snapshot, state)
					if err != nil {
						t.Fatal(err)
					}
					var body any = control.setup
					prefix, handle := codex.RemoteControlReadyPrefix, "control"
					if path == "readiness" {
						prefix, handle, body = codex.RemoteReadyPrefix, state.Process.Terminal, control.ready
					}
					if fault == "changed-binding" {
						if path == "control" {
							changed := control.setup
							changed.Binding = "foreign"
							body = changed
						} else {
							changed := control.ready
							changed.ConfigurationDigest = "foreign"
							body = changed
						}
					}
					raw, _ := json.Marshal(body)
					line := prefix + base64.RawURLEncoding.EncodeToString(raw)
					if fault == "invalid-encoding" {
						line = prefix + "%%%"
					} else if fault == "partial-line" {
						line = line[:len(line)/2]
					}
					terminal := map[string]any{"handle": handle, "tail": []string{line}, "source": "stream", "oldestCursor": "0", "truncated": false, "limited": false}
					runtime := state.Process.RuntimeID
					switch fault {
					case "empty":
						terminal["tail"] = []string{}
					case "screen", "screen-unavailable":
						terminal["source"] = fault
					case "missing-source":
						delete(terminal, "source")
					case "truncated", "limited":
						terminal[fault] = true
					case "missing-truncated":
						delete(terminal, "truncated")
					case "missing-limited":
						delete(terminal, "limited")
					case "dropped-prefix":
						terminal["oldestCursor"] = "1"
					case "missing-oldest":
						delete(terminal, "oldestCursor")
					case "wrong-runtime":
						runtime = "foreign"
					case "wrong-terminal":
						terminal["handle"] = "foreign"
					}
					original, reads := l.client, 0
					l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
						if len(args) > 1 && args[0] == "terminal" && args[1] == "read" {
							reads++
							if strings.Join(args, " ") != "terminal read --terminal "+handle+" --cursor 0 --limit 1000 --json" {
								t.Fatal("read did not explicitly request the original bounded stream", args)
							}
							return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminal": terminal}, "_meta": map[string]string{"runtimeId": runtime}})
						}
						return original.Raw(ctx, timeout, args...)
					}})
					if path == "control" {
						intent := l.controlIntent(snapshot, state)
						receipt, readErr := l.prepareControl(context.Background(), request(snapshot, b.LifecycleCreateResource, b.LifecycleInput{Resource: &intent}), true)
						if (readErr == nil) != (fault == "complete" || fault == "empty") || readErr == nil && receipt.State.ResourcePending != (fault == "empty") {
							t.Fatal("control stream qualification", receipt, readErr)
						}
					} else {
						ready, readErr := l.readLocalCodexReadiness(context.Background(), snapshot)
						if (readErr == nil) != (fault == "complete") || fault == "empty" && !errors.Is(readErr, errRemoteReadinessPending) || readErr == nil && ready.SessionID != control.ready.SessionID {
							t.Fatal("readiness stream qualification", ready, readErr)
						}
					}
					if reads != 1 || control.creates != 0 || control.sends != 0 || host.nativeSends != 0 {
						t.Fatal("read qualification dispatched extra work", reads, control.creates, control.sends, host.nativeSends)
					}
				})
			}
		})
	}
}
