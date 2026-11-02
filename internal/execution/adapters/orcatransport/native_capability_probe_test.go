package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

const capabilityProbeMode = "capability-baseline"

type capabilityTurn struct {
	Command, Request, Provider, Expected, Final string
	Accepted, Started, Completed, Original      bool
	Submits                                     uint64
}

type capabilityEvidence struct {
	Mode             string            `json:"mode"`
	Process          b.ProcessIdentity `json:"process"`
	Provider         string            `json:"provider"`
	Turns            []capabilityTurn  `json:"turns"`
	Before           native.Result     `json:"before_census"`
	After            native.Result     `json:"after_census"`
	HelperCalls      int               `json:"helper_calls"`
	CreatedTerminals int               `json:"created_terminals"`
	ManagedEvidence  bool              `json:"managed_evidence"`
	CloseCalls       int               `json:"close_calls"`
	CloseError       string            `json:"close_error"`
	CleanupError     string            `json:"cleanup_error"`
	Completed        bool              `json:"completed"`
	CleanedUp        bool              `json:"cleaned_up"`
	BusinessInputs   int               `json:"business_inputs"`
	SubmitCalls      uint64            `json:"submit_calls"`
	TextOnly         uint64            `json:"text_only_calls"`
	BareEnter        uint64            `json:"bare_enter_calls"`
	ScreenReads      uint64            `json:"screen_reads"`
	Failure          string            `json:"failure"`
	CreationUnknown  bool              `json:"creation_unknown"`
	CreationError    string            `json:"creation_error"`
	ManualCleanup    string            `json:"manual_cleanup"`
}

func validateCapabilityTarget(target nativeInputProbeTarget) error {
	if !filepath.IsAbs(target.CLI) || target.Runtime == "" || !strings.HasPrefix(target.Worktree, "id:") ||
		!strings.Contains(target.Worktree, "::") || target.Launch.Executable == "" || target.Launch.AccessMode != "full_access" ||
		target.Launch.Model == "" || target.Launch.ReasoningEffort == "" {
		return errors.New("capability probe requires an explicit runtime/worktree/CLI and full-access native model launch")
	}
	if target.Launch.RuntimeExecutable != "" || target.Launch.AuditModel != nil {
		return errors.New("capability probe cannot use managed helper or audit configuration")
	}
	_, err := target.Launch.Command("codex")
	return err
}

func (e capabilityEvidence) validate() error {
	if e.Mode != capabilityProbeMode || e.Process.RuntimeID == "" || e.Process.Terminal == "" ||
		e.Process.Incarnation == "" || e.Process.WorktreeID == "" || e.Provider == "" ||
		!localProbeEmptyCensus(e.Before, e.Process.RuntimeID) || !localProbeEmptyCensus(e.After, e.Process.RuntimeID) ||
		e.HelperCalls != 0 || e.CreatedTerminals != 1 || e.ManagedEvidence || e.CloseCalls < 0 || e.CloseCalls > 1 || !e.Completed || !e.CleanedUp ||
		e.CleanupError != "" || e.Failure != "" || e.CreationUnknown || e.CreationError != "" || e.ManualCleanup != "" || len(e.Turns) != 2 || e.BusinessInputs != 2 || e.SubmitCalls != 2 ||
		e.TextOnly != 0 || e.BareEnter != 0 || e.ScreenReads != 0 {
		return errors.New("native capability qualification is incomplete or uses the managed path")
	}
	seen := map[string]bool{}
	for _, turn := range e.Turns {
		if turn.Command == "" || turn.Request == "" || seen[turn.Request] || turn.Provider != e.Provider ||
			!turn.Accepted || !turn.Started || !turn.Completed || !turn.Original || turn.Submits != 1 ||
			turn.Expected == "" || turn.Final != turn.Expected {
			return errors.New("capability probe lacks two independently verified original turns")
		}
		seen[turn.Request] = true
	}
	return nil
}

