package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Exercise the existing history, workspace and stop entry points, not just a reader.
func TestFixedControlRetainedStream(t *testing.T) {
	for _, window := range []string{"over-limit", "three-pages", "evicted-prefix"} {
		t.Run(window, func(t *testing.T) {
			for _, mode := range []string{"success", "lost-send", "wrong-response", "wrong-input"} {
				t.Run(mode, func(t *testing.T) {
					l, host, snapshot, control := fixtureFixedControl(t)
					snapshot = prepareFixedControl(t, l, snapshot, control)
					snapshot, input := prepared(t, l, snapshot, strings.Repeat("c", 32))
					receipt, err := l.submit(context.Background(), request(snapshot, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
					if err != nil {
						t.Fatal(err)
					}
					bind(&snapshot, receipt)
					state, _ := l.state(snapshot)
					rows := 1001
					if window == "three-pages" {
						rows = 2505
					}
					if window == "evicted-prefix" {
						control.streamOldest = 5000
					}
					control.stream = make([]string, rows)
					for i := range control.stream {
						control.stream[i] = "previous control command echo"
					}
					// A well-formed old response cannot satisfy the new original request.
					stale := codex.RemoteControlResponse{ID: strings.Repeat("a", 32), Binding: control.setup.Binding, RequestDigest: strings.Repeat("b", 64)}
					raw, _ := json.Marshal(stale)
					control.stream[0] = codex.RemoteControlPrefix + base64.RawURLEncoding.EncodeToString(raw)
					control.readCursors = nil
					control.mode = mode
					value, err := l.recordedTurn(context.Background(), &state, &state.Turns[0])
					if mode == "lost-send" {
						if err == nil {
							t.Fatal("lost send receipt was treated as confirmed")
						}
						// Recreate the lifecycle: the original checkpoint/response must recover.
						l = &Lifecycle{setup: l.setup, profile: l.profile, client: l.client, clock: l.clock}
						value, err = l.recordedTurn(context.Background(), &state, &state.Turns[0])
					}
					if mode == "wrong-response" || mode == "wrong-input" {
						if err == nil || value != nil || control.sends != 1 || host.nativeSends != 1 {
							t.Fatal("foreign response accepted or extra input sent", value, err, control.sends, host.nativeSends)
						}
						return
					}
					if err != nil || value == nil || !value.Completed || value.FinalText != "exact native final" || control.sends != 1 {
						t.Fatal("original history unavailable after accumulated output", value, err, control.sends)
					}
					if _, err = l.recordedTurn(context.Background(), &state, &state.Turns[0]); err != nil || control.sends != 2 || control.requestIDs[0] == control.requestIDs[1] {
						t.Fatal("new query reused stale receipt", err, control.requestIDs)
					}
					fact, err := l.InspectLifecycleAuditWorkspace(context.Background(), snapshot, "")
					if err != nil || !fact.Qualified {
						t.Fatal("workspace query blocked", fact, err)
					}
					if err := l.revokeAuthentication(context.Background(), state); err != nil {
						t.Fatal("authentication stop blocked", err)
					}
					if err := l.closeControl(context.Background(), state); err != nil {
						t.Fatal(err)
					}
					if _, _, err := ReadAuthenticationStopEvidence(l.setup.RunDirectory, l.setup.Snapshot, state); err != nil {
						t.Fatal("formal cleanup proof missing", err)
					}
					if control.sends != 4 || control.creates != 1 || control.closes != 1 || host.nativeSends != 1 {
						t.Fatal("queries created resources or repeated business input", control.sends, control.creates, control.closes, host.nativeSends)
					}
					if len(control.readCursors) < 2 || control.readCursors[0] != 0 || control.readCursors[1] != control.streamOldest+1000 {
						t.Fatal("did not page original retained stream", control.readCursors)
					}
				})
			}
		})
	}
}

func TestFixedControlStreamBoundaries(t *testing.T) {
	for _, shape := range []string{"empty", "evicted-empty", "partial-line", "one-page", "exact-page", "exact-two-pages"} {
		t.Run(shape, func(t *testing.T) {
			l, _, snapshot, control := fixtureFixedControl(t)
			snapshot = prepareFixedControl(t, l, snapshot, control)
			state, _ := l.state(snapshot)
			rows := 0
			switch shape {
			case "evicted-empty":
				control.streamOldest = 5000
			case "one-page":
				rows = 1
			case "exact-page":
				rows = 1000
			case "exact-two-pages":
				rows = 2000
			}
			control.stream = make([]string, rows)
			for i := range control.stream {
				control.stream[i] = "complete output"
			}
			if shape == "partial-line" {
				original := l.client
				l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
					output, err := original.Raw(ctx, timeout, args...)
					if err != nil || len(args) < 2 || args[1] != "read" {
						return output, err
					}
					var envelope map[string]any
					_ = json.Unmarshal(output, &envelope)
					page := envelope["result"].(map[string]any)["terminal"].(map[string]any)
					page["tail"] = []string{codex.RemoteControlPrefix + "incomplete"}
					return json.Marshal(envelope)
				}})
			}
			control.readCursors = nil
			lines, err := l.controlLines(context.Background(), state, false)
			if err != nil || len(lines) != rows || len(control.readCursors) != max(1, (rows+999)/1000) {
				t.Fatal("complete snapshot or pending partial line mishandled", lines, err, control.readCursors)
			}
		})
	}
}

