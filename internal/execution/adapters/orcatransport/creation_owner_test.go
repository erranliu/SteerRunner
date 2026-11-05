package orcatransport

import (
	"encoding/json"

	"path/filepath"

	"sync/atomic"
)
import (
	"context"

	"errors"
	"os"

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
	l, f, s := workspaceFixtureOn(t, environment)
	l.profile.Executable = os.Args[0]
	l.setup.CommandTimeout = 3 * time.Second
	h := &creationTestHost{ManagedProcessHost: f, output: f.OutputCommand}
	l.setup.ProcessHost = h
	l.client = native.New(native.Config{CommandTimeout: "1s", Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
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

func TestStructuredWorkspaceRecoversOriginalOwnerOutput(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, f, s, host := ownedCreationFixture(t, environment)
			s.Ref.Protocol = b.ProtocolOrcaAgentSessionV1
			l.setup.Snapshot = s.Clone()
			var blockOriginalRead = true
			host.output = func(ctx context.Context, command b.LaunchCommand) ([]byte, error) {
				raw, err := f.OutputCommand(ctx, command)
				switch command.Args[0] + " " + command.Args[1] {
				case "worktree show":
					if blockOriginalRead {
						return nil, errors.New("service detached before workspace receipt commit")
					}
				case "terminal list":
					return []byte(`{"ok":true,"result":{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"hostIds":["local"],"omittedHostIds":[]}},"_meta":{"runtimeId":"runtime"}}`), nil
				}
				return raw, err
			}
			launches := 0
			l.setup.LaunchExecutionOwner = func(ctx context.Context, dir string) error {
				launches++
				return serveCreationOwner(ctx, dir, host, false)
			}
			r := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
			first, _, err := l.Execute(context.Background(), r)
			if err == nil || first.Disposition != b.OperationUnknown || first.Process != nil || len(first.Operation.Receipt) == 0 {
				t.Fatal("expected owner output before a failed original workspace query", first, err)
			}
			blockOriginalRead = false
			discovery := creationCount(f, "repo list") + creationCount(f, "status --json")
			// Reconstruct the lifecycle with no committed operation receipt, as
			// after service loss before the operation transaction was saved.
			next := &Lifecycle{setup: l.setup.Clone(), profile: l.profile, profileVersion: l.profileVersion, client: l.client}
			recovered, _, err := next.Reconcile(context.Background(), r)
			if err != nil || recovered.Disposition != b.OperationConfirmed || recovered.Process == nil ||
				recovered.Process.Kind != "orca_agent_session" || recovered.Process.RuntimeID != "runtime" ||
				recovered.Process.WorktreeID != f.workspace["id"] || string(recovered.Operation.Receipt) != string(first.Operation.Receipt) {
				t.Fatal("did not recover the exact original workspace binding", recovered, err)
			}
			if launches != 1 || creationCount(f, "worktree create") != 1 || creationCount(f, "worktree list") != 0 ||
				creationCount(f, "repo list")+creationCount(f, "status --json") != discovery ||
				creationCount(f, "terminal create") != 0 || creationCount(f, "terminal send") != 0 {
				t.Fatal("recovery repeated a discovery or mutation", launches, f.calls)
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

func TestCreationOwnerStartupUsesOriginalDeadline(t *testing.T) {
	l, f, s, _ := ownedCreationFixture(t, "")
	l.setup.CommandTimeout = 80 * time.Millisecond
	launches := 0
	l.setup.LaunchExecutionOwner = func(ctx context.Context, dir string) error {
		launches++
		plan, err := readCreationPlan(dir)
		if err != nil {
			return err
		}
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(plan.Deadline) {
			return errors.New("startup is not bounded by the original creation deadline")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	r := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
	receipt, _, err := l.Execute(context.Background(), r)
	if !errors.Is(err, context.DeadlineExceeded) || receipt.Disposition != b.OperationUnknown {
		t.Fatal("owner startup escaped original creation deadline", receipt.Disposition, err)
	}
	_, _, _ = l.Reconcile(context.Background(), r)
	_, _, _ = l.Execute(context.Background(), r)
	if launches != 1 || creationCount(f, "worktree create") != 0 {
		t.Fatal("timed out original startup was relaunched", launches)
	}
}
