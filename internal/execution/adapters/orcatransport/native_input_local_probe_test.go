package orcatransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func probeID(t *testing.T) string {
	t.Helper()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(id[:])
}

// The local adapter qualifies its existing v3 helper/control/history pipeline.
// It never substitutes remote completion, synthesizes readiness, or alters a
// production callback. Only native CLI calls and event persistence are recorded.
func runLocalNativeInputProbe(t *testing.T, root string, target nativeInputProbeTarget) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	record := map[string]any{"target_kind": "local", "completed": false, "cleaned_up": false, "emergency_cleanup": false, "business_inputs": 0, "launch": target.Launch}
	host := &nativeInputProbeHost{ManagedProcessHost: process.New(40 * time.Second), root: root}
	defer func() {
		record["total_generic_calls"], record["total_terminal_reads"] = host.submits.Load(), host.reads.Load()
		record["submit_calls"], record["text_only_calls"], record["bare_enter_calls"], record["screen_reads"] = host.businessSubmits.Load(), host.businessTextOnly.Load(), host.businessBareEnter.Load(), host.businessReads.Load()
		record["business_request_mismatches"] = host.businessForeignRequest.Load()
		data, _ := json.MarshalIndent(record, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "native-result.json"), data, 0600); err != nil {
			t.Error(err)
		}
	}()
	helperSHA, err := validateLocalProbeTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	record["agent_sha256"] = helperSHA
	creator := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: target.CLI, Args: args, WorkingDirectory: root, Timeout: timeout})
	}})
	census := func(ctx context.Context, name string) bool {
		result, e := creator.MetaCall(ctx, "terminal", "list", "--worktree", target.Worktree, "--json")
		record[name] = result
		return e == nil && localProbeEmptyCensus(result, target.Runtime)
	}
	waitEmpty := func(ctx context.Context, name string) bool {
		until := time.Now().Add(5 * time.Second)
		for {
			if census(ctx, name) {
				return true
			}
			if time.Now().After(until) {
				return false
			}
			select {
			case <-ctx.Done():
				return false
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
	if !census(ctx, "before_census") {
		t.Fatal("local probe requires an empty authoritative fresh worktree")
	}
	created, e := creator.MetaCall(ctx, "terminal", "create", "--worktree", target.Worktree, "--title", "SteerRunner isolated local input", "--json")
	record["terminal_create"] = created
	var value struct{ Terminal native.Terminal }
	if e != nil || created.CallUncertain || created.RuntimeID != target.Runtime || json.Unmarshal(created.Payload, &value) != nil || value.Terminal.Handle == "" || value.Terminal.IncarnationID == "" || value.Terminal.WorktreeID != strings.TrimPrefix(target.Worktree, "id:") {
		t.Fatal("original empty shell creation unverified", e)
	}
	original := value.Terminal
	mainIdentity := b.ProcessIdentity{Kind: "orca_terminal", RuntimeID: target.Runtime, Terminal: original.Handle, Incarnation: original.IncarnationID, WorktreeID: original.WorktreeID, ExecutionHost: "local"}
	var l *Lifecycle
	var snapshot b.LifecycleSnapshot
	var observed b.LifecycleObserver
	var closeObserver sync.Once
	stopObserver := func() {
		closeObserver.Do(func() {
			if observed != nil {
				c, done := context.WithTimeout(context.Background(), 15*time.Second)
				defer done()
				_ = observed.Close(c)
				if err := observed.(*observer).awaitCleanup(c); err != nil {
					t.Error(err)
				}
			}
		})
	}
	defer func() {
		stopObserver()
		cleanup, done := context.WithTimeout(context.Background(), 100*time.Second)
		defer done()
		if l != nil {
			candidate := b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{}}}
			intent, e := operation.New(snapshot.Ref.RunID, snapshot, candidate, probeID(t), "probe-stop", time.Now().UTC())
			if e == nil {
				writeLocalProbe(t, host, root, "stop-intent.json", intent)
				receipt, stopErr := l.StopLifecycle(cleanup, intent.Request, nil)
				record["stop_receipt"], record["stop_error"] = receipt, errorText(stopErr)
				writeLocalProbe(t, host, root, "stop-receipt.json", receipt)
				if stopErr == nil && receipt.State.Stopped && receipt.State.WorkerAbsent {
					verifyErr := l.verifyStopped(cleanup, snapshot)
					record["verify_stopped_error"] = errorText(verifyErr)
					state, stateErr := l.state(snapshot)
					if stateErr == nil && state.RemoteReadiness != nil {
						closure, authErr := ReadControlClosureEvidence(l.setup.RunDirectory, snapshot, state)
						record["control_closure"], record["control_error"] = closure, errorText(authErr)
						if path, pathErr := codex.RemoteExitPath(*state.RemoteReadiness); pathErr == nil {
							if raw, e := os.ReadFile(filepath.Join(state.RemoteReadiness.Workspace, filepath.FromSlash(path))); e == nil {
								record["signed_exit"] = json.RawMessage(raw)
							}
						}
						if verifyErr == nil && authErr == nil && waitEmpty(cleanup, "after_census") {
							record["cleaned_up"] = true
							l.closeStateFeed()
							return
						}
					}
				}
			}
			l.closeStateFeed()
		}
		// Emergency recovery is explicitly a failed qualification. Close only the
		// two positively owned terminals; never turn their absence into formal stop.
		record["emergency_cleanup"] = true
		if l != nil {
			if st, e := l.state(snapshot); e == nil && st.Control != nil {
				proof, e := creator.CloseReadonlyTerminal(cleanup, st.Control.Process)
				record["emergency_control"], record["emergency_control_error"] = proof, errorText(e)
			}
		}
		_, identityErr := creator.InspectBoundTerminal(cleanup, b.ExecutionTarget{}, mainIdentity)
		proof, e := b.StopReceipt{}, identityErr
		var progress stopProgress
		if saved, ok := record["stop_receipt"].(b.LifecycleReceipt); ok {
			_ = json.Unmarshal(saved.Operation.Receipt, &progress)
		}
		if e == nil && progress.Phase != "attempted" && progress.Phase != "business-confirmed" && progress.Phase != "confirmed" {
			proof, e = creator.CloseTerminalChecked(cleanup, mainIdentity.Terminal, mainIdentity.RuntimeID)
		}
		record["emergency_terminal"], record["emergency_terminal_error"] = proof, errorText(e)
		record["emergency_census_empty"] = waitEmpty(cleanup, "emergency_census")
		t.Error("formal original local stop/authentication cleanup not verified")
	}()
	shown, e := creator.ShowTerminal(ctx, original.Handle)
	if e != nil || shown.IncarnationID != original.IncarnationID || shown.WorktreeID != original.WorktreeID {
		t.Fatal(e)
	}
	original = shown
	profile := Profile{Executable: target.CLI, WaitTimeout: "40s", RuntimeExecutable: target.Launch.RuntimeExecutable, RuntimeSHA256: helperSHA, RuntimeConfigurationDigest: codex.RemoteConfigurationDigest(target.Launch.RemoteConfig())}
	identity := b.ProcessIdentity{Kind: "orca_terminal", Executable: target.CLI, Terminal: original.Handle, RuntimeID: target.Runtime, Incarnation: original.IncarnationID, WorktreeID: original.WorktreeID, ExecutionHost: "local"}
	mainIdentity = identity
	runID, attemptID, command := probeID(t), probeID(t), probeID(t)
	marker := "SR_LOCAL_NATIVE_" + command
	text := "No-tools native input qualification. Do not use tools, read or change files, run commands, create commits, deploy or publish. Reply exactly with " + marker + ".\r\n\r\n" + strings.Repeat("中文 original line; preserve blank lines.\r\n\r\n", 100)
	prompt, budget, e := model.FreezeInput(model.InputPart{Source: "caller.direct", Owner: model.CallerInput, Text: text})
	if e != nil {
		t.Fatal(e)
	}
	input := b.TurnInput{CommandID: command, Prompt: prompt, Budget: budget, InputVersion: 1, Model: target.Launch.Model, ReasoningEffort: target.Launch.ReasoningEffort, StepID: workflow.StepImplement}
	snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: runID, AttemptID: attemptID, External: identity}, Session: b.SessionContext{ProjectID: "native-input-probe", ExecutionID: runID, AttemptID: attemptID, WorkflowRunID: runID, SessionID: SessionID(identity), PendingCommandID: command}, ConfigurationDigest: profile.RuntimeConfigurationDigest, ResourceGeneration: probeID(t), Workspace: b.WorkspaceIntent{Path: original.WorktreePath, Branch: strings.TrimPrefix(original.Branch, "refs/heads/")}, PendingTurn: &input}
	config, _ := json.Marshal(profile)
	envelope, e := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: ProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: snapshot.Session.ProjectID, ProfileID: "native-local", AcceptedConfigVersion: "native-local"}, config)
	if e != nil {
		t.Fatal(e)
	}
	launch, e := target.Launch.Command("codex")
	if e != nil {
		t.Fatal(e)
	}
	setup := b.LifecycleSetup{Snapshot: snapshot, ProfileData: []b.ImplementationDataEnvelope{envelope}, ProfileID: "native-local", AcceptedConfigVersion: "native-local", ProcessHost: host, RepositoryPath: original.WorktreePath, RepositoryURL: target.Launch.RepositoryURL, RunDirectory: filepath.Join(root, "owned-journal"), AgentLaunchCommand: launch}
	port, e := NewLifecycle(setup)
	if e != nil {
		t.Fatal(e)
	}
	l = port.(*Lifecycle)
	snapshot.Implementation, e = l.envelope(snapshot, State{Process: identity})
	if e != nil {
		t.Fatal(e)
	}
	record["run_id"], record["process"], record["logical_sha256"], record["input_budget"] = runID, identity, digest([]byte(prompt)), budget
	writeLocalProbe(t, host, root, "original-input.json", input)
	step := 0
	for {
		candidate, e := reliable.Next(ctx, l, snapshot)
		if e != nil {
			t.Fatal("original preparation unverified", e)
		}
		if candidate.Done {
			t.Fatal("input was not prepared")
		}
		step++
		intent, e := operation.New(runID, snapshot, candidate, probeID(t), "probe-"+string(candidate.Action), time.Now().UTC())
		if e != nil {
			t.Fatal(e)
		}
		snapshot.Operations = append(snapshot.Operations, intent.Request.Operation)
		if candidate.Action == b.LifecycleStartTurn {
			host.businessRequest = nativeRequest(intent.Request.Operation.ID)
			host.businessTerminal = identity.Terminal
			record["request_id"] = host.businessRequest
			record["operation_id"] = intent.Request.Operation.ID
		}
		writeLocalProbe(t, host, root, "intent-"+intent.Request.Operation.ID+".json", intent)
		poll := 0
		for {
			poll++
			intent.Request.Snapshot = snapshot.Clone()
			writeLocalProbe(t, host, root, "current-intent.json", intent)
			writeLocalProbe(t, host, root, "snapshot.json", snapshot)
			mode := operation.InvocationMode(intent.Mode)
			result := operation.Invoke(ctx, l, l, intent.Request, mode)
			writeLocalProbe(t, host, root, fmt.Sprintf("step-%02d-%03d-receipt.json", step, poll), result.Receipt)
			status, diagnostic := operation.Outcome(result.Receipt, result.Err)
			intent.Settle(snapshot.Operations, result.Receipt, mode, b.OperationStatus(status), diagnostic, time.Now().UTC())
			snapshot, _, e = reliable.ApplySessionReceipt(snapshot, nil, result.Receipt, false)
			if e != nil {
				t.Fatal("receipt binding changed", e)
			}
			writeLocalProbe(t, host, root, "current-intent.json", intent)
			writeLocalProbe(t, host, root, "snapshot.json", snapshot)
			if result.Err != nil {
				// Match the existing application's unknown-resource recovery: keep
				// the original intent and only reconcile, never execute again.
				if candidate.Action == b.LifecycleCreateResource && intent.NeedsReconciliation() && poll < reliable.ReconciliationStepLimit {
					record["preparation_reconciliation_reason"] = result.Err.Error()
					intent.Mode = string(operation.ReconcileOriginalIntent)
					if e = controlPause(ctx); e != nil {
						t.Fatal(e)
					}
					continue
				}
				t.Fatal("original lifecycle step unverified", candidate.Action, result.Err)
			}
			if result.Receipt.State.ResourcePending {
				intent.Mode = string(operation.ReconcileOriginalIntent)
				if e = controlPause(ctx); e != nil {
					t.Fatal(e)
				}
				continue
			}
			if candidate.Action == b.LifecycleStartTurn {
				observed = result.Observer
				record["submit_receipt"], record["initial_start_confirmed"] = result.Receipt.Operation.Receipt, result.Receipt.State.TurnStarted
				if result.Receipt.State.TurnStarted {
					t.Fatal("send stage was promoted to original business start")
				}
				record["business_inputs"] = 1
			}
			break
		}
		if candidate.Action == b.LifecycleStartTurn {
			break
		}
		if step >= reliable.PreparationStepLimit {
			t.Fatal("existing preparation step limit exceeded")
		}
	}
	if observed == nil {
		t.Fatal("accepted input has no original observer")
	}
	var facts []b.EventFact
	for {
		select {
		case event, ok := <-observed.Events():
			if !ok {
				t.Fatal("observer ended before original completion")
			}
			if len(event.Implementation) > 0 {
				snapshot.Implementation = event.Implementation
			}
			writeLocalProbe(t, host, root, "snapshot.json", snapshot)
			facts = append(facts, event.Fact)
			writeLocalProbe(t, host, root, "observer-facts.json", facts)
			record["facts"] = facts
			data, _ := json.Marshal(event.Fact)
			record["last_fact"] = json.RawMessage(data)
			complete := event.Fact.Kind == b.EventCompleted && event.Fact.Completed
			event.Persisted <- nil
			event.Processed <- nil
			if !complete {
				continue
			}
			receipt, _, e := l.ObserveLifecycle(ctx, snapshot)
			if e != nil || receipt.TurnEvidence == nil || !receipt.TurnEvidence.FinalVerified || strings.TrimSpace(receipt.TurnEvidence.LastMessage) != marker {
				t.Fatal("original local final mismatch", e, receipt.Diagnostic)
			}
			state, e := l.state(snapshot)
			if e != nil {
				t.Fatal(e)
			}
			turn := turnFor(&state, command)
			if turn == nil || ValidateRecordedTurn(state, turn) != nil || !turn.Started || !turn.Settled || turn.Recorded == nil || turn.Recorded.Model != input.Model || turn.Recorded.ReasoningEffort != input.ReasoningEffort {
				t.Fatal("original history/model binding missing")
			}
			if host.businessSubmits.Load() != 1 || host.businessTextOnly.Load() != 0 || host.businessBareEnter.Load() != 0 || host.businessReads.Load() != 0 || host.businessForeignRequest.Load() != 0 {
				t.Fatal("business input repeated or used composer projection")
			}
			record["wire_sha256"], record["recorded_turn"], record["final"], record["completed"] = digest([]byte(wirePrompt(turn))), turn.Recorded, receipt.TurnEvidence, true
			record["submit_calls"], record["text_only_calls"], record["bare_enter_calls"], record["screen_reads"] = host.businessSubmits.Load(), host.businessTextOnly.Load(), host.businessBareEnter.Load(), host.businessReads.Load()
			record["snapshot"] = snapshot
			writeLocalProbe(t, host, root, "wire-input.txt", wirePrompt(turn))
			stopObserver()
			return
		case <-ctx.Done():
			t.Fatal("original local input did not complete", ctx.Err())
		}
	}
}

