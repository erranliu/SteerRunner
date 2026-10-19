package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"testing"
	"time"
)

func TestTerminalStopReconcilesOnlyProvenUnattemptedClose(t *testing.T) {
	for _, attempted := range []bool{false, true} {
		t.Run(map[bool]string{false: "offline-before-close", true: "lost-close-response"}[attempted], func(t *testing.T) {
			l, f, s := fixture(t)
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
			f.failShow = !attempted
			f.unknownClose = attempted
			r, err = l.StopLifecycle(context.Background(), q, nil)
			if err == nil || r.State.Stopped {
				t.Fatal("fault did not remain unverified", r, err)
			}
			q.Operation.Receipt, _ = json.Marshal(r)
			f.failShow = false
			f.unknownClose = false
			before := len(f.calls)
			r, _, err = l.Reconcile(context.Background(), q)
			closes := 0
			for _, c := range f.calls[before:] {
				if c[0] == "terminal" && c[1] == "close" {
					closes++
				}
			}
			if attempted {
				if closes != 0 || r.State.Stopped {
					t.Fatal("unknown close was repeated or overstated", closes, r, err)
				}
			} else if err != nil || !r.State.Stopped || closes != 1 {
				t.Fatal("proven unattempted original stop did not finish", closes, r, err)
			}
			if !attempted {
				q.Operation.Receipt, _ = json.Marshal(r)
				before = len(f.calls)
				again, _, replayErr := l.Reconcile(context.Background(), q)
				if replayErr != nil || !again.State.Stopped || len(f.calls) != before {
					t.Fatal("confirmed stop replay repeated a close or lost proof")
				}
			}
			// Missing, corrupt and foreign progress must never authorize a close.
			for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"operation":{"id":"foreign"}}`)} {
				q.Operation.Receipt = bad
				before = len(f.calls)
				r, _, _ = l.Reconcile(context.Background(), q)
				if r.State.Stopped || len(f.calls) != before {
					t.Fatal("unbound stop progress caused an effect")
				}
			}
		})
	}
}

func TestTerminalStopProgressCannotCrossIncarnation(t *testing.T) {
	l, f, s := fixture(t)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	f.failShow = true
	r, _ = l.StopLifecycle(context.Background(), q, nil)
	q.Operation.Receipt, _ = json.Marshal(r)
	f.failShow = false
	f.incarnation = "replacement"
	before := len(f.calls)
	r, _, err = l.Reconcile(context.Background(), q)
	if err == nil || r.State.Stopped {
		t.Fatal("replacement incarnation was accepted")
	}
	for _, c := range f.calls[before:] {
		if c[0] == "terminal" && c[1] == "close" {
			t.Fatal("replacement incarnation was closed")
		}
	}
}

func TestTerminalStoppedNativeRemoteScope(t *testing.T) {
	for _, mode := range []string{"initializing-branch-late", "initializing-restart-branch-late", "started-branch-late", "initializing-foreign-branch", "initializing-foreign-worktree", "initializing-foreign-path", "live", "already-absent", "lost-close", "lost-close-still-live", "truncated", "missing-host", "wrong-incarnation", "wrong-close-handle", "accepted-close", "mixed-control", "legacy", "legacy-state-v5", "legacy-state-v4", "native-state-v5"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s := fixtureOn(t, "paired-host")
			created, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, created)
			if strings.HasPrefix(mode, "initializing-") || mode == "started-branch-late" {
				state, e := l.state(s)
				if e != nil {
					t.Fatal(e)
				}
				l.setup.AgentLaunchCommand = "codex-frozen-start"
				state.LaunchAttempted = true
				state.LaunchAccepted = true
				state.LaunchRequest = nativeRequest("0123456789abcdef0123456789abcdef")
				state.LaunchDigest = digest([]byte(l.setup.AgentLaunchCommand))
				state.LaunchConfirmed = mode == "started-branch-late"
				s.Implementation, _ = l.envelope(s, state)
				if mode == "initializing-restart-branch-late" {
					setup := l.setup
					setup.Snapshot = s
					reloaded, e := NewLifecycleWithStateFactory(setup, l.stateFactory)
					if e != nil {
						t.Fatal(e)
					}
					l = reloaded.(*Lifecycle)
				}
			}
			if strings.HasPrefix(mode, "legacy") {
				l.profileVersion = ProfileVersion
				// Old state v5 may contain retired per-turn reader fields.
				// Legacy stop reads the bound Process without loading that runtime.
				raw, _ := json.Marshal(map[string]any{"process": s.Ref.External, "turns": []any{map[string]any{"inspection": map[string]any{"unknown": true}}}})
				prior := s.Implementation[0]
				if mode == "legacy-state-v5" {
					prior.CodecVersion = 5
				}
				if mode == "legacy-state-v4" {
					prior.CodecVersion = 4
				}
				e, err := b.NewImplementationDataEnvelope(prior, raw)
				if err != nil {
					t.Fatal(err)
				}
				s.Implementation[0] = e
			}
			if mode == "native-state-v5" {
				prior := s.Implementation[0]
				prior.CodecVersion = 5
				s.Implementation[0], _ = b.NewImplementationDataEnvelope(prior, prior.Payload)
			}
			if mode == "mixed-control" {
				state, _ := l.state(s)
				state.Control = &ControlTerminal{}
				s.Implementation, _ = l.envelope(s, state)
			}
			// Service shutdown detaches its observation and must never close the
			// remote business terminal or claim writer/authentication settlement.
			host := l.setup.ProcessHost.(*fakeHost)
			beforeShutdown := len(host.calls)
			shutdown, shutdownErr := l.ShutdownLifecycle(context.Background(), s, nil)
			if shutdownErr != nil || shutdown.Stopped || shutdown.WorkerAbsent || shutdown.WritersAbsent || len(host.calls) != beforeShutdown {
				t.Fatal("service shutdown affected remote execution", shutdown, shutdownErr)
			}
			live := mode != "already-absent"
			closes := 0
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				row := map[string]any{"handle": s.Ref.External.Terminal, "incarnationId": s.Ref.External.Incarnation, "worktreeId": s.Ref.External.WorktreeID, "worktreePath": s.Workspace.Path, "branch": "refs/heads/" + s.Workspace.Branch, "connected": true}
				if strings.Contains(mode, "branch-late") {
					row["branch"] = ""
				}
				if mode == "initializing-foreign-branch" {
					row["branch"] = "refs/heads/foreign"
				}
				if mode == "initializing-foreign-worktree" {
					row["worktreeId"] = "foreign"
				}
				if mode == "initializing-foreign-path" {
					row["worktreePath"] = "C:/foreign"
				}
				var result any
				switch args[0] + " " + args[1] {
				case "terminal list":
					rows := []any{}
					if live {
						rows = append(rows, row)
					}
					hosts := []string{"local"}
					if mode == "missing-host" {
						hosts = []string{"other"}
					}
					result = map[string]any{"terminals": rows, "totalCount": len(rows), "truncated": mode == "truncated", "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": []string{}}}
				case "terminal show":
					if mode == "wrong-incarnation" {
						row["incarnationId"] = "replacement"
					}
					result = map[string]any{"terminal": row}
				case "terminal close":
					closes++
					if mode != "lost-close-still-live" {
						live = false
					}
					if strings.HasPrefix(mode, "lost-close") {
						return nil, errors.New("lost original close response")
					}
					result = map[string]any{"close": map[string]any{"handle": s.Ref.External.Terminal, "ptyKilled": true}}
					if mode == "wrong-close-handle" {
						result = map[string]any{"close": map[string]any{"handle": "replacement", "ptyKilled": true}}
					}
					if mode == "accepted-close" {
						result = map[string]any{"close": map[string]any{"handle": s.Ref.External.Terminal, "ptyKilled": false}}
					}
				default:
					t.Fatal("native stop invoked helper or created a resource", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			receipt, err := l.StopLifecycle(ctx, r, nil)
			success := mode == "initializing-branch-late" || mode == "initializing-restart-branch-late" || mode == "live" || mode == "already-absent" || mode == "lost-close"
			if (err == nil) != success || receipt.State.Stopped != success || receipt.State.WritersAbsent {
				t.Fatal("stop scope changed", receipt, err)
			}
			if strings.HasPrefix(mode, "legacy") && l.StopEvidenceScope() != "" {
				t.Fatal("legacy evidence was downgraded")
			}
			if !strings.HasPrefix(mode, "legacy") && l.StopEvidenceScope() != "orca_terminal" {
				t.Fatal("scope missing")
			}
			want := 1
			if mode == "started-branch-late" || mode == "initializing-foreign-branch" || mode == "initializing-foreign-worktree" || mode == "initializing-foreign-path" || mode == "already-absent" || mode == "wrong-incarnation" || mode == "mixed-control" || mode == "legacy-state-v4" || mode == "native-state-v5" {
				want = 0
			}
			if closes != want {
				t.Fatal("unexpected close count", closes, want)
			}
			r.Operation.Receipt, _ = json.Marshal(receipt)
			probe, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer stop()
			before := closes
			_, _, _ = l.Reconcile(probe, r)
			if closes != before {
				t.Fatal("close replayed during reconciliation")
			}
		})
	}
}
