package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

type liveProbeHost struct{}

// Keep readiness and independent queries real while deliberately dropping
// every notification. No production simulation/profile option is introduced.
type recordingProbeFeed struct {
	native.StateFeed
	samples   *[]native.StateSample
	queryOnly bool
}

func (f recordingProbeFeed) Drain() ([]native.StateSample, error) {
	samples, err := f.StateFeed.Drain()
	*f.samples = append(*f.samples, samples...)
	if f.queryOnly {
		return nil, err
	}
	return samples, err
}

func (liveProbeHost) StartManaged(*exec.Cmd) (b.StopHandle, error) {
	return nil, errors.New("native probe owns an ordinary Orca terminal")
}
func (liveProbeHost) InspectProcess(*b.ProcessIdentity) string { return "unknown" }
func (liveProbeHost) NewProcessGeneration() string             { return "native-probe" }
func (liveProbeHost) WriteExecutionFile(path string, raw []byte) error {
	// Local Runner journal only; this does not provide workspace file transport.
	if filepath.Base(filepath.Dir(path)) == "orca-initial-terminals" {
		return process.New(time.Second).WriteExecutionFile(path, raw)
	}
	return errors.New("native probe has no workspace file channel")
}
func (liveProbeHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Executable, c.Args...)
	cmd.Dir = c.WorkingDirectory
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		blocked := false
		for _, b := range c.BlockedEnvironment {
			if strings.EqualFold(key, b) {
				blocked = true
			}
		}
		if !blocked {
			cmd.Env = append(cmd.Env, item)
		}
	}
	return cmd.Output()
}

