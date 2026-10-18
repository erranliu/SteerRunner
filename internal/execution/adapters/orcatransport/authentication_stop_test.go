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
	"testing"
	"time"
)

func TestRemoteAuthenticationStopReconcilesOnlyOriginalFacility(t *testing.T) {
	for _, mode := range []string{"success", "lost-read", "lost-close-absent", "unknown-create", "wrong-session", "wrong-runtime", "wrong-incarnation", "revocation-unconfirmed"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s := fixture(t)
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, receipt)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			state.RemoteReadiness = &codex.RemoteReadiness{AuthenticationControl: true, SessionID: "original", ConfigurationDigest: "frozen", Process: b.ProcessIdentity{PID: 1, Created: "original-created"}}
			f.callerFileWriter = func(path string, raw []byte) error {
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					return err
				}
				return os.WriteFile(path, raw, 0600)
			}
			creates, reads, closes, businessCloses := 0, 0, 0, 0
			actor := map[string]any{"handle": "auth-actor", "incarnationId": "auth-inc", "worktreeId": state.Process.WorktreeID, "connected": true, "status": "running"}
			proof := codex.RemoteAuthenticationCleanup{Readiness: *state.RemoteReadiness, Revoked: true, BrokerClosed: true, SocketCleaned: true, Mode: "host-gh-repository-broker"}
			if mode == "wrong-session" {
				proof.Readiness.SessionID = "foreign"
			}
			if mode == "revocation-unconfirmed" {
				proof.Revoked = false
			}
			raw, _ := json.Marshal(proof)
			original := l.client
			l.client = native.New(native.Config{Output: func(ctx context.Context, _ time.Duration, args ...string) ([]byte, error) {
				delegate := func() ([]byte, error) {
					got, e := original.MetaCall(ctx, args...)
					return json.Marshal(map[string]any{"ok": e == nil, "result": json.RawMessage(got.Payload), "_meta": map[string]any{"runtimeId": got.RuntimeID}})
				}
				terminal := ""
				for i, arg := range args {
					if arg == "--terminal" && i+1 < len(args) {
						terminal = args[i+1]
					}
				}
				result := any(nil)
				runtime := state.Process.RuntimeID
				action := args[0] + " " + args[1]
				switch action {
				case "terminal create":
					creates++
					if mode == "unknown-create" {
						return nil, errors.New("lost create")
					}
					if mode == "wrong-runtime" {
						runtime = "foreign"
					}
					result = map[string]any{"terminal": actor}
				case "terminal show":
					if terminal == state.Process.Terminal {
						return delegate()
					}
					shown := actor
					if mode == "wrong-incarnation" {
						shown = map[string]any{"handle": "auth-actor", "incarnationId": "foreign-inc", "worktreeId": state.Process.WorktreeID, "connected": true}
					}
					result = map[string]any{"terminal": shown}
				case "terminal read":
					reads++
					if mode == "lost-read" && reads == 1 {
						return nil, errors.New("read lost")
					}
					result = map[string]any{"terminal": map[string]any{"handle": "auth-actor", "tail": []string{codex.RemoteAuthenticationPrefix + base64.RawURLEncoding.EncodeToString(raw)}}}
				case "terminal close":
					if terminal == state.Process.Terminal {
						businessCloses++
						return delegate()
					}
					closes++
					if mode == "lost-close-absent" {
						return nil, errors.New("close response lost")
					}
					result = map[string]any{"close": map[string]any{"handle": "auth-actor", "ptyKilled": true}}
				case "terminal list":
					result = map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"original"}, "omittedHostIds": []string{}}}
				default:
					got, e := original.MetaCall(ctx, args...)
					return json.Marshal(map[string]any{"ok": e == nil, "result": json.RawMessage(got.Payload), "_meta": map[string]any{"runtimeId": got.RuntimeID}})
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
			}})
			err = l.revokeAuthentication(context.Background(), state)
			if mode == "lost-read" {
				if err == nil {
					t.Fatal("lost read falsely confirmed")
				}
				err = l.revokeAuthentication(context.Background(), state)
			}
			success := mode == "success" || mode == "lost-read" || mode == "lost-close-absent"
			if (err == nil) != success {
				t.Fatal("unexpected stop result", mode, err)
			}
			if creates != 1 {
				t.Fatal("original revoker was replaced", creates)
			}
			if success {
				if closes != 1 {
					t.Fatal("actor closure missing or duplicated", closes)
				}
				if err = l.revokeAuthentication(context.Background(), state); err != nil || creates != 1 || closes != 1 {
					t.Fatal("completed revocation repeated an effect", err, creates, closes)
				}
				if _, _, err = ReadAuthenticationStopEvidence(l.setup.RunDirectory, l.setup.Snapshot, state); err != nil {
					t.Fatal("original cleanup evidence unavailable", err)
				}
				changed := l.setup.Snapshot.Clone()
				changed.ConfigurationDigest += "foreign"
				if _, _, err = ReadAuthenticationStopEvidence(l.setup.RunDirectory, changed, state); err == nil {
					t.Fatal("foreign execution accepted cleanup proof")
				}
			} else {
				if closes != 0 {
					t.Fatal("invalid proof closed an actor", closes)
				}
				if mode == "unknown-create" || mode == "wrong-runtime" {
					_ = l.revokeAuthentication(context.Background(), state)
					if creates != 1 {
						t.Fatal("unknown revoker creation repeated")
					}
				}
				if mode == "revocation-unconfirmed" {
					s.Implementation, err = l.envelope(s, state)
					if err != nil {
						t.Fatal(err)
					}
					r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
					stopped, stopErr := l.StopLifecycle(context.Background(), r, nil)
					if stopErr == nil || !stopped.State.WorkerAbsent || stopped.State.Stopped || stopped.Disposition == b.OperationConfirmed || businessCloses != 1 || closes != 1 || creates != 1 {
						t.Fatal("credential failure kept worker running or falsely completed stop", stopped, stopErr, businessCloses, closes, creates)
					}
					r.Operation.Receipt, _ = json.Marshal(stopped)
					again, _, reconcileErr := l.reconcileStop(context.Background(), r)
					if reconcileErr == nil || !again.State.WorkerAbsent || again.State.Stopped || businessCloses != 1 || closes != 1 || creates != 1 {
						t.Fatal("credential uncertainty was promoted or original physical stop repeated", again, reconcileErr, businessCloses, closes, creates)
					}
				}
			}
		})
	}
}

func TestAuthenticationStopRejectsNonCodexReadiness(t *testing.T) {
	if _, _, err := ReadAuthenticationStopEvidence(t.TempDir(), b.LifecycleSnapshot{}, State{}); err == nil {
		t.Fatal("missing original Codex authentication binding accepted")
	}
}