func validateLocalProbeTarget(target nativeInputProbeTarget) (string, error) {
	if target.Environment != "" || !filepath.IsAbs(target.Launch.RuntimeExecutable) || target.Launch.RepositoryURL == "" || target.Launch.Model == "" || target.Launch.ReasoningEffort == "" {
		return "", errors.New("local probe needs the original v3 helper, repository and model configuration")
	}
	// The probe freezes a workflow model turn. Its helper must receive the same
	// model-enabled configuration that production workflow freezing supplies.
	audit := target.Launch.AuditModel
	if audit == nil || audit.Model == "" || strings.TrimSpace(audit.Model) != audit.Model || strings.ContainsAny(audit.Model, "\x00\r\n\t") || audit.Model == target.Launch.Model || !codex.ValidReasoningEffort(audit.ReasoningEffort) || !codex.ValidReasoningEffort(target.Launch.ReasoningEffort) {
		return "", errors.New("local model turn needs its original valid, distinct audit model configuration")
	}
	return codex.RuntimeDigest(target.Launch.RuntimeExecutable)
}

func writeLocalProbe(t *testing.T, host b.ManagedProcessHost, root, name string, value any) {
	t.Helper()
	var raw []byte
	var err error
	if text, ok := value.(string); ok {
		raw = []byte(text)
	} else {
		raw, err = json.MarshalIndent(value, "", "  ")
	}
	if err == nil {
		err = host.WriteExecutionFile(filepath.Join(root, name), raw)
	}
	if err != nil {
		t.Fatal("persist original local probe", name, err)
	}
}

