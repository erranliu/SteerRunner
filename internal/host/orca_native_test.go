package host

import (
	"bytes"
	"context"
	"encoding/base64"
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
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
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
	// Evidence directories in different packages can share a basename. The
	// original runtime forbids reusing a prior workspace's native identity.
	made, err := c.MetaCall(ctx, "worktree", "create", "--repo", "id:"+repo, "--name", "sr-stop-"+filepath.Base(root)+"-"+contracts.NewID()[:8], "--base-branch", base, "--no-parent", "--setup", "skip", "--json")
	var creation struct{ Worktree native.Worktree }
	if err != nil || json.Unmarshal(made.Payload, &creation) != nil || creation.Worktree.ID == "" {
		t.Fatal("native worktree creation", err)
	}
	w := creation.Worktree
	cleaned := false
	t.Cleanup(func() {
		defer func() {
			raw, _ := json.Marshal(map[string]any{"passed": !t.Failed(), "resources_cleaned": cleaned, "runtime_id": made.RuntimeID, "worktree_id": w.ID, "environment": environment, "business_inputs": 0, "real_codex": os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_HELPER") != "", "fixture": "isolated current Run bound to real original native identities"})
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
	helper := os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_HELPER")
	var config codex.RemoteAgentConfig
	var launchCommand string
	args := []string{"terminal", "create", "--worktree", "id:" + w.ID, "--title", "SteerRunner stop proof probe", "--command", "powershell.exe -NoProfile"}
	if helper != "" {
		config = codex.RemoteAgentConfig{Executable: os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_NATIVE"), RepositoryURL: "https://github.com/erranliu/SteerlineTest.git", AccessMode: "sandbox", Model: os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_MODEL")}
		if config.Executable == "" || len(os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_HELPER_SHA256")) != 64 {
			t.Fatal("real native helper identity missing")
		}
		raw, _ := json.Marshal(config)
		launchCommand = "& '" + strings.ReplaceAll(helper, "'", "''") + "' '--configuration' '" + base64.RawURLEncoding.EncodeToString(raw) + "'"
	}
	args = append(args, "--json")
	created, err := c.MetaCall(ctx, args...)
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
	if helper != "" {
		profile.RuntimeExecutable, profile.RuntimeSHA256, profile.RuntimeConfigurationDigest = helper, os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_HELPER_SHA256"), codex.RemoteConfigurationDigest(config)
	}
	id, attempt, digest := contracts.NewID(), contracts.NewID(), strings.Repeat("a", 64)
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: id, AttemptID: attempt, External: p}, Session: b.SessionContext{ProjectID: "stop-probe", ExecutionID: id, WorkflowRunID: id, AttemptID: attempt}, ConfigurationDigest: digest, ResourceGeneration: "probe-generation"}
	s.Workspace = b.WorkspaceIntent{Path: w.Path, Branch: strings.TrimPrefix(w.Branch, "refs/heads/"), BaseCommit: base}
	profileRaw, _ := json.Marshal(profile)
	profileData, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.ProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.ProfileKey, ProjectID: s.Session.ProjectID, ProfileID: id, AcceptedConfigVersion: digest}, profileRaw)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "state")
	state := orca.State{Process: p}
	freezeState := func() {
		raw, _ := json.Marshal(state)
		e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.StateVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.StateKey, ProjectID: s.Session.ProjectID, ExecutionID: id, AttemptID: attempt, Generation: s.ResourceGeneration, ConfigDigest: digest}, raw)
		if err != nil {
			t.Fatal(err)
		}
		s.Implementation = []b.ImplementationDataEnvelope{e}
	}
	freezeState()
	if helper != "" {
		// Exercise the same preparation gate before starting the native helper.
		owner, e := orca.NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{profileData}, ProfileID: id, AcceptedConfigVersion: digest, ProcessHost: process.New(30 * time.Second), RepositoryPath: repository, RepositoryURL: config.RepositoryURL, RunDirectory: dir, AgentLaunchCommand: launchCommand})
		if e != nil {
			t.Fatal(e)
		}
		facts, e := owner.PreparationFacts(ctx, s)
		if e != nil || facts.ResourceRequired == nil || facts.ResourceRequired.Purpose != "native-control" {
			t.Fatal("original control preparation unavailable", facts, e)
		}
		ref := s.Ref
		operation := b.Operation{ID: contracts.NewID(), Action: string(b.LifecycleCreateResource), PayloadDigest: digest, Ref: &ref, LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: s.Session.ProjectID, ExecutionID: id, ConfigurationDigest: digest, ResourceGeneration: s.ResourceGeneration, InputBindingDigest: b.LifecycleBindingDigest(s), Ref: ref}}
		q := b.LifecycleRequest{Snapshot: s, Input: b.LifecycleInput{Resource: facts.ResourceRequired}, Operation: operation}
		receipt, _, e := owner.Execute(ctx, q)
		for e == nil && receipt.Disposition == b.OperationAccepted {
			s.Implementation = receipt.Implementation
			q.Snapshot = s
			time.Sleep(200 * time.Millisecond)
			receipt, _, e = owner.Reconcile(ctx, q)
		}
		if e != nil || receipt.Disposition != b.OperationConfirmed {
			t.Fatal("original control preparation failed", receipt, e)
		}
		s.Implementation = receipt.Implementation
		for _, v := range s.Implementation {
			if v.ImplementationKey == orca.StateKey {
				if e = json.Unmarshal(v.Payload, &state); e != nil {
					t.Fatal(e)
				}
			}
		}
		// This is a first submission, not a retry of a native UUID. An uncertain
		// response fails this owned probe rather than resending the startup.
		_, e = c.MetaCall(ctx, "terminal", "send", "--terminal", p.Terminal, "--text", launchCommand, "--enter", "--json")
		if e != nil {
			t.Fatal("original helper startup", e)
		}
	}
	var ready *codex.RemoteReadiness
	if helper != "" {
		profile.RuntimeExecutable, profile.RuntimeSHA256, profile.RuntimeConfigurationDigest = helper, os.Getenv("STEERRUNNER_ORCA_STOP_PROBE_HELPER_SHA256"), codex.RemoteConfigurationDigest(config)
		for until := time.Now().Add(150 * time.Second); ready == nil; {
			read, e := c.MetaCall(ctx, "terminal", "read", "--terminal", p.Terminal, "--limit", "1000", "--json")
			var value struct{ Terminal struct{ Tail []string } }
			if e != nil || json.Unmarshal(read.Payload, &value) != nil {
				t.Fatal("real native readiness read", e)
			}
			for _, line := range value.Terminal.Tail {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, codex.RemoteReadyPrefix) {
					raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteReadyPrefix))
					var r codex.RemoteReadiness
					if e != nil || json.Unmarshal(raw, &r) != nil || r.RuntimeSHA256 != profile.RuntimeSHA256 || r.ConfigurationDigest != profile.RuntimeConfigurationDigest || !strings.EqualFold(strings.ReplaceAll(r.Workspace, "\\", "/"), strings.ReplaceAll(w.Path, "\\", "/")) || !r.Verified || len(r.ExitPublicKey) != 32 {
						t.Fatal("real native readiness binding invalid", e)
					}
					ready = &r
					_ = os.WriteFile(filepath.Join(root, "native-readiness.json"), raw, 0600)
				}
				if strings.Contains(line, "preparation/execution failed:") {
					t.Fatal("real native preparation failed", line)
				}
			}
			if time.Now().After(until) {
				t.Fatal("real Codex readiness timed out")
			}
			if ready == nil {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
	rootIdentity, err := process.CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if ready == nil {
		ready = &codex.RemoteReadiness{Process: *rootIdentity, SessionID: "probe-live-root"}
	}
	state.RemoteReadiness = ready
	// Native reader settlement is covered through the fixed protocol in the
	// workflow tests; this zero-business probe isolates offline signed exit.
	stateRaw, _ := json.Marshal(state)
	stateData, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: orca.StateVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.StateKey, ProjectID: s.Session.ProjectID, ExecutionID: id, AttemptID: attempt, Generation: s.ResourceGeneration, ConfigDigest: digest}, stateRaw)
	if err != nil {
		t.Fatal(err)
	}
	s.Implementation = []b.ImplementationDataEnvelope{stateData}
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
	r := &runstate.Record{ID: id, InputDigest: digest, Revision: 1, Snapshot: s, Profile: []b.ImplementationDataEnvelope{profileData}, RepositoryPath: repository, Request: contracts.RunRequest{Repository: contracts.Repository{URL: config.RepositoryURL}, Execution: contracts.Execution{Adapter: "orca-terminal"}}, Status: "blocked", Phase: "probe", CommandStates: map[string]string{}}
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
	port, err := orca.NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: r.Profile, ProfileID: id, AcceptedConfigVersion: digest, ProcessHost: probeProcessHost{cli: cli}, RepositoryPath: repository, RepositoryURL: config.RepositoryURL, RunDirectory: dir})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := port.ShutdownLifecycle(ctx, s, nil)
	if err == nil || facts.Stopped {
		t.Fatal("passive shutdown claimed active PTY stopped", facts, err)
	}
	if helper != "" {
		stopping, e := orca.NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: r.Profile, ProfileID: id, AcceptedConfigVersion: digest, ProcessHost: probeProcessHost{cli: cli, allowStop: true, runDirectory: dir}, RepositoryPath: repository, RepositoryURL: config.RepositoryURL, RunDirectory: dir})
		if e != nil {
			t.Fatal(e)
		}
		ref := s.Ref
		stop := b.LifecycleRequest{Snapshot: s, Input: b.LifecycleInput{Stop: &b.StopInput{}}, Operation: b.Operation{ID: contracts.NewID(), Action: string(b.LifecycleStopExecution), PayloadDigest: digest, Ref: &ref, LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: s.Session.ProjectID, ExecutionID: id, ConfigurationDigest: digest, ResourceGeneration: s.ResourceGeneration, InputBindingDigest: b.LifecycleBindingDigest(s), Ref: ref}}}
		receipt, e := stopping.StopLifecycle(ctx, stop, nil)
		raw, _ := json.Marshal(receipt)
		_ = os.WriteFile(filepath.Join(root, "adapter-stop.json"), raw, 0600)
		if e != nil || receipt.Disposition != b.OperationConfirmed || !receipt.State.Stopped || !receipt.State.WorkerAbsent {
			t.Fatal("real adapter stop failed", receipt.Diagnostic, e)
		}
		path, _ := codex.RemoteExitPath(*ready)
		raw, e = native.ReadStopArtifact(ctx, environment, p.RuntimeID, w.ID, path)
		if e != nil || codex.VerifyRemoteExit(raw, *ready) != nil {
			t.Fatal("real native exit unavailable", e)
		}
		_ = os.WriteFile(filepath.Join(root, "native-exit.json"), raw, 0600)
	} else {
		proof, e := c.CloseTerminalChecked(ctx, p.Terminal, p.RuntimeID)
		if e != nil || proof.Disposition != "confirmed" {
			t.Fatal("native original PTY close unverified", proof, e)
		}
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
	if helper != "" {
		verify("stopped-verified", true)
		facts, err = port.ShutdownLifecycle(ctx, s, nil)
		if err != nil || !facts.Stopped || !facts.WorkerAbsent {
			t.Fatal("real shutdown proof rejected", facts, err)
		}
		return
	}
	verify("closed-pty-native-unverified", false)
	facts, err = port.ShutdownLifecycle(ctx, s, nil)
	if err == nil || facts.Stopped || !strings.Contains(err.Error(), "PTY exit is insufficient") {
		t.Fatal("shutdown trusted PTY disappearance instead of native exit", facts, err)
	}
}