func auditCapabilityCalls(root, cli, launch string) (created, helpers int, err error) {
	files, err := filepath.Glob(filepath.Join(root, "actual-cli-*.json"))
	if err != nil || len(files) == 0 {
		return 0, 0, errors.New("capability probe has no actual command evidence")
	}
	for _, file := range files {
		raw, e := os.ReadFile(file)
		var value struct{ Command b.LaunchCommand }
		if e != nil || json.Unmarshal(raw, &value) != nil || !strings.EqualFold(value.Command.Executable, cli) || len(value.Command.Args) < 2 {
			return created, helpers + 1, errors.New("capability probe invoked an unexpected executable or lost command evidence")
		}
		args := value.Command.Args
		if args[0] != "terminal" {
			return created, helpers + 1, errors.New("capability probe invoked a non-terminal command")
		}
		if args[1] == "create" {
			created++
			found := false
			for i, arg := range args {
				if arg == "--command" && i+1 < len(args) {
					found = true
					if args[i+1] != launch {
						return created, helpers + 1, errors.New("capability terminal launch differs from the fixed native command")
					}
				}
			}
			if !found {
				return created, helpers + 1, errors.New("capability terminal has no fixed native launch")
			}
		}
	}
	return created, helpers, nil
}

// The capability probe proves only terminal absence. The production local
// managed scope intentionally remains unchanged and is not invoked here.
func closeCapabilityTerminal(ctx context.Context, client native.Client, p b.ProcessIdentity) (int, string, error) {
	if err := client.VerifyTerminalStopped(ctx, p); err == nil {
		return 0, "", nil
	}
	t, err := client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, p)
	if err != nil || t.WorktreeID != p.WorktreeID {
		return 0, "", errors.Join(err, errors.New("capability cleanup original worktree is unverified"))
	}
	_, closeErr := client.CloseTerminalChecked(ctx, p.Terminal, p.RuntimeID)
	// Never repeat close, including a lost response. The complete original-host
	// inventory is the independent proof; a close receipt alone is insufficient.
	verifyErr := client.VerifyTerminalStopped(ctx, p)
	return 1, errorText(closeErr), verifyErr
}

func cleanupCapabilityProbe(ctx context.Context, client native.Client, target nativeInputProbeTarget, e *capabilityEvidence) error {
	var closeErr error
	if e.CreationUnknown {
		// A census is evidence for manual reconciliation, not authority to adopt
		// a terminal whose exact creation identity was never received.
		e.ManualCleanup = fmt.Sprintf("creation identity unknown; inspect target.json and terminal-create.json, then reconcile runtime=%q environment=%q worktree=%q with after_census; no automatic close or replacement", target.Runtime, target.Environment, target.Worktree)
		closeErr = errors.New("capability creation identity unknown; manual reconciliation required")
	} else {
		e.CloseCalls, e.CloseError, closeErr = closeCapabilityTerminal(ctx, client, e.Process)
	}
	var censusErr error
	e.After, censusErr = client.MetaCall(ctx, "terminal", "list", "--worktree", target.Worktree, "--json")
	err := errors.Join(closeErr, censusErr)
	e.CleanupError = errorText(err)
	e.CleanedUp = err == nil && !e.CreationUnknown && localProbeEmptyCensus(e.After, target.Runtime)
	if !e.CleanedUp {
		return errors.Join(err, errors.New("capability original terminal cleanup remains unverified"))
	}
	return nil
}

