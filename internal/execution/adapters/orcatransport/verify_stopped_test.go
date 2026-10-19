package orcatransport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestCodexStoppedRequiresNativeExitProofBeyondPTY(t *testing.T) {
	for _, name := range []string{"live-root", "missing-root", "no-readiness", "remote-live-root"} {
		t.Run(name, func(t *testing.T) {
			environment := ""
			if name == "remote-live-root" {
				environment = "paired-host"
			}
			l, _, s := fixtureOn(t, environment)
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, receipt)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			current, err := process.CurrentIdentity()
			if err != nil {
				t.Fatal(err)
			}
			if name != "no-readiness" {
				state.RemoteReadiness = &codex.RemoteReadiness{SessionID: "original-session", Process: *current}
			}
			if name == "missing-root" {
				state.RemoteReadiness.Process = b.ProcessIdentity{}
			}
			s.Implementation, err = l.envelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s)
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) != 5 || args[0] != "terminal" || args[1] != "list" {
					t.Fatalf("verification caused an effect: %v", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			err = l.verifyStopped(context.Background(), s)
			if err == nil || !strings.Contains(err.Error(), "PTY exit is insufficient") {
				t.Fatal("native exit was inferred from PTY absence", err)
			}
			if process.Probe(current).State != "matches" {
				t.Fatal("live root fixture exited")
			}
			facts, err := l.ShutdownLifecycle(context.Background(), s, nil)
			if err == nil || facts.Stopped {
				t.Fatal("shutdown overstated native resource settlement", facts, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("read-only probe changed snapshot")
			}
		})
	}
}

func TestCodexStoppedRequiresSignedExitAndEveryOriginalReader(t *testing.T) {
	for _, mode := range []string{"complete", "live-business", "changed-signature", "reader-complete", "reader-live", "reader-native-unknown", "unknown-create", "journal-reader", "stop-after-helper-exit"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s := fixtureOn(t, "paired-host")
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, receipt)
			state, _ := l.state(s)
			public, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			ready := codex.RemoteReadiness{ExitPublicKey: public, Owner: b.ProcessIdentity{Kind: "os", PID: 1, Created: "owner", Executable: "helper"}, Process: b.ProcessIdentity{Kind: "os", PID: 2, Created: "worker", Executable: "codex"}, NativeTransport: codex.RemoteNativeTransport, Verified: true, Contract: codex.PolicyContract, RuntimeSHA256: strings.Repeat("a", 64), ConfigurationDigest: strings.Repeat("b", 64), SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", Workspace: s.Workspace.Path, Repository: "https://github.com/erranliu/SteerlineTest.git", CheckedAt: time.Now().UTC()}
			l.profile.RuntimeSHA256, l.profile.RuntimeConfigurationDigest = ready.RuntimeSHA256, ready.ConfigurationDigest
			l.profile.RuntimeExecutable = "C:/runtime/helper.exe"
			l.setup.RepositoryURL = ready.Repository
			l.setup.Snapshot.Workspace = s.Workspace
			state.RemoteReadiness = &ready
			exit, _ := json.Marshal(codex.RemoteExit{Readiness: ready, WritersAbsent: true, AuthenticationClosed: true, ClosedAt: time.Now().UTC()})
			signature := ed25519.Sign(private, exit)
			if mode == "changed-signature" {
				signature[0] ^= 1
			}
			signed, _ := json.Marshal(map[string]any{"body": json.RawMessage(exit), "signature": signature})
			reads := 0
			l.readStopArtifact = func(_ context.Context, env, runtime, worktree, path string) ([]byte, error) {
				reads++
				if env != "paired-host" || runtime != state.Process.RuntimeID || worktree != state.Process.WorktreeID {
					t.Fatal("artifact used another host/resource")
				}
				return signed, nil
			}
			reader := &NativeInspection{Attempted: true, Process: &b.ProcessIdentity{Kind: "orca_original_turn_reader", RuntimeID: state.Process.RuntimeID, WorktreeID: state.Process.WorktreeID, Terminal: "reader", Incarnation: "reader-inc"}, NativeSettled: true, At: time.Now().UTC()}
			if strings.HasPrefix(mode, "reader-") || mode == "unknown-create" || mode == "journal-reader" {
				if mode == "reader-native-unknown" {
					reader.NativeSettled = false
				}
				if mode == "unknown-create" {
					reader.Process = nil
					reader.Unknown = true
				}
				turn := Turn{Command: strings.Repeat("c", 32), WireFormat: remoteWireCommand, Prompt: "report", InputDigest: digest([]byte("report")), Inspection: reader, Native: NativeProof{Pane: "original-pane"}}
				state.Turns = []Turn{turn}
				if mode == "journal-reader" {
					state.Turns[0].Inspection = nil
					name, binding, e := l.journalPath(state)
					if e != nil {
						t.Fatal(e)
					}
					raw, _ := json.Marshal(inspectionJournal{Version: 1, Binding: binding, Entries: []inspectionEntry{{ID: strings.Repeat("e", 32), Command: turn.Command, InputDigest: digest([]byte(wirePrompt(&turn))), Inspection: *reader}}})
					if e = os.MkdirAll(filepath.Dir(name), 0700); e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(name, raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			s.Implementation, err = l.envelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s)
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if mode == "stop-after-helper-exit" && args[1] == "show" {
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminal": map[string]any{"handle": state.Process.Terminal, "incarnationId": state.Process.Incarnation, "worktreeId": state.Process.WorktreeID, "worktreePath": s.Workspace.Path, "branch": s.Workspace.Branch, "status": "running", "connected": true}}, "_meta": map[string]string{"runtimeId": state.Process.RuntimeID}})
				}
				if mode == "stop-after-helper-exit" && args[1] == "close" {
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"close": map[string]any{"handle": state.Process.Terminal, "ptyKilled": true}}, "_meta": map[string]string{"runtimeId": state.Process.RuntimeID}})
				}
				if len(args) < 2 || args[0] != "terminal" || args[1] != "list" {
					t.Fatalf("offline effect: %v", args)
				}
				terminals := []any{}
				if mode == "live-business" {
					terminals = append(terminals, map[string]any{"handle": state.Process.Terminal, "incarnationId": state.Process.Incarnation, "worktreeId": state.Process.WorktreeID, "status": "running"})
				}
				if mode == "reader-live" {
					terminals = append(terminals, map[string]any{"handle": "reader", "incarnationId": "reader-inc", "worktreeId": state.Process.WorktreeID, "status": "running"})
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": terminals, "totalCount": len(terminals), "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": state.Process.RuntimeID}})
			}})
			if mode == "stop-after-helper-exit" {
				result, e := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
				if e != nil || !result.State.Stopped || result.Disposition != b.OperationConfirmed {
					t.Fatal("settled helper shell could not stop", e)
				}
				return
			}
			err = l.verifyStopped(context.Background(), s)
			want := mode == "complete" || mode == "reader-complete" || mode == "journal-reader"
			if (err == nil) != want {
				t.Fatal(mode, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) || mode == "live-business" && reads != 0 {
				t.Fatal("offline verification changed state or trusted artifact before PTY stop")
			}
		})
	}
}
