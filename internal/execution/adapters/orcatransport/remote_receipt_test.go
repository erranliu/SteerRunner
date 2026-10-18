package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedRemoteReceiptRecoversMissedStartWithoutResending(t *testing.T) {
	for _, mode := range []string{"success", "wrong-runtime", "wrong-incarnation", "wrong-session", "wrong-hash", "wrong-config", "wrong-input", "failed", "unknown-create", "unknown-close", "in-progress", "shutdown-during-close", "shutdown-deadline", "restart-known-close", "restart-absent-close", "restart-unknown-create", "restart-corrupt-journal", "restart-queued", "bounded-first-inspection"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s := fixtureOn(t, "paired-remote")
			f.callerFileWriter = func(path string, data []byte) error {
				rel, err := filepath.Rel(l.setup.RunDirectory, path)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return errors.New("unexpected remote/external file write")
				}
				if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					return err
				}
				return os.WriteFile(path, data, 0600)
			}
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, _ := l.state(s)
			now := time.Now().UTC()
			ready := codex.RemoteReadiness{NativeTransport: codex.RemoteNativeTransport, Verified: true, Contract: codex.PolicyContract, Workspace: s.Workspace.Path, Repository: "https://github.com/erranliu/SteerlineTest.git", SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", RuntimeSHA256: strings.Repeat("b", 64), ConfigurationDigest: strings.Repeat("a", 64), Process: b.ProcessIdentity{PID: 123, Created: "native-process"}, CheckedAt: now, ExpiresAt: now.Add(time.Hour)}
			l.setup.RepositoryURL = ready.Repository
			l.profile.RuntimeExecutable = "C:/runtime/steerrunner-agent.exe"
			l.profile.RuntimeConfigurationDigest = ready.ConfigurationDigest
			l.profile.RuntimeSHA256 = ready.RuntimeSHA256
			state.RemoteReadiness = &ready
			s.Implementation, _ = l.envelope(s, state)
			baseline := fixtureState(1, "READY", "done")
			baseline.ProviderSession.ID = ready.SessionID
			l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
				return &controlledStateFeed{initial: baseline, current: baseline}, nil
			}
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			f.unstarted = true
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, _ = l.state(s)
			turn := &state.Turns[0]
			if turn.Started || !strings.HasPrefix(f.nativePrompt, "[SteerRunner command=") {
				t.Fatal("test did not exercise marked accepted-only input")
			}
			original := l.client
			creates, closes := 0, 0
			recovering := false
			absenceObserved := false
			closeEntered, allowClose := make(chan struct{}), make(chan struct{})
			value := codex.RemoteTurnReceipt{Source: codex.RemoteTurnSource, Readiness: ready, InputDigest: digest([]byte(f.nativePrompt)), NativeTurnID: "01a14502-5798-78f1-aaa3-2d7cd15594c8", UserItemID: "user", Started: true, Completed: true, Status: "completed", FinalItemID: "native-final", FinalText: "exact report"}
			value.FinalDigest = digest([]byte(value.FinalText))
			switch mode {
			case "wrong-session":
				value.Readiness.SessionID = "01a1433a-c857-7fc0-bf8c-02a784ad118f"
			case "wrong-hash":
				value.Readiness.RuntimeSHA256 = strings.Repeat("c", 64)
			case "wrong-config":
				value.Readiness.ConfigurationDigest = strings.Repeat("d", 64)
			case "wrong-input":
				value.InputDigest = strings.Repeat("0", 64)
			case "failed":
				value.Status = "failed"
			case "in-progress":
				value.Completed = false
				value.Status = "inProgress"
				value.FinalText = ""
				value.FinalItemID = ""
				value.FinalDigest = ""
			}
			l.client = native.New(native.Config{CommandTimeout: "1s", Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				aux := strings.Contains(joined, "inspector") || len(args) > 1 && args[0] == "terminal" && (args[1] == "create" || args[1] == "list" && strings.HasPrefix(mode, "restart-"))
				if !aux {
					return original.Raw(ctx, d, args...)
				}
				runtime := "runtime"
				inc := "inspector-inc"
				if mode == "wrong-runtime" {
					runtime = "other"
				}
				if mode == "wrong-incarnation" && args[1] == "show" {
					inc = "other"
				}
				terminal := map[string]any{"handle": "inspector", "incarnationId": inc, "worktreeId": s.Ref.External.WorktreeID, "connected": true, "status": "running"}
				var result any
				switch args[1] {
				case "create":
					if mode == "restart-queued" && !recovering {
						return nil, &native.CLIQueueError{Cause: context.DeadlineExceeded}
					}
					if err := native.CheckpointDispatch(ctx); err != nil {
						return nil, err
					}
					intent, err := l.loadInspections(state)
					if err != nil || len(intent.Entries) == 0 || !intent.Entries[len(intent.Entries)-1].Inspection.Unknown || intent.Entries[len(intent.Entries)-1].Inspection.Process != nil {
						t.Fatal("create crossed dispatch without durable unknown intent", err)
					}
					creates++
					if mode == "unknown-create" || mode == "restart-unknown-create" {
						return nil, errors.New("lost create")
					}
					result = map[string]any{"terminal": terminal}
				case "show":
					if mode == "restart-absent-close" && recovering && !absenceObserved {
						return nil, errors.New("original inspector already absent")
					}
					result = map[string]any{"terminal": terminal}
				case "list":
					if !recovering {
						return nil, errors.New("inventory unavailable during process loss")
					}
					absenceObserved = true
					result = map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"execution-host"}, "omittedHostIds": []string{}}}
				case "read":
					raw, _ := json.Marshal(value)
					terminal["tail"] = []string{codex.RemoteTurnPrefix + base64.RawURLEncoding.EncodeToString(raw)}
					result = map[string]any{"terminal": terminal}
				case "close":
					closes++
					if strings.HasPrefix(mode, "shutdown-") {
						close(closeEntered)
						<-allowClose
					}
					if mode == "unknown-close" || strings.HasPrefix(mode, "restart-") && !recovering {
						return nil, errors.New("lost close")
					}
					result = map[string]any{"close": map[string]any{"handle": "inspector", "ptyKilled": true}}
				default:
					t.Fatal(args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
			}})
			if mode == "bounded-first-inspection" {
				r, _, err := l.observeOnce(context.Background(), s)
				if err != nil || creates != 0 || r.TurnEvidence != nil || f.nativeSends != 1 {
					t.Fatal("active input caused an immediate history resource", err, creates)
				}
			}
			l.clock = func() time.Time { return now.Add(31 * time.Second) }
			if strings.HasPrefix(mode, "shutdown-") {
				o := l.observe(s)
				defer o.Close(context.Background())
				defer func() {
					select {
					case <-allowClose:
					default:
						close(allowClose)
					}
				}()
				select {
				case <-closeEntered:
				case <-time.After(time.Second):
					t.Fatal("original inspector did not reach close")
				}
				_ = o.Close(context.Background()) // Passive cancellation must not await actor event acknowledgements.
				if mode == "shutdown-deadline" {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
					facts, err := l.ShutdownLifecycle(ctx, s, o.StopHandle())
					cancel()
					if err == nil || facts.Stopped {
						t.Fatal("unconfirmed readonly cleanup was hidden", facts, err)
					}
				} else {
					done := make(chan error, 1)
					go func() { _, err := l.ShutdownLifecycle(context.Background(), s, o.StopHandle()); done <- err }()
					select {
					case err := <-done:
						t.Fatal("shutdown returned before original close", err)
					case <-time.After(20 * time.Millisecond):
					}
					close(allowClose)
					select {
					case err := <-done:
						if err != nil {
							t.Fatal(err)
						}
					case <-time.After(time.Second):
						t.Fatal("shutdown did not observe inspector close")
					}
				}
				select {
				case <-allowClose:
				default:
					close(allowClose)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				facts, err := l.ShutdownLifecycle(ctx, s, o.StopHandle())
				if err != nil || facts.Stopped || f.nativeSends != 1 || creates != 1 || closes != 1 {
					t.Fatal("shutdown changed business execution or retried a resource", facts, err, creates, closes, f.nativeSends)
				}
				return
			}
			beforeObservation := s.Clone() // A kill loses the uncommitted observation receipt.
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "success" || mode == "bounded-first-inspection" {
				if r.TurnEvidence == nil || r.TurnEvidence.FinalNativeTurnID != value.NativeTurnID || r.TurnEvidence.FinalSource != codex.RemoteTurnSource || !r.State.TurnStarted || !r.State.TurnCompleted || closes != 1 {
					t.Fatalf("%+v", r)
				}
			} else if r.TurnEvidence != nil {
				t.Fatalf("invalid evidence promoted: %+v", r)
			}
			bind(&s, r)
			if strings.HasPrefix(mode, "restart-") {
				s = beforeObservation
				path, binding, err := l.journalPath(state)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "restart-queued" {
					if _, err := os.Stat(path); !os.IsNotExist(err) || creates != 0 {
						t.Fatal("queued command left an ambiguous create intent", err, creates)
					}
				}
				if mode == "restart-corrupt-journal" {
					j, err := l.loadInspections(state)
					if err != nil {
						t.Fatal(err)
					}
					j.Entries[0].InputDigest = strings.Repeat("0", 64)
					raw, _ := json.Marshal(j)
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
				setup := l.setup.Clone()
				setup.Snapshot = s.Clone()
				port, err := NewLifecycleWithStateFactory(setup, l.stateFactory)
				if err != nil {
					t.Fatal(err)
				}
				restored := port.(*Lifecycle)
				restored.profile, restored.client = l.profile, l.client
				restored.clock = func() time.Time { return now.Add(time.Minute) }
				restoredState, err := restored.state(s)
				if err != nil {
					t.Fatal(err)
				}
				restoredPath, restoredBinding, err := restored.journalPath(restoredState)
				if err != nil || restoredPath != path || restoredBinding != binding {
					t.Fatal("restart changed ownership key", err)
				}
				recovering = true
				r, _, err = restored.observeOnce(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "restart-queued" {
					if r.TurnEvidence == nil || creates != 1 || closes != 1 {
						t.Fatal("never-invoked query did not recover original observation", r.Diagnostic, creates, closes)
					}
				} else if mode == "restart-known-close" || mode == "restart-absent-close" {
					wantCloses := 3 // Lost close, original cleanup, new query cleanup.
					if mode == "restart-absent-close" {
						wantCloses = 2
					}
					if r.TurnEvidence == nil || creates != 2 || closes != wantCloses {
						t.Fatal("known original inspector did not settle before observation", r.Diagnostic, creates, closes)
					}
				} else if r.TurnEvidence != nil || creates != 1 || mode == "restart-unknown-create" && closes != 0 || mode == "restart-corrupt-journal" && closes != 1 {
					t.Fatal("uncertain ownership permitted effects", r.Diagnostic, creates, closes)
				}
			}
			if mode == "in-progress" {
				l.clock = func() time.Time { return now.Add(2 * time.Minute) }
				value.Completed = true
				value.Status = "completed"
				value.FinalText = "exact report"
				value.FinalItemID = "native-final"
				value.FinalDigest = digest([]byte(value.FinalText))
				r, _, err = l.observeOnce(context.Background(), s)
				if err != nil || r.TurnEvidence == nil || creates != 2 || closes != 2 {
					t.Fatal(r.Diagnostic, err, creates, closes)
				}
			}
			if mode == "unknown-create" || mode == "unknown-close" || mode == "wrong-runtime" || mode == "wrong-incarnation" {
				l.clock = func() time.Time { return now.Add(time.Minute) }
				_, _, _ = l.observeOnce(context.Background(), s)
				if creates != 1 {
					t.Fatal("uncertain inspector replaced")
				}
			}
			if f.nativeSends != 1 {
				t.Fatal("business input resent", f.nativeSends)
			}
		})
	}
}

func TestRemoteWireCommandSeparatesIdenticalReportPrompts(t *testing.T) {
	a := Turn{WireFormat: remoteWireCommand, Command: strings.Repeat("a", 32), Prompt: "report\r\n"}
	a.InputDigest = digest([]byte(a.Prompt))
	b := a
	b.Command = strings.Repeat("b", 32)
	if wirePrompt(&a) == wirePrompt(&b) || strings.HasSuffix(wirePrompt(&a), "\n") || !strings.HasSuffix(wirePrompt(&a), "\nreport") {
		t.Fatal("wire identity/canonicalization lost")
	}
}