func createCapabilityTerminal(ctx context.Context, client native.Client, target nativeInputProbeTarget, launch string, e *capabilityEvidence) (native.Result, error) {
	e.CreationUnknown = true
	created, err := client.MetaCall(ctx, "terminal", "create", "--worktree", target.Worktree, "--title", "SteerRunner B0 native capability", "--command", launch, "--json")
	var made struct{ Terminal native.Terminal }
	if err != nil || created.CallUncertain || created.RuntimeID != target.Runtime || json.Unmarshal(created.Payload, &made) != nil ||
		made.Terminal.Handle == "" || made.Terminal.IncarnationID == "" || made.Terminal.WorktreeID != strings.TrimPrefix(target.Worktree, "id:") {
		err = errors.Join(err, errors.New("capability creation unknown; no replacement or ambiguous cleanup"))
		e.CreationError = err.Error()
		return created, err
	}
	e.Process = b.ProcessIdentity{Kind: "orca_terminal", Executable: target.CLI, RuntimeID: target.Runtime, Terminal: made.Terminal.Handle,
		Incarnation: made.Terminal.IncarnationID, WorktreeID: made.Terminal.WorktreeID, ExecutionHost: (Profile{Environment: target.Environment}).Target().ExecutionHost}
	e.CreationUnknown = false
	return created, nil
}

