package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

type creationTestHost struct {
	b.ManagedProcessHost
	output func(context.Context, b.LaunchCommand) ([]byte, error)
	write  func(string, []byte) error
	absent bool
}

func (h *creationTestHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	return h.output(ctx, c)
}
func (h *creationTestHost) WriteExecutionFile(path string, raw []byte) error {
	if h.write != nil {
		return h.write(path, raw)
	}
	return process.New(time.Second).WriteExecutionFile(path, raw)
}
func (h *creationTestHost) InspectProcess(p *b.ProcessIdentity) string {
	if h.absent {
		return "process absent"
	}
	return process.InspectProcess(p)
}

func ownedCreationFixture(t *testing.T, environment string) (*Lifecycle, *fakeHost, b.LifecycleSnapshot, *creationTestHost) {
	t.Helper()
	l, f, s := fixtureOn(t, environment)
	l.profile.Executable = os.Args[0]
	l.setup.CommandTimeout = 3 * time.Second
	h := &creationTestHost{ManagedProcessHost: f, output: f.OutputCommand}
	l.setup.ProcessHost = h
	l.client = native.New(native.Config{Target: l.profile.Target(), CommandTimeout: "1s", Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
		return h.OutputCommand(ctx, b.LaunchCommand{Executable: l.profile.Executable, Args: args, Timeout: d})
	}})
	return l, f, s, h
}

func creationCount(f *fakeHost, name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if len(c) >= 2 && c[0]+" "+c[1] == name {
			n++
		}
	}
	return n
}

func TestCreationOwnerRetainsWorkspaceAcrossCallerLoss(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, f, s, host := ownedCreationFixture(t, environment)
			l.setup.AgentLaunchCommand, f.commandMode = "opaque-start", true
			entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			host.output = func(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
				raw, err := f.OutputCommand(ctx, c)
				if c.Args[0] == "worktree" && c.Args[1] == "create" {
					close(entered) // The effect happened before its response was delivered.
					select {
					case <-release:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				return raw, err
			}
			launches := 0
			l.setup.LaunchExecutionOwner = func(_ context.Context, dir string) error {
				launches++
				go func() { finished <- serveCreationOwner(context.Background(), dir, host, false) }()
				return nil
			}
			r := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
			ctx, cancel := context.WithCancel(context.Background())
			observed := make(chan error, 1)
			go func() { _, _, err := l.Execute(ctx, r); observed <- err }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("original create was not dispatched")
			}
			cancel()
			if err := <-observed; !errors.Is(err, context.Canceled) {
				t.Fatal("caller did not detach", err)
			}
			close(release)
			if err := <-finished; err != nil {
				t.Fatal(err)
			}
			// A new lifecycle has no in-memory creation response or service receipt.
			next := &Lifecycle{setup: l.setup.Clone(), profile: l.profile, profileVersion: l.profileVersion, client: l.client}
			next.setup.LaunchExecutionOwner = func(context.Context, string) error { t.Fatal("replacement owner launched"); return nil }
			recovered, _, err := next.Reconcile(context.Background(), r)
			if err != nil || recovered.Disposition != b.OperationConfirmed || recovered.Process == nil || recovered.Process.Terminal != "terminal" || recovered.Process.Incarnation != f.incarnation || recovered.Workspace == nil || recovered.Workspace.Base != s.WorkspacePlan.BaseCommit {
				t.Fatal("original create was not recovered", recovered, err)
			}
			second, _, err := next.Reconcile(context.Background(), r)
			if err != nil || !reflect.DeepEqual(second.Process, recovered.Process) || launches != 1 || creationCount(f, "worktree create") != 1 || creationCount(f, "terminal send") != 0 || creationCount(f, "worktree list") != 0 {
				t.Fatal("recovery replaced/resent/rediscovered original resources", err, launches)
			}
		})
	}
}

