package host

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Opt-in native contract probe: real PTY and released Runner EXE, no model/PR.
// The current-format Run is an isolated fixture bound to the actually created
// native resource. It never imports installation state or business authority.
func TestOrcaStoppedNativeProbe(t *testing.T) {
	root := os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_ROOT")
	if root == "" {
		t.Skip("requires a new independent debug package probe directory")
	}
	cli, runner, repo, base, repository := os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_CLI"), os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_RUNNER"), os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_REPO"), os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_BASE"), os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_REPOSITORY")
	environment := os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_ENVIRONMENT")
	if !filepath.IsAbs(root) || !strings.HasPrefix(strings.ToLower(filepath.Clean(root)), `c:\steerlinedebug\`) || !filepath.IsAbs(cli) || !filepath.IsAbs(runner) || repo == "" || len(base) != 40 || !filepath.IsAbs(repository) {
		t.Fatal("explicit independent probe identity missing")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	calls := 0
	c := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if environment != "" {
			args = append(args, "--environment", environment)
		}
		callCtx, stop := context.WithTimeout(ctx, timeout)
		defer stop()
		cmd := exec.CommandContext(callCtx, cli, args...)
		cmd.Dir = repository
		// Explicit routing only, independent from any enclosing Orca session.
		for _, item := range os.Environ() {
			key, _, _ := strings.Cut(item, "=")
			if key != "ORCA_ENVIRONMENT" && key != "ORCA_PAIRING_CODE" && key != "ORCA_REMOTE_PAIRING" {
				cmd.Env = append(cmd.Env, item)
			}
		}
		raw, err := cmd.Output()
		calls++
		_ = os.WriteFile(filepath.Join(root, fmt.Sprintf("orca-%02d.json", calls)), raw, 0600)
		return raw, err
	}})
	made, err := c.MetaCall(ctx, "worktree", "create", "--repo", "id:"+repo, "--name", "sr-stop-"+filepath.Base(root), "--base-branch", base, "--no-parent", "--setup", "skip", "--json")
	var creation struct{ Worktree native.Worktree }
	if err != nil || json.Unmarshal(made.Payload, &creation) != nil || creation.Worktree.ID == "" {
		t.Fatal("native worktree creation", err)
	}
	w := creation.Worktree
	cleaned := false
	t.Cleanup(func() {
		defer func() {
			raw, _ := json.Marshal(map[string]any{"passed": !t.Failed(), "resources_cleaned": cleaned, "runtime_id": made.RuntimeID, "worktree_id": w.ID, "environment": environment, "model_inputs": 0, "fixture": "real PTY plus recorded live OS root; PTY exit must not prove native settlement"})
			_ = os.WriteFile(filepath.Join(root, "result.json"), raw, 0600)
		}()
		cleanup, stop := context.WithTimeout(context.Background(), 40*time.Second)
		defer stop()
		if _, e := c.MetaCall(cleanup, "terminal", "close", "--worktree", "id:"+w.ID, "--all", "--json"); e != nil {
			t.Error(e)
			return
		}
		if _, e := c.MetaCall(cleanup, "worktree", "rm", "--worktree", "id:"+w.ID, "--force", "--json"); e != nil {
			t.Error(e)
			return
		}
		cleaned = true
	})
	created, err := c.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+w.ID, "--title", "SteerRunner stop proof probe", "--json")
	var opened struct{ Terminal native.Terminal }
	if err != nil || json.Unmarshal(created.Payload, &opened) != nil {
		t.Fatal(err)
	}
	terminal, err := c.ShowTerminal(ctx, opened.Terminal.Handle)
	if err != nil {
		t.Fatal(err)
	}
	p := b.ProcessIdentity{Kind: "orca_terminal", Executable: cli, Terminal: terminal.Handle, RuntimeID: made.RuntimeID, Incarnation: terminal.IncarnationID, WorktreeID: w.ID, ExecutionHost: "local"}
	if environment != "" {
		p.ExecutionHost = environment
	}
	profile := orca.Profile{Executable: cli, Environment: environment, WaitTimeout: "10s"}
	id, attempt, digest := contracts.NewID(), contracts.NewID(), strings.Repeat("a", 64)
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: id, AttemptID: attempt, External: p}, Session: b.SessionContext{ProjectID: "stop-probe", ExecutionID: id, WorkflowRunID: id, AttemptID: attempt}, ConfigurationDigest: digest, ResourceGeneration: "probe-generation"}
	profileRaw, _ := json.Marshal(profile)
	profileData, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.ProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.ProfileKey, ProjectID: s.Session.ProjectID, ProfileID: id, AcceptedConfigVersion: digest}, profileRaw)
	if err != nil {
		t.Fatal(err)
	}
	rootIdentity, err := process.CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	stateRaw, _ := json.Marshal(orca.State{Process: p, RemoteReadiness: &codex.RemoteReadiness{Process: *rootIdentity, SessionID: "probe-live-root"}})
	stateData, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.StateVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.StateKey, ProjectID: s.Session.ProjectID, ExecutionID: id, AttemptID: attempt, Generation: s.ResourceGeneration, ConfigDigest: digest}, stateRaw)
	if err != nil {
		t.Fatal(err)
	}
	s.Implementation = []b.ImplementationDataEnvelope{stateData}
	dir := filepath.Join(root, "state")
	if err = os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "service", "writer.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := &runstate.Record{ID: id, InputDigest: digest, Revision: 1, Snapshot: s, Profile: []b.ImplementationDataEnvelope{profileData}, RepositoryPath: repository, Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, Status: "blocked", Phase: "probe", CommandStates: map[string]string{}}
	if err = store.Commit(r, 0, nil); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	failure, _ := json.Marshal(contracts.ServiceShutdown{InstanceID: "original-probe", RequestID: "stop-probe", StateDir: dir, ExecutableSHA256: "probe", ResourcesSettled: false, Diagnostic: "original failed shutdown fixture"})
	if err = os.WriteFile(filepath.Join(dir, "service", "shutdown.json"), failure, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	verify := func(label string, want bool) {
		t.Helper()
		cmd := exec.CommandContext(ctx, runner, "service", "verify-stopped", "--state-dir", dir, "--json")
		raw, e := cmd.Output()
		_ = os.WriteFile(filepath.Join(root, label+".json"), raw, 0600)
		var response struct {
			OK   bool                          `json:"ok"`
			Data contracts.StoppedVerification `json:"data"`
		}
		if e != nil || json.Unmarshal(raw, &response) != nil || !response.OK || response.Data.ResourcesSettled != want || len(response.Data.Runs) != 1 || response.Data.Runs[0].RunID != id || response.Data.Runs[0].InputDigest != digest {
			t.Fatalf("%s: %s %v", label, raw, e)
		}
		after, e := os.ReadFile(filepath.Join(dir, "runner.db"))
		if e != nil || !bytes.Equal(before, after) {
			t.Fatal("verification changed database", e)
		}
		after, e = os.ReadFile(filepath.Join(dir, "service", "shutdown.json"))
		if e != nil || !bytes.Equal(failure, after) {
			t.Fatal("verification changed original failure", e)
		}
		if _, e = os.Stat(endpointPath(dir)); !os.IsNotExist(e) {
			t.Fatal("verification started a service", e)
		}
		status, fault := Status(ctx, dir)
		if fault != nil || status.Status != "not_running" || status.LastShutdown == nil || status.LastShutdown.ResourcesSettled {
			t.Fatal("offline verification replaced original shutdown or started execution", status, fault)
		}
	}
	verify("active-rejected", false)
	port, err := orca.NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: r.Profile, ProfileID: id, AcceptedConfigVersion: digest, ProcessHost: probeProcessHost{cli: cli}, RepositoryPath: repository, RunDirectory: dir})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := port.ShutdownLifecycle(ctx, s, nil)
	if err == nil || facts.Stopped {
		t.Fatal("passive shutdown claimed active PTY stopped", facts, err)
	}
	proof, err := c.CloseTerminalChecked(ctx, p.Terminal, p.RuntimeID)
	if err != nil || proof.Disposition != "confirmed" {
		t.Fatal("native original PTY close unverified", proof, err)
	}
	// A close receipt precedes Orca's asynchronously published inventory. Keep
	// the production query fail-closed; wait for the actual original-host fact
	// here, without another close, alternate identity or input.
	for until := time.Now().Add(10 * time.Second); ; {
		err = c.VerifyTerminalStopped(ctx, p)
		if err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("native stop inventory did not settle", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	// This live process stands for the recorded native writer; its existence
	// must not be erased by the disappearance of the independent original PTY.
	if process.Probe(rootIdentity).State != "matches" {
		t.Fatal("recorded writer fixture is not live")
	}
	verify("closed-pty-native-unverified", false)
	facts, err = port.ShutdownLifecycle(ctx, s, nil)
	if err == nil || facts.Stopped || !strings.Contains(err.Error(), "PTY exit is insufficient") {
		t.Fatal("shutdown trusted PTY disappearance instead of native exit", facts, err)
	}
}

// The production process host executes only the adapter's read-only query.
// Its StartManaged/WriteExecutionFile paths must remain unreachable here.
type probeProcessHost struct{ cli string }

func (p probeProcessHost) StartManaged(*exec.Cmd) (b.StopHandle, error) {
	return nil, fmt.Errorf("probe forbids process startup")
}
func (p probeProcessHost) InspectProcess(*b.ProcessIdentity) string { return "unknown" }
func (p probeProcessHost) NewProcessGeneration() string             { return "probe" }
func (p probeProcessHost) WriteExecutionFile(string, []byte) error {
	return fmt.Errorf("probe forbids evidence writes")
}
func (p probeProcessHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	if c.Executable != p.cli || len(c.Args) < 2 || c.Args[0] != "terminal" || c.Args[1] != "list" {
		return nil, fmt.Errorf("probe forbids adapter effect: %v", c.Args)
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Executable, c.Args...)
	cmd.Dir = c.WorkingDirectory
	return cmd.Output()
}