func runNativeCapabilityProbe(t *testing.T) {
	root := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_ROOT")
	var target nativeInputProbeTarget
	raw, err := os.ReadFile(filepath.Join(root, "target.json"))
	if err != nil || b.DecodeStrictJSON(raw, &target, "trailing capability target") != nil || validateCapabilityTarget(target) != nil {
		t.Fatal("invalid native capability target", err)
	}
	attempt, err := os.OpenFile(filepath.Join(root, "probe-attempted"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal("capability probe already attempted; never replay", err)
	}
	_ = attempt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	host := &nativeInputProbeHost{ManagedProcessHost: process.New(40 * time.Second), root: root}
	evidence := capabilityEvidence{Mode: capabilityProbeMode}
	launch, err := target.Launch.Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		evidence.SubmitCalls, evidence.TextOnly, evidence.BareEnter, evidence.ScreenReads = host.businessSubmits.Load(), host.businessTextOnly.Load(), host.businessBareEnter.Load(), host.businessReads.Load()
		var auditErr error
		evidence.CreatedTerminals, evidence.HelperCalls, auditErr = auditCapabilityCalls(root, target.CLI, launch)
		if auditErr != nil {
			evidence.Failure = auditErr.Error()
		}
		if err := evidence.validate(); err != nil {
			evidence.Failure = errors.Join(errors.New(evidence.Failure), err).Error()
			t.Error("native capability qualification failed", err)
		}
		writeLocalProbe(t, host, root, "native-result.json", evidence)
	}()
	profile := Profile{Executable: target.CLI, Environment: target.Environment, WaitTimeout: "40s"}
	creator := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if target.Environment != "" {
			args = append(args, "--environment", target.Environment)
		}
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: target.CLI, Args: args, WorkingDirectory: root, Timeout: timeout,
			BlockedEnvironment: []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}})
	}})
	census := func(ctx context.Context) (native.Result, error) {
		return creator.MetaCall(ctx, "terminal", "list", "--worktree", target.Worktree, "--json")
	}
	evidence.Before, err = census(ctx)
	if err != nil || !localProbeEmptyCensus(evidence.Before, target.Runtime) {
		t.Fatal("capability probe requires an empty complete original worktree", err)
	}
	// Register before the mutation: response loss or an incomplete identity must
	// still produce the read-only census and manual reconciliation record.
	evidence.CreationUnknown = true
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 45*time.Second)
		defer done()
		if err := cleanupCapabilityProbe(cleanup, creator, target, &evidence); err != nil {
			t.Error(err)
		}
	}()
	created, err := createCapabilityTerminal(ctx, creator, target, launch, &evidence)
	writeLocalProbe(t, host, root, "terminal-create.json", created)
	if err != nil {
		t.Fatal(err)
	}
	p := evidence.Process
	terminal, err := creator.InspectBoundTerminal(ctx, profile.Target(), p)
	if err != nil {
		t.Fatal(err)
	}
	var ready native.AgentState
	for {
		raw, e := native.QueryState(ctx, target.Environment, target.Runtime, target.Worktree)
		if e == nil {
			ready, e = native.DecodeOriginalState(raw, orcaStateResource(p))
			if e == nil {
				ready, e = native.QueryOriginalAgent(ctx, target.Environment, orcaStateResource(p), ready.PaneKey)
			}
		}
		if e == nil && ready.MainAgent != nil && ready.MainAgent.State == "done" && ready.LastAssistantMessage == "READY" && ready.ProviderSession.ID != "" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("capability initialization unverified", e)
		case <-time.After(time.Second):
		}
	}
	evidence.Provider = ready.ProviderSession.ID
	writeLocalProbe(t, host, root, "ready.json", ready)
	run, attemptID := probeID(t), probeID(t)
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: run, AttemptID: attemptID, External: p},
		Session: b.SessionContext{ProjectID: "capability-probe", ExecutionID: run, AttemptID: attemptID, WorkflowRunID: run, SessionID: SessionID(p)}, ConfigurationDigest: "capability-probe", ResourceGeneration: probeID(t),
		Workspace: b.WorkspaceIntent{Path: terminal.WorktreePath, Branch: strings.TrimPrefix(terminal.Branch, "refs/heads/")}}
	version := NativeRemoteProfileVersion
	if target.Environment == "" {
		version = ProfileVersion
	}
	payload, _ := json.Marshal(profile)
	config, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: version, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: s.Session.ProjectID, ProfileID: "capability", AcceptedConfigVersion: "capability"}, payload)
	if err != nil {
		t.Fatal(err)
	}
	port, err := NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{config}, ProfileID: "capability", AcceptedConfigVersion: "capability", ProcessHost: host, RepositoryPath: root, RunDirectory: root})
	if err != nil {
		t.Fatal(err)
	}
	l := port.(*Lifecycle)
	defer l.closeStateFeed()
	s.Implementation, err = l.envelope(s, State{Process: p})
	if err != nil {
		t.Fatal(err)
	}
	a, bNonce := probeID(t), probeID(t)
	prompts := []string{"No tools, commands, file changes, commits or publication.\n中文原生能力核验。\n\nRemember " + a + ". Reply exactly SR_B0_FIRST_" + a + ".",
		"Do not use tools or change files. Reply exactly SR_B0_SECOND_" + bNonce + "_ followed immediately by the value you remembered in the previous turn."}
	expected := []string{"SR_B0_FIRST_" + a, "SR_B0_SECOND_" + bNonce + "_" + a}
	for index, prompt := range prompts {
		// This explicit real wait is test composition, not a production routing
		// change or substitute completion signal.
		if err = l.waitOriginalComposer(ctx, s); err != nil {
			t.Fatal(err)
		}
		_, budget, err := m.FreezeInput(m.InputPart{Source: "caller.direct", Owner: m.CallerInput, Text: prompt})
		if err != nil {
			t.Fatal(err)
		}
		input := b.TurnInput{CommandID: probeID(t), Prompt: prompt, Budget: budget, InputVersion: uint64(index + 1)}
		s.PendingTurn, s.Session.PendingCommandID = &input, input.CommandID
		prepare, err := operation.New(run, s, b.LifecycleCandidate{Action: b.LifecyclePrepareSession, Input: b.LifecycleInput{Turn: &input}}, probeID(t), "b0-prepare", time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		writeLocalProbe(t, host, root, fmt.Sprintf("turn-%d-prepare-intent.json", index+1), prepare)
		receipt, _, err := l.Execute(ctx, prepare.Request)
		if err != nil {
			t.Fatal("capability preparation failed", err)
		}
		bind(&s, receipt)
		intent, err := operation.New(run, s, b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: &input}}, probeID(t), "b0-submit", time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		host.businessTerminal, host.businessRequest = p.Terminal, nativeRequest(intent.Request.Operation.ID)
		before := host.businessSubmits.Load()
		writeLocalProbe(t, host, root, fmt.Sprintf("turn-%d-intent.json", index+1), intent)
		receipt, observerPort, err := l.Execute(ctx, intent.Request)
		writeLocalProbe(t, host, root, fmt.Sprintf("turn-%d-submit.json", index+1), receipt)
		if err != nil || observerPort == nil || !receipt.State.InputAccepted || receipt.State.TurnStarted {
			t.Fatal("capability input unknown; no resend", err)
		}
		turn := capabilityTurn{Command: input.CommandID, Request: receipt.RequestID, Expected: expected[index], Accepted: true}
		bind(&s, receipt)
		func() {
			defer observerPort.Close(context.Background())
			complete := false
			for !complete {
				select {
				case event, ok := <-observerPort.Events():
					if !ok {
						t.Fatal("capability observer closed before original completion")
					}
					if len(event.Implementation) != 0 {
						s.Implementation = event.Implementation
					}
					complete = event.Fact.Kind == b.EventCompleted && event.Fact.Completed
					event.Persisted <- nil
				case <-ctx.Done():
					t.Fatal("capability original completion deadline", ctx.Err())
				}
			}
			_ = observerPort.Close(ctx)
			if err := observerPort.(*observer).awaitCleanup(ctx); err != nil {
				t.Fatal(err)
			}
		}()
		receipt, _, err = l.ObserveLifecycle(ctx, s)
		bind(&s, receipt)
		state, stateErr := l.state(s)
		completedTurn := turnFor(&state, input.CommandID)
		if err != nil || stateErr != nil || completedTurn == nil || receipt.TurnEvidence == nil || !receipt.TurnEvidence.FinalVerified {
			t.Fatal("capability final missing", err, stateErr)
		}
		evidence.ManagedEvidence = evidence.ManagedEvidence || state.RemoteReadiness != nil || state.Control != nil || state.TerminalReadiness != nil ||
			completedTurn.Recorded != nil || completedTurn.Extension != nil || completedTurn.WireFormat != "" || receipt.TurnEvidence.FinalSource != NativeReplySource
		sample, err := native.QueryOriginalSample(ctx, target.Environment, orcaStateResource(p), ready.PaneKey)
		if err != nil {
			t.Fatal(err)
		}
		verified, err := queryOriginalInput(ctx, target.Environment, p, ready.PaneKey, completedTurn.Native.BaseRootAt, sample.AgentState)
		writeLocalProbe(t, host, root, fmt.Sprintf("turn-%d-original.json", index+1), verified)
		writeLocalProbe(t, host, root, fmt.Sprintf("turn-%d-state.json", index+1), state)
		turn.Provider, turn.Final, turn.Started, turn.Completed, turn.Submits = verified.ProviderSession.ID, receipt.TurnEvidence.LastMessage, completedTurn.Started, completedTurn.Settled, host.businessSubmits.Load()-before
		turn.Original = err == nil && shortOriginalInputVerified(verified, p, completedTurn)
		if verified.NativeInput != nil {
			writeLocalProbe(t, host, root, fmt.Sprintf("turn-%d-input-pages.json", index+1), verified.NativeInput.Pages)
		}
		evidence.Turns = append(evidence.Turns, turn)
		evidence.BusinessInputs++
		if !turn.Original || turn.Final != turn.Expected || turn.Provider != evidence.Provider || evidence.ManagedEvidence {
			t.Fatal("capability original input/final/unchanged provider is unverified", err)
		}
	}
	evidence.Completed = true
}