func TestCreationOwnerUnknownAndForeignEvidenceNeverRedispatch(t *testing.T) {
	for _, mode := range []string{"wire-loss", "missing-result", "corrupt-result", "foreign-result", "changed-output", "foreign-operation", "changed-command", "missing-plan", "write-failed"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, host := ownedCreationFixture(t, "")
			var launches atomic.Int32
			if mode == "wire-loss" {
				host.output = func(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
					if c.Args[0] == "worktree" && c.Args[1] == "create" {
						_, _ = f.OutputCommand(ctx, c)
						return nil, errors.New("external create succeeded but no resource identity returned")
					}
					return f.OutputCommand(ctx, c)
				}
			}
			if mode == "write-failed" {
				host.write = func(path string, raw []byte) error {
					if filepath.Base(path) == "create-result.json" {
						return errors.New("receipt persistence unavailable")
					}
					return process.New(time.Second).WriteExecutionFile(path, raw)
				}
			}
			l.setup.LaunchExecutionOwner = func(_ context.Context, dir string) error {
				launches.Add(1)
				return serveCreationOwner(context.Background(), dir, host, false)
			}
			r := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
			_, _, _ = l.Execute(context.Background(), r)
			key := l.creationKey(s, r.Operation.ID, r.Operation.PayloadDigest, "workspace")
			dir := creationDirectory(l.setup.RunDirectory, key)
			path := filepath.Join(dir, "create-result.json")
			raw, _ := os.ReadFile(path)
			var result creationResult
			_ = json.Unmarshal(raw, &result)
			switch mode {
			case "missing-result":
				_ = os.Remove(path)
				host.absent = true
			case "corrupt-result":
				_ = os.WriteFile(path, []byte("{"), 0600)
			case "foreign-result":
				result.PlanDigest = "foreign-operation"
				raw, _ = json.Marshal(result)
				_ = os.WriteFile(path, raw, 0600)
			case "changed-output":
				result.Output = []byte(`{"ok":true,"result":{}}`)
				raw, _ = json.Marshal(result)
				_ = os.WriteFile(path, raw, 0600)
			case "foreign-operation":
				r.Operation.ID += "-foreign"
			case "changed-command":
				r.Input.Workspace.BaseCommit = "ffffffffffffffffffffffffffffffffffffffff"
			case "missing-plan":
				_ = os.Remove(filepath.Join(dir, CreationOwnerConfigFile))
			}
			for i := 0; i < 2; i++ {
				receipt, _, err := l.Reconcile(context.Background(), r)
				if err == nil || receipt.Disposition != b.OperationUnknown || receipt.Process != nil {
					t.Fatal("unverified creation was adopted", mode, receipt, err)
				}
			}
			if launches.Load() != 1 || creationCount(f, "worktree create") != 1 || creationCount(f, "worktree list") != 0 || creationCount(f, "terminal send") != 0 {
				t.Fatal("unknown creation repeated effects or guessed ownership", mode)
			}
			if err := serveCreationOwner(context.Background(), dir, host, false); err == nil {
				t.Fatal("replacement creation owner was admitted")
			}
		})
	}
}

func TestCreationOwnerControlReceiptSurvivesServiceLoss(t *testing.T) {
	l, f, s, control := fixtureFixedControl(t)
	l.profile.Executable = os.Args[0]
	original := l.client
	host := &creationTestHost{ManagedProcessHost: f, output: func(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
		return original.Raw(ctx, c.Timeout, c.Args...)
	}}
	l.setup.ProcessHost = host
	l.setup.CommandTimeout = 3 * time.Second
	l.setup.LaunchExecutionOwner = func(_ context.Context, dir string) error {
		return serveCreationOwner(context.Background(), dir, host, false)
	}
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	input := l.controlIntent(s, state)
	r := request(s, b.LifecycleCreateResource, b.LifecycleInput{Resource: &input})
	first, _, err := l.Execute(context.Background(), r)
	if err != nil || first.Disposition != b.OperationConfirmed {
		t.Fatal(first, err)
	}
	// Do not merge the response. Recovery starts from the old persisted snapshot.
	second, _, err := l.Reconcile(context.Background(), r)
	if err != nil || second.Disposition != b.OperationConfirmed || control.creates != 1 || control.sends != 0 || !reflect.DeepEqual(first.Implementation, second.Implementation) {
		t.Fatal("control creation was repeated or rebound", second, err, control.creates)
	}
}