// The production process host executes only the adapter's read-only query.
// Its StartManaged/WriteExecutionFile paths must remain unreachable here.
type probeProcessHost struct {
	cli          string
	allowStop    bool
	runDirectory string
}

func (p probeProcessHost) StartManaged(*exec.Cmd) (b.StopHandle, error) {
	return nil, fmt.Errorf("probe forbids process startup")
}
func (p probeProcessHost) InspectProcess(*b.ProcessIdentity) string { return "unknown" }
func (p probeProcessHost) NewProcessGeneration() string             { return "probe" }
func (p probeProcessHost) WriteExecutionFile(path string, raw []byte) error {
	root := filepath.Join(p.runDirectory, "orca-native-inspections") + string(filepath.Separator)
	if !p.allowStop || !filepath.IsAbs(p.runDirectory) || !strings.HasPrefix(strings.ToLower(filepath.Clean(path)), strings.ToLower(root)) {
		return fmt.Errorf("probe forbids evidence writes")
	}
	return process.New(30*time.Second).WriteExecutionFile(path, raw)
}
func (p probeProcessHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	if c.Executable != p.cli || len(c.Args) < 2 || c.Args[0] != "terminal" || c.Args[1] != "list" && !(p.allowStop && (c.Args[1] == "show" || c.Args[1] == "close" || c.Args[1] == "read" || c.Args[1] == "send")) {
		return nil, fmt.Errorf("probe forbids adapter effect: %v", c.Args)
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Executable, c.Args...)
	cmd.Dir = c.WorkingDirectory
	return cmd.Output()
}