func capabilityCensus(runtime string) native.Result {
	raw, _ := json.Marshal(map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false,
		"hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}})
	return native.Result{RuntimeID: runtime, Payload: raw}
}

func TestOrcaCapabilityProbeTarget(t *testing.T) {
	for _, mode := range []string{"local", "remote", "helper", "audit", "missing-runtime", "non-id-worktree", "implicit-permissions", "unsafe-launch"} {
		t.Run(mode, func(t *testing.T) {
			target := nativeInputProbeTarget{CLI: filepath.Join(t.TempDir(), "orca.exe"), Worktree: "id:repo::C:/owned", Runtime: "runtime",
				Launch: agentlaunch.Config{Executable: "codex", Model: "model", ReasoningEffort: "high", AccessMode: "full_access"}}
			switch mode {
			case "remote":
				target.Environment = "fixed-remote"
			case "helper":
				target.Launch.RuntimeExecutable = "helper.exe"
			case "audit":
				target.Launch.AuditModel = &workflow.ModelSettings{Model: "audit"}
			case "missing-runtime":
				target.Runtime = ""
			case "non-id-worktree":
				target.Worktree = "active"
			case "implicit-permissions":
				target.Launch.AccessMode = ""
			case "unsafe-launch":
				target.Launch.Executable = "codex\nnew-command"
			}
			err := validateCapabilityTarget(target)
			if (err == nil) != (mode == "local" || mode == "remote") {
				t.Fatal(mode, err)
			}
		})
	}
}

func TestOrcaCapabilityProbeEvidence(t *testing.T) {
	for _, fault := range []string{"valid", "old-mode", "one-turn", "duplicate-request", "provider", "missing-final", "double-send", "managed", "no-command-evidence", "cleanup", "wrong-runtime", "screen-read"} {
		t.Run(fault, func(t *testing.T) {
			e := capabilityEvidence{Mode: capabilityProbeMode, Process: b.ProcessIdentity{RuntimeID: "runtime", Terminal: "terminal", Incarnation: "incarnation", WorktreeID: "worktree"},
				Provider: "provider", Before: capabilityCensus("runtime"), After: capabilityCensus("runtime"), CreatedTerminals: 1,
				CloseCalls: 1, Completed: true, CleanedUp: true, BusinessInputs: 2, SubmitCalls: 2}
			for i := 0; i < 2; i++ {
				e.Turns = append(e.Turns, capabilityTurn{Command: fmt.Sprint(i), Request: fmt.Sprint(i), Provider: "provider", Expected: "final", Final: "final",
					Accepted: true, Started: true, Completed: true, Original: true, Submits: 1})
			}
			switch fault {
			case "old-mode":
				e.Mode = "long-input"
			case "one-turn":
				e.Turns = e.Turns[:1]
			case "duplicate-request":
				e.Turns[1].Request = e.Turns[0].Request
			case "provider":
				e.Turns[1].Provider = "another"
			case "missing-final":
				e.Turns[1].Final = ""
			case "double-send":
				e.Turns[1].Submits = 2
			case "managed":
				e.ManagedEvidence = true
			case "no-command-evidence":
				e.CreatedTerminals = 0
			case "cleanup":
				e.CleanedUp = false
			case "wrong-runtime":
				e.After.RuntimeID = "another"
			case "screen-read":
				e.ScreenReads = 1
			}
			if (e.validate() == nil) != (fault == "valid") {
				t.Fatal("invalid native capability evidence admitted", fault)
			}
			if fault == "valid" {
				raw, err := json.Marshal(e)
				var fields map[string]json.RawMessage
				if err != nil || json.Unmarshal(raw, &fields) != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"failure", "cleanup_error", "close_error"} {
					if _, present := fields[key]; !present {
						t.Fatal("strict PowerShell consumer requires an explicit empty diagnostic", key)
					}
				}
			}
		})
	}
}

