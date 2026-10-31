package orcatransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
)

type nativeInputProbeTarget struct {
	Worktree, Runtime, Environment, CLI string
	Launch                              agentlaunch.Config
}

// The existing opt-in lifecycle entry owns one fresh terminal and never replays
// a prior probe. The host records real calls without replacing responses.
func runNativeInputLiveProbe(t *testing.T) {
	root := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_ROOT")
	if root == "" {
		t.Fatal("explicit mode requires a fresh probe root")
	}
	var target nativeInputProbeTarget
	raw, e := os.ReadFile(filepath.Join(root, "target.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = contracts.DecodeStrict(raw, &target); e != nil {
		t.Fatal(e)
	}
	if target.Worktree == "" || target.Runtime == "" || target.CLI == "" || target.Launch.Executable == "" || target.Launch.Model == "" {
		t.Fatal("incomplete fixed native probe target")
	}
	owner, e := os.OpenFile(filepath.Join(root, "probe-attempted"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		t.Fatal("probe root already attempted; never replay", e)
	}
	owner.Close()
	if target.Environment == "" {
		runLocalNativeInputProbe(t, root, target)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	baseHost := process.New(40 * time.Second)
	host := &nativeInputProbeHost{ManagedProcessHost: baseHost, root: root}
	profile := Profile{Executable: target.CLI, Environment: target.Environment, WaitTimeout: "40s"}
	creator := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		args = append(args, "--environment", target.Environment)
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: target.CLI, Args: args, WorkingDirectory: root, Timeout: timeout})
	}})
	record := map[string]any{"business_inputs": 0, "completed": false, "cleaned_up": false, "launch": target.Launch, "mode": os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE")}
	defer func() {
		record["text_calls"], record["enter_flag_calls"] = host.pastes.Load(), host.enters.Load()
		record["submit_calls"], record["text_only_calls"], record["bare_enter_calls"], record["screen_reads"] = host.submits.Load(), host.textOnly.Load(), host.bareEnter.Load(), host.reads.Load()
		record["business_request_mismatches"] = host.businessForeignRequest.Load()
		raw, _ := json.MarshalIndent(record, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "native-result.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	}()
	census := func(ctx context.Context, name string) bool {
		result, err := creator.MetaCall(ctx, "terminal", "list", "--worktree", target.Worktree, "--json")
		record[name] = result
		var v struct {
			TotalCount *int
			Truncated  *bool
			HostScope  *struct{ HostIDs, OmittedHostIDs *[]string }
			Terminals  *[]json.RawMessage
		}
		return err == nil && result.RuntimeID == target.Runtime && !result.CallUncertain && result.ErrorCode == "" && json.Unmarshal(result.Payload, &v) == nil && v.TotalCount != nil && *v.TotalCount == 0 && v.Truncated != nil && !*v.Truncated && v.HostScope != nil && v.HostScope.HostIDs != nil && len(*v.HostScope.HostIDs) > 0 && v.HostScope.OmittedHostIDs != nil && len(*v.HostScope.OmittedHostIDs) == 0 && v.Terminals != nil && len(*v.Terminals) == 0
	}
	if !census(ctx, "before_census") {
		t.Fatal("probe requires an empty, completely listed independent worktree")
	}
	launch, e := target.Launch.Command("codex")
	if e != nil {
		t.Fatal(e)
	}
	initializationStarted := time.Now().UTC()
	record["initialization_started_at"] = initializationStarted
	created, e := creator.MetaCall(ctx, "terminal", "create", "--worktree", target.Worktree, "--title", "SteerRunner isolated native input probe", "--command", launch, "--json")
	var value struct{ Terminal native.Terminal }
	if e != nil || created.CallUncertain || created.ErrorCode != "" || created.RuntimeID != target.Runtime || json.Unmarshal(created.Payload, &value) != nil || value.Terminal.Handle == "" || value.Terminal.IncarnationID == "" || value.Terminal.WorktreeID != strings.TrimPrefix(target.Worktree, "id:") {
		t.Fatal("new probe creation unverified", e)
	}
	terminal := value.Terminal
	identity := b.ProcessIdentity{Kind: "orca_terminal", Executable: target.CLI, Terminal: terminal.Handle, RuntimeID: target.Runtime, Incarnation: terminal.IncarnationID, WorktreeID: terminal.WorktreeID, ExecutionHost: profile.Target().ExecutionHost}
	record["terminal"] = terminal
	formalStopAttempted, formalStopVerified := false, false
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if formalStopAttempted {
			record["cleaned_up"] = formalStopVerified && census(cleanup, "after_census")
			if record["cleaned_up"] != true {
				t.Error("original formal remote stop/census unverified; no close repeated")
			}
			return
		}
		inspected, e := creator.InspectBoundTerminal(cleanup, profile.Target(), identity)
		if e == nil && inspected.WorktreeID != identity.WorktreeID {
			e = fmt.Errorf("original emergency cleanup worktree changed")
		}
		var proof b.StopReceipt
		if e == nil {
			proof, e = creator.CloseTerminalChecked(cleanup, identity.Terminal, identity.RuntimeID)
		}
		record["cleanup"] = proof
		record["emergency_cleanup"] = true
		record["cleaned_up"] = e == nil && proof.Disposition == "confirmed" && census(cleanup, "after_census")
		if record["cleaned_up"] != true {
			t.Error("owned terminal cleanup/census unverified", e)
		}
	}()
	shown, e := creator.ShowTerminal(ctx, terminal.Handle)
	if e != nil || shown.IncarnationID != identity.Incarnation || shown.WorktreeID != identity.WorktreeID {
		t.Fatal("original created terminal changed before readiness", e)
	}
	terminal = shown
	var ready native.AgentState
	// Preparation consumes the same bounded fixture deadline as the local
	// adapter; slow original READY requests are only queried, never resent.
	startup := ctx
	for {
		raw, e := native.QueryState(startup, target.Environment, target.Runtime, "id:"+identity.WorktreeID)
		var state native.AgentState
		if e == nil {
			state, e = native.DecodeOriginalState(raw, identity)
		}
		if e == nil {
			sample, queryErr := native.QueryOriginalSample(startup, target.Environment, identity, state.PaneKey)
			e = queryErr
			state = sample.AgentState
		}
		record["last_startup_state"], record["last_startup_error"] = state, errorText(e)
		if e == nil && state.MainAgent != nil && state.MainAgent.State == "done" && state.LastAssistantMessage == "READY" {
			ready = state
			break
		}
		select {
		case <-startup.Done():
			// Failure-only diagnostics do not authorize input or repair readiness.
			diagnostic, done := context.WithTimeout(context.Background(), 5*time.Second)
			result, readErr := creator.MetaCall(diagnostic, "terminal", "read", "--terminal", identity.Terminal, "--cursor", "0", "--limit", "1000", "--json")
			done()
			record["startup_failure_stream"], record["startup_failure_stream_error"] = result, errorText(readErr)
			t.Fatal("initial READY not verified", e)
		case <-time.After(time.Second):
		}
	}
	record["ready"] = ready
	record["ready_at"], record["initialization_elapsed_ms"] = time.Now().UTC(), time.Since(initializationStarted).Milliseconds()
	runID, attemptID := probeID(t), probeID(t)
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: runID, AttemptID: attemptID, External: identity}, Session: b.SessionContext{ProjectID: "native-input-probe", ExecutionID: runID, AttemptID: attemptID, WorkflowRunID: runID, SessionID: SessionID(identity)}, ConfigurationDigest: "native-input-probe", ResourceGeneration: probeID(t), Workspace: b.WorkspaceIntent{Path: terminal.WorktreePath, Branch: strings.TrimPrefix(terminal.Branch, "refs/heads/")}}
	record["run_id"], record["attempt_id"], record["process"] = runID, attemptID, identity
	raw, _ = json.Marshal(profile)
	data, e := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: NativeRemoteProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: s.Session.ProjectID, ProfileID: "native", AcceptedConfigVersion: "native"}, raw)
	if e != nil {
		t.Fatal(e)
	}
	port, e := NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{data}, ProfileID: "native", AcceptedConfigVersion: "native", ProcessHost: host, RepositoryPath: root, RunDirectory: filepath.Join(root, "owned-journal")})
	if e != nil {
		t.Fatal(e)
	}
	l := port.(*Lifecycle)
	defer l.closeStateFeed()
	s.Implementation, e = l.envelope(s, State{Process: identity})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		formalStopAttempted = true
		intent, err := operation.New(runID, s, b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{}}}, probeID(t), "probe-stop", time.Now().UTC())
		if err != nil {
			t.Error(err)
			return
		}
		writeLocalProbe(t, host, root, "stop-intent.json", intent)
		receipt, stopErr := l.StopLifecycle(cleanup, intent.Request, nil)
		record["stop_receipt"], record["stop_error"], record["stop_scope"] = receipt, errorText(stopErr), l.StopEvidenceScope()
		writeLocalProbe(t, host, root, "stop-receipt.json", receipt)
		if stopErr == nil && receipt.Disposition == b.OperationConfirmed && receipt.State.Stopped && receipt.State.WorkerAbsent {
			verifyErr := l.verifyStopped(cleanup, s)
			record["verify_stopped_error"] = errorText(verifyErr)
			formalStopVerified = verifyErr == nil
		}
		if !formalStopVerified {
			t.Error("formal original remote stop unverified", stopErr)
		}
	}()
	if os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE") == "short-sequence" {
		runShortInputSequence(t, ctx, l, s, creator, identity, ready, record, root)
		return
	}
	nonce := make([]byte, 16)
	if _, e := rand.Read(nonce); e != nil {
		t.Fatal(e)
	}
	marker := "SR_NATIVE_INPUT_" + hex.EncodeToString(nonce)
	text := "No-tools native input qualification. Do not use tools, read or change files, run commands, create commits, deploy or publish. Reply exactly with " + marker + ".\r\n\r\n" + strings.Repeat("中文 original line; preserve blank lines.\r\n\r\n", 100)
	prompt, budget, e := model.FreezeInput(model.InputPart{Source: "caller.direct", Owner: model.CallerInput, Text: text})
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "original-input.txt"), []byte(prompt), 0600); e != nil {
		t.Fatal(e)
	}
	record["prompt_sha256"] = digest([]byte(prompt))
	record["logical_bytes"] = len(prompt)
	input := b.TurnInput{CommandID: hex.EncodeToString(nonce), Prompt: prompt, Budget: budget, InputVersion: 1}
	writeLocalProbe(t, host, root, "original-input.json", input)
	writeLocalProbe(t, host, root, "wire-input.txt", nativePromptText(prompt))
	s.PendingTurn = &input
	s.Session.PendingCommandID = input.CommandID
	prepared, e := operation.New(runID, s, b.LifecycleCandidate{Action: b.LifecyclePrepareSession, Input: b.LifecycleInput{Turn: &input}}, probeID(t), "probe-prepare", time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	writeLocalProbe(t, host, root, "prepare-intent.json", prepared)
	r, _, e := l.Execute(ctx, prepared.Request)
	writeLocalProbe(t, host, root, "prepare-receipt.json", r)
	if e != nil {
		t.Fatal(e)
	}
	bind(&s, r)
	intent, e := operation.New(runID, s, b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: &input}}, probeID(t), "probe-submit", time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	req := intent.Request
	host.businessTerminal, host.businessRequest = identity.Terminal, nativeRequest(req.Operation.ID)
	writeLocalProbe(t, host, root, "submit-intent.json", intent)
	writeLocalProbe(t, host, root, "snapshot.json", s)
	record["business_started_at"] = time.Now().UTC()
	r, obs, e := l.Execute(ctx, req)
	writeLocalProbe(t, host, root, "submit-receipt.json", r)
	record["submission_attempts"] = 1
	if r.State.InputAccepted {
		record["business_inputs"] = 1
	}
	record["submit_receipt"] = r.Operation.Receipt
	record["request_id"] = r.RequestID
	record["operation_id"] = req.Operation.ID
	record["initial_start_confirmed"] = r.State.TurnStarted
	if r.State.TurnStarted {
		t.Fatal("send stage was promoted to original business start")
	}
	if e != nil || obs == nil {
		t.Fatal("original submission unverified", e)
	}
	bind(&s, r)
	defer func() {
		obs.Close(context.Background())
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if e := obs.(*observer).awaitCleanup(cleanup); e != nil {
			t.Error(e)
		}
	}()
	diagnostics := []string{}
	for {
		select {
		case event, ok := <-obs.Events():
			if !ok {
				t.Fatal("observer ended before completion", diagnostics)
			}
			if len(event.Implementation) > 0 {
				s.Implementation = event.Implementation
			}
			if event.Observation != nil {
				diagnostics = append(diagnostics, event.Observation.Diagnostic)
			}
			completed := event.Fact.Kind == b.EventCompleted && event.Fact.Completed
			event.Persisted <- nil
			event.Processed <- nil
			if completed {
				r, _, e = l.ObserveLifecycle(ctx, s)
				if e != nil || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || strings.TrimSpace(r.TurnEvidence.LastMessage) != marker {
					t.Fatal("original final unverified", e, r.Diagnostic)
				}
				sample, err := native.QueryOriginalSample(ctx, target.Environment, identity, ready.PaneKey)
				if err != nil {
					t.Fatal(err)
				}
				verified, err := native.QueryOriginalInput(ctx, target.Environment, identity, ready.PaneKey, ready.MainAgent.StateStartedAt, sample.AgentState)
				if err != nil || verified.NativeInput == nil || verified.NativeInput.Text != nativePromptText(prompt) || !verified.NativeInput.MatchesOriginalProcess(identity) || !verified.NativeInput.MatchesOriginalQuery(verified) || verified.LastAssistantMessage != marker || verified.MainAgent.StateStartedAt <= ready.MainAgent.StateStartedAt {
					t.Fatal("independent original input/final mismatch", err)
				}
				record["original_query"] = verified
				record["verified_original_input"] = verified.NativeInput
				writeLocalProbe(t, host, root, "original-input-pages.json", verified.NativeInput.Pages)
				writeLocalProbe(t, host, root, "snapshot.json", s)
				if host.submits.Load() != 1 || host.textOnly.Load() != 0 || host.bareEnter.Load() != 0 || host.reads.Load() != 0 || host.businessForeignRequest.Load() != 0 {
					t.Fatal("expected one production generic submit and no split input or screen read")
				}
				record["completed"] = true
				record["completed_at"] = time.Now().UTC()
				record["final"] = r.TurnEvidence
				record["diagnostics"] = diagnostics
				return
			}
		case <-ctx.Done():
			record["diagnostics"] = diagnostics
			t.Fatal("native original did not complete", ctx.Err())
		}
	}
}