func TestOrcaNativeStateLiveLifecycle(t *testing.T) {
	if mode := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE"); mode != "" {
		if mode != "long-input" && mode != "short-sequence" {
			t.Fatal("unknown native probe mode")
		}
		runNativeInputLiveProbe(t)
		return
	}
	root := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_ROOT")
	if root == "" {
		t.Skip("native lifecycle is opt-in through STEERRUNNER_ORCA_STATE_PROBE_ROOT")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	rawFixture, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	// This focused lifecycle probe needs only the fixed native target. The
	// broader fixture CLI validates the full business manifest separately.
	var fixture struct {
		Execution       contracts.Execution `json:"execution"`
		RepositoryPath  string              `json:"repository_path"`
		RemoteRuntimeID string              `json:"remote_runtime_id"`
	}
	if err != nil || json.Unmarshal(rawFixture, &fixture) != nil || contracts.ValidateExecution(fixture.Execution) != nil || fixture.Execution.Adapter != "orca-terminal" || !filepath.IsAbs(fixture.RepositoryPath) || fixture.RemoteRuntimeID == "" {
		t.Fatal("native lifecycle requires an independent Runner Orca fixture")
	}
	worktree, runtime, executable := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_WORKTREE"), os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_RUNTIME"), os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_CLI")
	environment := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_ENVIRONMENT")
	queryOnly := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_QUERY_ONLY") == "1"
	if worktree == "" || runtime == "" || executable == "" || environment != fixture.Execution.Target.Environment || runtime != fixture.RemoteRuntimeID {
		t.Fatal("missing fixed native probe target")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	repository := fixture.RepositoryPath
	host := liveProbeHost{}
	creator := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if environment != "" {
			args = append(append([]string(nil), args...), "--environment", environment)
		}
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: executable, Args: args, WorkingDirectory: repository, Timeout: timeout})
	}})
	// Match the production agent-launch readiness input: an untouched Codex TUI
	// has no completed native main-agent record until its first turn runs.
	launch, err := (&agentlaunch.Config{Executable: "codex"}).Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	created, err := creator.MetaCall(ctx, "terminal", "create", "--worktree", worktree, "--title", "SteerRunner production state probe", "--command", launch, "--json")
	var result struct{ Terminal native.Terminal }
	if err != nil || created.RuntimeID != runtime || json.Unmarshal(created.Payload, &result) != nil || result.Terminal.Handle == "" {
		t.Fatalf("native ordinary terminal creation unverified: %v", err)
	}
	terminal := result.Terminal
	record := map[string]any{"inputs": 0, "results": 0, "cleaned_up": false, "runtime": runtime, "environment": environment, "query_only": queryOnly}
	defer func() {
		data, _ := json.MarshalIndent(record, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "evidence", "native-production-lifecycle.json"), append(data, '\n'), 0600); err != nil {
			t.Error(err)
		}
	}()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		proof, err := creator.CloseTerminalChecked(cleanup, terminal.Handle, runtime)
		record["cleaned_up"] = err == nil && proof.Disposition == "confirmed"
		if err != nil || proof.Disposition != "confirmed" {
			t.Error("probe cleanup unverified", err)
		}
	}()
	terminal, err = creator.ShowTerminal(ctx, terminal.Handle)
	if err != nil {
		t.Fatal(err)
	}
	p := Profile{Executable: executable, Environment: environment, WaitTimeout: "40s"}
	process := b.ProcessIdentity{Kind: "orca_terminal", Executable: executable, Terminal: terminal.Handle, RuntimeID: runtime, Incarnation: terminal.IncarnationID, WorktreeID: terminal.WorktreeID, ExecutionHost: p.Target().ExecutionHost}
	startup, startupCancel := context.WithTimeout(ctx, 90*time.Second)
	defer startupCancel()
	startupFeed, err := native.OpenStateFeed(startup, environment, process)
	if err != nil {
		t.Fatal(err)
	}
	for {
		current, queryErr := startupFeed.Query(startup)
		if queryErr == nil && current.MainAgent != nil && current.MainAgent.State == "done" && current.ProviderSession.ID != "" && current.LastAssistantMessage == "READY" {
			break
		}
		select {
		case <-startup.Done():
			startupFeed.Close()
			t.Fatalf("native startup READY unavailable: %v; last=%+v", queryErr, current)
		case <-time.After(time.Second):
		}
	}
	startupFeed.Close()
	startupCancel()
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: p.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: strings.Repeat("a", 32), AttemptID: strings.Repeat("b", 32), External: process}, Session: b.SessionContext{ProjectID: "native-probe", ExecutionID: strings.Repeat("a", 32), AttemptID: strings.Repeat("b", 32), WorkflowRunID: strings.Repeat("a", 32), SessionID: SessionID(process)}, ConfigurationDigest: "native-probe", ResourceGeneration: "native-probe", Workspace: b.WorkspaceIntent{Path: terminal.WorktreePath, Branch: strings.TrimPrefix(terminal.Branch, "refs/heads/")}}
	raw, _ := json.Marshal(p)
	data, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: NativeRemoteProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: "native-probe", ProfileID: "native", AcceptedConfigVersion: "native"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	port, err := NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{data}, ProfileID: "native", AcceptedConfigVersion: "native", ProcessHost: host, RepositoryPath: repository})
	if err != nil {
		t.Fatal(err)
	}
	l := port.(*Lifecycle)
	defer l.closeStateFeed()
	var queries []native.AgentState
	var baselines []NativeProof
	var receipts []json.RawMessage
	var publicMessages []b.PublicMessage
	var samples []native.StateSample
	l.stateFactory = func(ctx context.Context, environment string, p b.ProcessIdentity) (native.StateFeed, error) {
		feed, err := native.OpenStateFeed(ctx, environment, p)
		if err != nil {
			return nil, err
		}
		return recordingProbeFeed{StateFeed: feed, samples: &samples, queryOnly: queryOnly}, nil
	}
	query := l.queryNative
	l.queryNative = func(ctx context.Context, p b.ProcessIdentity, pane string) (native.StateSample, error) {
		state, err := query(ctx, p, pane)
		if err == nil {
			queries = append(queries, state.AgentState)
		}
		return state, err
	}
	defer func() {
		record["queries"], record["baselines"], record["receipts"], record["public_messages"], record["samples"] = queries, baselines, receipts, publicMessages, samples
	}()
	s.Implementation, err = l.envelope(s, State{Process: process})
	if err != nil {
		t.Fatal(err)
	}
	var proofs []NativeProof
	for i, command := range []string{strings.Repeat("c", 32), strings.Repeat("d", 32)} {
		prompt := "Read README.md and summarize its purpose in one sentence. Do not change any files, do not create commits, do not publish anything, and do not run project checks. Treat this as a read-only description task and keep the answer to one sentence."
		if queryOnly {
			prompt = "Reply with the JSON object {\"outcome\":\"done\",\"summary\":\"ordinary completion probe\"}. Do not use tools or change files."
		}
		input := b.TurnInput{CommandID: command, Prompt: prompt}
		s.PendingTurn = &input
		req := request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input})
		req.Operation.ID += "-" + process.Terminal + "-" + command
		r, _, err := l.Execute(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		bind(&s, r)
		preparedState, err := l.state(s)
		if err != nil {
			t.Fatal(err)
		}
		baselines = append(baselines, turnFor(&preparedState, command).Native)
		req = request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})
		req.Operation.ID += "-" + process.Terminal + "-" + command
		r, obs, err := l.Execute(ctx, req)
		if err != nil || obs == nil {
			t.Fatalf("native original submission unverified: %v", err)
		}
		defer func() {
			obs.Close(context.Background())
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := obs.(*observer).awaitCleanup(cleanup); err != nil {
				t.Error(err)
			}
		}()
		bind(&s, r)
		receipts = append(receipts, append(json.RawMessage(nil), r.Operation.Receipt...))
		record["inputs"] = i + 1
		completed := false
		for !completed {
			select {
			case event, ok := <-obs.Events():
				if !ok {
					t.Fatal("native observation ended before complete reply")
				}
				if len(event.Implementation) > 0 {
					s.Implementation = event.Implementation
				}
				if event.PublicMessage != nil {
					if event.Fact.Completed || len(event.Implementation) > 0 || event.Observation != nil || event.PublicMessage.TurnID != command || event.PublicMessage.ObservedAt.IsZero() {
						t.Fatal("native message mixed facts or changed identity")
					}
					publicMessages = append(publicMessages, *event.PublicMessage)
				}
				completed = event.Fact.Kind == b.EventCompleted && event.Fact.Completed
				event.Persisted <- nil
				event.Processed <- nil
				if queryOnly && event.Observation != nil && event.Observation.Diagnostic == nativePromptMissing {
					if len(queries) == 0 || queries[len(queries)-1].Prompt != "" || queries[len(queries)-1].MainAgent == nil || queries[len(queries)-1].MainAgent.State != "done" {
						t.Fatal("missing-prompt diagnostic has no original query")
					}
					state, err := l.state(s)
					if err != nil || turnFor(&state, command).Native.EndedAt != 0 || turnFor(&state, command).Native.WorkingAt != 0 {
						t.Fatal("query-only observation invented start/end", err)
					}
					obs.Close(context.Background())
					if err := obs.(*observer).awaitCleanup(ctx); err != nil {
						t.Fatal(err)
					}
					record["empty_prompt_query_only_unverified"] = true
					return
				}
			case err := <-obs.Done():
				if err != nil {
					t.Fatal(err)
				}
				if !completed {
					t.Fatal("native original result unavailable")
				}
			case <-ctx.Done():
				obs.Close(context.Background())
				t.Fatal(ctx.Err())
			}
		}
		obs.Close(context.Background())
		if err := obs.(*observer).awaitCleanup(ctx); err != nil {
			t.Fatal(err)
		}
		r, _, err = l.ObserveLifecycle(ctx, s)
		if err != nil || r.TurnEvidence == nil || r.TurnEvidence.NativeOutcome != "unreported" {
			t.Fatalf("native completed result provenance unverified: %v", err)
		}
		bind(&s, r)
		state, err := l.state(s)
		if err != nil {
			t.Fatal(err)
		}
		proof := turnFor(&state, command).Native
		if queryOnly && (proof.WorkingAt != 0 || proof.EndedAt <= proof.BaseRootAt || len(queries) == 0) {
			t.Fatal("query-only probe invented working or accepted its old baseline", proof)
		}
		proofs = append(proofs, proof)
		record["results"] = i + 1
	}
	record["proofs"] = proofs
}