func TestOrcaCapabilityProbeCleanup(t *testing.T) {
	for _, scenario := range []string{"closed", "lost-response", "active", "replacement", "truncated", "missing-identity"} {
		t.Run(scenario, func(t *testing.T) {
			p := b.ProcessIdentity{RuntimeID: "runtime", Terminal: "terminal", Incarnation: "incarnation", WorktreeID: "worktree"}
			closed, closeCalls := false, 0
			client := native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				row := map[string]any{"handle": "terminal", "incarnationId": "incarnation", "worktreeId": "worktree", "status": "running"}
				if scenario == "replacement" {
					row["incarnationId"] = "replacement"
				}
				var result any
				switch args[0] + " " + args[1] {
				case "terminal list":
					rows := []any{}
					if !closed {
						rows = append(rows, row)
					}
					result = map[string]any{"terminals": rows, "totalCount": len(rows), "truncated": scenario == "truncated",
						"hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}
				case "terminal show":
					result = map[string]any{"terminal": row}
				case "terminal close":
					closeCalls++
					closed = scenario != "active"
					if scenario == "lost-response" {
						return nil, errors.New("lost close response")
					}
					result = map[string]any{"close": map[string]any{"handle": "terminal", "ptyKilled": true}}
				default:
					t.Fatal("cleanup created or sent input", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": "runtime"}})
			}})
			if scenario == "missing-identity" {
				p.RuntimeID = ""
			}
			calls, closeError, err := closeCapabilityTerminal(context.Background(), client, p)
			want := scenario == "closed" || scenario == "lost-response"
			if (err == nil) != want || closeCalls > 1 || calls != closeCalls || scenario == "lost-response" && closeError == "" {
				t.Fatal("wrong terminal cleanup qualification", calls, closeCalls, closeError, err)
			}
		})
	}
}