// Logs actual public reads only; no send interception, mutation, or fabricated response.
type nativeInputProbeHost struct {
	b.ManagedProcessHost
	root                                                                string
	reads                                                               atomic.Uint64
	calls                                                               atomic.Uint64
	pastes                                                              atomic.Uint64
	enters                                                              atomic.Uint64
	submits                                                             atomic.Uint64
	textOnly                                                            atomic.Uint64
	bareEnter                                                           atomic.Uint64
	businessRequest, businessTerminal                                   string
	businessSubmits, businessTextOnly, businessBareEnter, businessReads atomic.Uint64
	businessForeignRequest                                              atomic.Uint64
}

func (h *nativeInputProbeHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	started := time.Now().UTC()
	arg := func(key string) string {
		for i, a := range c.Args {
			if a == key && i+1 < len(c.Args) {
				return c.Args[i+1]
			}
		}
		return ""
	}
	if len(c.Args) > 1 && c.Args[0] == "terminal" && h.businessRequest != "" {
		if c.Args[1] == "send" && arg("--terminal") == h.businessTerminal {
			enter := false
			for _, a := range c.Args {
				enter = enter || a == "--enter"
			}
			if arg("--text") != "" && enter {
				h.businessSubmits.Add(1)
				if arg("--retry-request") != h.businessRequest {
					h.businessForeignRequest.Add(1)
				}
			} else if enter {
				h.businessBareEnter.Add(1)
			} else {
				h.businessTextOnly.Add(1)
			}
		}
		if c.Args[1] == "read" && arg("--terminal") == h.businessTerminal {
			h.businessReads.Add(1)
		}
	}
	if len(c.Args) > 1 && c.Args[0] == "terminal" && c.Args[1] == "send" {
		hasText, hasEnter := false, false
		for _, arg := range c.Args {
			if arg == "--text" {
				hasText = true
				h.pastes.Add(1)
			}
			if arg == "--enter" {
				hasEnter = true
				h.enters.Add(1)
			}
		}
		if hasText && hasEnter {
			h.submits.Add(1)
		} else if hasText {
			h.textOnly.Add(1)
		} else if hasEnter {
			h.bareEnter.Add(1)
		}
	}
	raw, err := h.ManagedProcessHost.OutputCommand(ctx, c)
	{
		textBytes := 0
		for i, arg := range c.Args {
			if arg == "--text" && i+1 < len(c.Args) {
				textBytes = len(c.Args[i+1])
			}
		}
		data, _ := json.MarshalIndent(map[string]any{"started": started, "returned": time.Now().UTC(), "command": c, "text_transport_bytes": textBytes, "windows_command_line_utf16_with_nul": process.CommandLineUnits(c.Executable, c.Args), "exit_error": errorText(err), "context_error": errorText(ctx.Err()), "stdout_bytes": len(raw), "stdout": json.RawMessage(raw)}, "", "  ")
		if len(data) == 0 {
			data, _ = json.Marshal(map[string]any{"started": started, "returned": time.Now().UTC(), "command": c, "text_transport_bytes": textBytes, "exit_error": errorText(err), "context_error": errorText(ctx.Err()), "stdout_text": string(raw)})
		}
		if e := os.WriteFile(filepath.Join(h.root, fmt.Sprintf("actual-cli-%03d.json", h.calls.Add(1))), data, 0600); e != nil {
			return nil, e
		}
	}
	if len(c.Args) > 1 && c.Args[0] == "terminal" && c.Args[1] == "read" {
		n := h.reads.Add(1)
		if e := os.WriteFile(filepath.Join(h.root, fmt.Sprintf("unexpected-terminal-read-%02d.json", n)), raw, 0600); e != nil {
			return nil, e
		}
	}
	return raw, err
}