func TestLocalNativeProbeTargetAndBusinessScope(t *testing.T) {
	root := t.TempDir()
	helper := filepath.Join(root, "existing-agent.exe")
	if err := os.WriteFile(helper, []byte("isolated fixture image"), 0600); err != nil {
		t.Fatal(err)
	}
	target := nativeInputProbeTarget{Launch: agentlaunch.Config{RuntimeExecutable: helper, RepositoryURL: "https://github.com/owner/repo.git", Model: "model", ReasoningEffort: "high", AuditModel: &workflow.ModelSettings{Model: "audit-model", ReasoningEffort: "high"}}}
	actual, err := validateLocalProbeTarget(target)
	if err != nil || actual != digest([]byte("isolated fixture image")) {
		t.Fatal(actual, err)
	}
	for _, fault := range []string{"remote", "helper", "repository", "model", "effort", "audit-missing", "audit-empty", "audit-same-model", "audit-effort", "audit-name", "writer-effort"} {
		t.Run(fault, func(t *testing.T) {
			changed := target
			changed.Launch = *target.Launch.Clone()
			switch fault {
			case "remote":
				changed.Environment = "paired"
			case "helper":
				changed.Launch.RuntimeExecutable = ""
			case "repository":
				changed.Launch.RepositoryURL = ""
			case "model":
				changed.Launch.Model = ""
			case "effort":
				changed.Launch.ReasoningEffort = ""
			case "audit-missing":
				changed.Launch.AuditModel = nil
			case "audit-empty":
				changed.Launch.AuditModel.Model = ""
			case "audit-same-model":
				changed.Launch.AuditModel.Model = target.Launch.Model
			case "audit-effort":
				changed.Launch.AuditModel.ReasoningEffort = "invalid"
			case "audit-name":
				changed.Launch.AuditModel.Model = "audit\nmodel"
			case "writer-effort":
				changed.Launch.ReasoningEffort = "invalid"
			}
			if _, err := validateLocalProbeTarget(changed); err == nil {
				t.Fatal("unqualified local target admitted")
			}
		})
	}
	host := &nativeInputProbeHost{ManagedProcessHost: &fakeHost{root: root, incarnation: "original"}, root: root}
	call := func(args ...string) {
		t.Helper()
		if _, err := host.OutputCommand(context.Background(), b.LaunchCommand{Args: args, Timeout: time.Second}); err != nil {
			t.Fatal(err)
		}
	}
	call("terminal", "send", "--terminal", "terminal", "--text", "original initialization", "--enter", "--retry-request", "launch")
	call("terminal", "read", "--terminal", "terminal")
	host.businessRequest, host.businessTerminal = "business", "terminal"
	call("terminal", "send", "--terminal", "control", "--text", "original query", "--enter", "--retry-request", "query")
	call("terminal", "read", "--terminal", "control")
	call("terminal", "send", "--terminal", "terminal", "--text", "original body", "--enter", "--retry-request", "business")
	if host.submits.Load() != 3 || host.reads.Load() != 2 || host.businessSubmits.Load() != 1 || host.businessReads.Load() != 0 {
		t.Fatal("setup/control were counted as business input")
	}
	call("terminal", "read", "--terminal", "terminal")
	if host.businessReads.Load() != 1 {
		t.Fatal("business composer read hidden")
	}
	call("terminal", "send", "--terminal", "terminal", "--text", "raw fragment")
	call("terminal", "send", "--terminal", "terminal", "--enter")
	call("terminal", "send", "--terminal", "terminal", "--text", "foreign generic", "--enter", "--retry-request", "foreign")
	if host.businessTextOnly.Load() != 1 || host.businessBareEnter.Load() != 1 || host.businessForeignRequest.Load() != 1 {
		t.Fatal("unbound raw/Enter or wrong request was hidden")
	}
}