func TestCreationOwnerIncompleteLaunchClaimNeverRetries(t *testing.T) {
	for _, mode := range []string{"plan-write-failed", "launch-failed", "launch-response-lost", "completed"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, host := ownedCreationFixture(t, "")
			launches := 0
			if mode == "plan-write-failed" {
				host.write = func(path string, raw []byte) error {
					if filepath.Base(path) == CreationOwnerConfigFile {
						return errors.New("original plan write failed")
					}
					return process.New(time.Second).WriteExecutionFile(path, raw)
				}
			}
			l.setup.LaunchExecutionOwner = func(_ context.Context, dir string) error {
				launches++
				if mode == "launch-failed" {
					return errors.New("launch outcome unknown")
				}
				err := serveCreationOwner(context.Background(), dir, host, false)
				if mode == "launch-response-lost" && err == nil {
					return errors.New("caller lost original owner readiness response")
				}
				return err
			}
			r := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
			_, _, _ = l.Execute(context.Background(), r)
			// Even accidental repeated Execute cannot turn a missing result or
			// incomplete plan into permission to dispatch a second owner.
			for i := 0; i < 2; i++ {
				_, _, _ = l.Execute(context.Background(), r)
				_, _, _ = l.Reconcile(context.Background(), r)
			}
			wantCreates, wantLaunches := 1, 1
			if mode == "plan-write-failed" {
				wantCreates, wantLaunches = 0, 0
			} else if mode == "launch-failed" {
				wantCreates = 0
			}
			if launches != wantLaunches || creationCount(f, "worktree create") != wantCreates || creationCount(f, "terminal send") != 0 {
				t.Fatal("original launch claim repeated an effect", mode, launches, creationCount(f, "worktree create"))
			}
		})
	}
}

func TestCreationOwnerInitialTerminalReceiptAndEmptyRecovery(t *testing.T) {
	for _, mode := range []string{"receipt-before-checkpoint", "reconcile-empty"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, host := ownedCreationFixture(t, "")
			// First obtain an exact worktree creation response, as preparation does.
			original, err := l.client.MetaCall(context.Background(), "worktree", "create", "--name", "original", "--base-branch", s.WorkspacePlan.BaseCommit)
			if err != nil {
				t.Fatal(err)
			}
			var c workspaceCreation
			_ = json.Unmarshal(original.Payload, &c)
			c.RuntimeID, c.AgentTerminalHandle = original.RuntimeID, ""
			creates, launches := 0, 0
			made := false
			host.output = func(ctx context.Context, command b.LaunchCommand) ([]byte, error) {
				if command.Args[0] == "terminal" && command.Args[1] == "list" && !made {
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"original-host"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": c.RuntimeID}})
				}
				if command.Args[0] == "terminal" && command.Args[1] == "create" {
					creates++
					made = true
					return f.OutputCommand(ctx, b.LaunchCommand{Args: []string{"terminal", "show", "--terminal", "terminal"}})
				}
				return f.OutputCommand(ctx, command)
			}
			l.setup.LaunchExecutionOwner = func(_ context.Context, dir string) error {
				launches++
				return serveCreationOwner(context.Background(), dir, host, false)
			}
			if mode == "reconcile-empty" {
				if _, err := l.initialWorkspaceTerminalRecovery(context.Background(), s, c, true); err == nil || creates != 0 || launches != 0 {
					t.Fatal("reconciliation created an unstarted terminal", err, creates, launches)
				}
				return
			}
			rejectCheckpoint := true
			host.write = func(path string, raw []byte) error {
				var checkpoint initialTerminalCheckpoint
				if filepath.Base(filepath.Dir(path)) == "orca-initial-terminals" && json.Unmarshal(raw, &checkpoint) == nil && checkpoint.Process != nil && rejectCheckpoint {
					return errors.New("caller checkpoint commit unavailable")
				}
				return process.New(time.Second).WriteExecutionFile(path, raw)
			}
			if _, err := l.initialWorkspaceTerminal(context.Background(), s, c); err == nil {
				t.Fatal("checkpoint failure not injected")
			}
			rejectCheckpoint = false
			terminal, err := l.initialWorkspaceTerminalRecovery(context.Background(), s, c, true)
			if err != nil || terminal.Handle != "terminal" || terminal.IncarnationID != f.incarnation || creates != 1 || launches != 1 {
				t.Fatal("original initial terminal was lost or replaced", terminal, err, creates, launches)
			}
		})
	}
}