func TestFixedControlStreamFaults(t *testing.T) {
	for _, fault := range []string{"complete", "growing-stream", "missing-source", "screen", "missing-next", "invalid-next", "missing-oldest", "missing-limited", "missing-truncated", "no-progress", "premature-end", "lost-page", "rewound-stream", "missing-row", "wrong-runtime", "wrong-terminal", "changed-incarnation", "lost-read", "cancelled"} {
		t.Run(fault, func(t *testing.T) {
			l, host, snapshot, control := fixtureFixedControl(t)
			snapshot = prepareFixedControl(t, l, snapshot, control)
			state, _ := l.state(snapshot)
			q := codex.RemoteControlRequest{ID: contracts.NewID(), Binding: control.setup.Binding, Method: "authentication-stop", Session: control.ready.SessionID}
			response := codex.RemoteControlResponse{ID: q.ID, Binding: q.Binding, RequestDigest: codex.RemoteControlDigest(q), Authentication: &codex.RemoteAuthenticationCleanup{Readiness: control.ready, Revoked: true, BrokerClosed: true, SocketCleaned: true, Mode: "host-gh-repository-broker"}}
			raw, _ := json.Marshal(response)
			control.stream = make([]string, 1001)
			for i := range control.stream {
				control.stream[i] = "earlier control output"
			}
			control.stream[1000] = codex.RemoteControlPrefix + base64.RawURLEncoding.EncodeToString(raw)
			original, reads := l.client, 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				if len(args) < 2 || args[0] != "terminal" || args[1] != "read" {
					return original.Raw(ctx, timeout, args...)
				}
				reads++
				if reads == 2 && fault == "growing-stream" {
					// This malformed later output must not enter the first snapshot.
					control.stream = append(control.stream, codex.RemoteControlPrefix+"%%%")
					control.stream = append(control.stream, make([]string, 1500)...)
				}
				output, err := original.Raw(ctx, timeout, args...)
				if err != nil {
					return output, err
				}
				var envelope map[string]any
				if err := json.Unmarshal(output, &envelope); err != nil {
					t.Fatal(err)
				}
				page := envelope["result"].(map[string]any)["terminal"].(map[string]any)
				if reads == 1 {
					switch fault {
					case "no-progress":
						page["tail"], page["nextCursor"] = []string{}, "0"
					case "premature-end":
						page["limited"] = false
					}
				}
				if reads == 2 {
					switch fault {
					case "missing-source":
						delete(page, "source")
					case "screen":
						page["source"] = "screen"
					case "missing-next":
						delete(page, "nextCursor")
					case "invalid-next":
						page["nextCursor"] = "not-a-cursor"
					case "missing-oldest":
						delete(page, "oldestCursor")
					case "missing-limited":
						delete(page, "limited")
					case "missing-truncated":
						delete(page, "truncated")
					case "lost-page":
						page["oldestCursor"], page["truncated"] = "1001", true
					case "rewound-stream":
						page["latestCursor"] = "999"
					case "missing-row":
						page["tail"] = []string{}
					case "wrong-runtime":
						envelope["_meta"].(map[string]any)["runtimeId"] = "foreign"
					case "wrong-terminal":
						page["handle"] = "foreign"
					case "changed-incarnation":
						control.mode = "wrong-incarnation"
					case "lost-read":
						return nil, errors.New("lost read receipt")
					case "cancelled":
						cancel()
					}
				}
				return json.Marshal(envelope)
			}})
			value, err := l.dispatchControl(ctx, state, q)
			want := fault == "complete" || fault == "growing-stream"
			if (err == nil) != want || want && (value.ID != q.ID || value.Authentication == nil || validateAuthenticationStop(control.ready, *value.Authentication) != nil) {
				t.Fatal("incorrect original response qualification", fault, value, err)
			}
			if reads != 2 && fault != "no-progress" && fault != "premature-end" || reads != 1 && (fault == "no-progress" || fault == "premature-end") || control.sends != 0 || host.nativeSends != 0 || control.creates != 1 {
				t.Fatal("read fault dispatched extra input or chased output", reads, control.sends, host.nativeSends, control.creates)
			}
		})
	}
}