// Match the existing originalWorktreeInventory contract: an unrelated omitted
// paired host does not own terminals bound to this exact local runtime.
func localProbeEmptyCensus(result native.Result, runtime string) bool {
	var v struct {
		TotalCount *int
		Truncated  *bool
		HostScope  *struct{ HostIDs, OmittedHostIDs *[]string }
		Terminals  *[]native.Terminal
	}
	if result.CallUncertain || result.ErrorCode != "" || result.RuntimeID != runtime || json.Unmarshal(result.Payload, &v) != nil || v.TotalCount == nil || *v.TotalCount != 0 || v.Truncated == nil || *v.Truncated || v.Terminals == nil || len(*v.Terminals) != 0 || v.HostScope == nil || v.HostScope.HostIDs == nil || v.HostScope.OmittedHostIDs == nil {
		return false
	}
	seen := map[string]bool{}
	for _, id := range *v.HostScope.HostIDs {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	if !seen["local"] {
		return false
	}
	omitted := map[string]bool{}
	for _, id := range *v.HostScope.OmittedHostIDs {
		if id == "" || seen[id] || omitted[id] {
			return false
		}
		omitted[id] = true
	}
	return true
}

func TestLocalNativeProbeCensusKeepsOriginalHostScope(t *testing.T) {
	for _, fault := range []string{"none", "foreign-runtime", "omitted-local", "no-local", "duplicate-host", "duplicate-omission", "truncated", "nonempty", "missing-count"} {
		t.Run(fault, func(t *testing.T) {
			r := native.Result{RuntimeID: "original"}
			hosts, omitted := []string{"local"}, []string{"runtime:offline-peer"}
			count := any(0)
			rows := []any{}
			truncated := false
			switch fault {
			case "foreign-runtime":
				r.RuntimeID = "replacement"
			case "omitted-local":
				omitted = append(omitted, "local")
			case "no-local":
				hosts = []string{"other"}
			case "duplicate-host":
				hosts = append(hosts, "local")
			case "duplicate-omission":
				omitted = append(omitted, omitted[0])
			case "truncated":
				truncated = true
			case "nonempty":
				count = 1
				rows = append(rows, map[string]string{"handle": "foreign"})
			case "missing-count":
				count = nil
			}
			r.Payload, _ = json.Marshal(map[string]any{"terminals": rows, "totalCount": count, "truncated": truncated, "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": omitted}})
			if localProbeEmptyCensus(r, "original") != (fault == "none") {
				t.Fatal("incorrect original-host inventory result")
			}
		})
	}
}
