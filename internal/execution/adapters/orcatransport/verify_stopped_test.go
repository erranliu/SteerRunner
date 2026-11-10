package orcatransport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestCodexStoppedRequiresNativeExitProofBeyondPTY(t *testing.T) {
	for _, name := range []string{"live-root", "missing-root", "no-readiness"} {
		t.Run(name, func(t *testing.T) {
			environment := ""
			l, host, s := frozenFixtureOn(t, environment)
			s = frozenWorkspaceFixture(t, l, host, s)
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
			before, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			inventoryCalls := 0
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				inventoryCalls++
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
			// Retain the separate current-port rejection: a legacy reader is not
			// itself executable, and an old snapshot cannot settle this port.
			port, currentSnapshot, agent := structuredFixture(t)
			currentState, err := port.agentState(currentSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			currentState.Attempted = true
			currentSnapshot.Implementation, err = port.agentEnvelope(currentSnapshot, currentState)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := port.StopLifecycle(context.Background(), request(currentSnapshot, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{TurnID: "native-turn"}}), nil)
			if err != nil || !receipt.State.Stopped || agent.closes != 1 {
				t.Fatal("original close was not acknowledged", receipt, err, agent.closes)
			}
			currentSnapshot, _, err = reliable.ApplySessionReceipt(currentSnapshot, nil, receipt, false)
			if err != nil {
				t.Fatal(err)
			}
			currentBefore, err := json.Marshal(currentSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			setupBefore := port.setup.Clone()
			facts, err := port.ShutdownLifecycle(context.Background(), currentSnapshot, nil)
			if err != nil || !facts.Stopped {
				t.Fatal("acknowledged current session did not settle", facts, err)
			}
			facts, err = port.ShutdownLifecycle(context.Background(), s, nil)
			if err == nil || err.Error() != "Orca terminal execution binding changed" || facts.Stopped {
				t.Fatal("shutdown overstated native resource settlement", facts, err)
			}
			if agent.closes != 1 || agent.creates != 0 || agent.reads != 0 || agent.details != 0 || len(port.setup.ProcessHost.(*fakeHost).calls) != 0 || inventoryCalls != 1 {
				t.Fatal("shutdown proof caused extra native I/O", agent, inventoryCalls)
			}
			currentAfter, err := json.Marshal(currentSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			if string(currentBefore) != string(currentAfter) || !reflect.DeepEqual(port.setup.Clone(), setupBefore) {
				t.Fatal("shutdown changed the current snapshot or setup")
			}
			after, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("read-only probe changed snapshot")
			}
		})
	}
}

func TestLocalCodexStoppedUsesRegisteredResources(t *testing.T) {
	for _, mode := range []string{"complete", "live-business", "live-control", "missing-exit-file", "missing-control", "pending-control"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			// The frozen resources have exited; verification itself only reads inventory.
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) < 2 || args[1] != "list" {
					t.Fatal("verification effect", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			state, _ := l.state(s)
			if mode == "missing-control" {
				state.Control = nil
				s.Implementation, _ = l.envelope(s, state)
			}
			if mode == "pending-control" {
				// An unfinished historical query journal is not stop evidence.
				if e := os.WriteFile(filepath.Join(l.setup.RunDirectory, "orca-control.json"), []byte(`{"pending":true}`), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "live-business" || mode == "live-control" {
				handle, inc := state.Process.Terminal, state.Process.Incarnation
				if mode == "live-control" {
					handle, inc = state.Control.Process.Terminal, state.Control.Process.Incarnation
				}
				l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
					if len(args) < 2 || args[1] != "list" {
						t.Fatal("offline verification effect", args)
					}
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{map[string]any{"handle": handle, "incarnationId": inc, "worktreeId": state.Process.WorktreeID, "status": "running"}}, "totalCount": 1, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": state.Process.RuntimeID}})
				}})
			}
			before, _ := json.Marshal(s)
			err := l.verifyStopped(context.Background(), s)
			if (err == nil) != (mode == "complete" || mode == "missing-exit-file" || mode == "pending-control") {
				t.Fatal(mode, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("readonly verification changed snapshot")
			}
		})
	}
}

func TestHostNativeReadinessRejectsRetiredFullAccessContract(t *testing.T) {
	l, _, s, c := fixtureFixedControl(t)
	s = prepareFixedControl(t, l, s, c)
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	state.RemoteReadiness.Contract = codex.ReadOnlyPolicyContract
	if _, err = l.codexReadiness(state, s.Workspace.Path); err == nil {
		t.Fatal("retired full-access contract admitted")
	}
}