func TestOrcaCapabilityProbeUnknownCreation(t *testing.T) {
	for _, scenario := range []string{"response-loss-empty", "response-loss-ambiguous", "missing-identity", "census-error"} {
		t.Run(scenario, func(t *testing.T) {
			var calls []string
			target := nativeInputProbeTarget{Runtime: "original-runtime", Environment: "fixed-remote", Worktree: "id:repo::C:/owned"}
			client := native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				calls = append(calls, args[0]+" "+args[1])
				if len(calls) == 1 && calls[0] == "terminal create" {
					if scenario == "missing-identity" {
						return []byte(`{"ok":true,"result":{"terminal":{"handle":"unclaimed"}},"_meta":{"runtimeId":"original-runtime"}}`), nil
					}
					return nil, errors.New("creation response lost")
				}
				if strings.Join(args, " ") != "terminal list --worktree "+target.Worktree+" --json" {
					t.Fatal("unknown creation must only query the original worktree", args)
				}
				if scenario == "census-error" {
					return nil, errors.New("inventory unavailable")
				}
				result := capabilityCensus(target.Runtime)
				if scenario == "response-loss-ambiguous" {
					result.Payload = json.RawMessage(`{"terminals":[{"handle":"unclaimed","incarnationId":"new","worktreeId":"repo::C:/owned"}],"totalCount":1,"truncated":false,"hostScope":{"hostIds":["local"],"omittedHostIds":[]}}`)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result.Payload, "_meta": map[string]string{"runtimeId": target.Runtime}})
			}})
			e := capabilityEvidence{}
			if _, err := createCapabilityTerminal(context.Background(), client, target, "fixed native launch", &e); err == nil {
				t.Fatal("unknown creation accepted")
			}
			err := cleanupCapabilityProbe(context.Background(), client, target, &e)
			if err == nil || strings.Join(calls, ",") != "terminal create,terminal list" || e.CloseCalls != 0 || e.CleanedUp || e.CleanupError == "" || e.CreationError == "" || !e.CreationUnknown ||
				!strings.Contains(e.ManualCleanup, target.Runtime) || !strings.Contains(e.ManualCleanup, target.Environment) || !strings.Contains(e.ManualCleanup, target.Worktree) {
				t.Fatal("unknown creation lost reconciliation facts or acquired cleanup authority", e, err)
			}
			if scenario != "census-error" && (e.After.RuntimeID != target.Runtime || len(e.After.Payload) == 0) {
				t.Fatal("original census not retained", e.After)
			}
		})
	}
}
