package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

type checkPort interface {
	Prepare(context.Context) (codex.Binding, error)
	Start(context.Context) (string, error)
	Next(context.Context) (codex.EnvironmentCall, error)
	Execute(context.Context, string, contracts.CheckResources) (map[string]string, error)
	Verify(context.Context, string, contracts.CheckResources) (map[string]string, error)
	Reply(codex.EnvironmentCall, string) error
	Cleanup(context.Context, string, contracts.CheckResources) (contracts.CheckResources, error)
	Stop(context.Context) bool
}

type checkWorker struct {
	gate   sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	port   checkPort
}

func (e *Engine) commitCheckLocked(next *runstate.CheckRecord, command *storage.Command) error {
	expected := uint64(0)
	if prior := e.checks[next.View.CheckID]; prior != nil {
		expected = prior.View.Revision
	}
	next.View.Revision = expected + 1
	next.View.UpdatedAt = time.Now().UTC()
	if err := e.store.CommitCheck(next, expected, command); err != nil {
		return err
	}
	e.checks[next.View.CheckID] = next
	return nil
}

func (e *Engine) HandleEnvironmentCheck(command, checkID, requestID string, input *contracts.Request) contracts.Response {
	if input == nil {
		if command != "environment-check inspect" || (checkID == "") == (requestID == "") {
			return contracts.Rejected("", contracts.Fail(2, "invalid_arguments", "inspect requires exactly one check-id or request-id."))
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		inspection := contracts.CheckInspection{}
		if requestID != "" {
			c, err := e.store.CheckCommand(requestID)
			if errors.Is(err, sql.ErrNoRows) {
				return contracts.Rejected("", contracts.Fail(5, "request_not_found", "Original environment check request was not found."))
			}
			if err != nil {
				return contracts.Rejected("", fault(err))
			}
			inspection.Command = &c
			if c.CheckID != nil {
				checkID = *c.CheckID
			}
		}
		if checkID != "" {
			r := e.checks[checkID]
			if r == nil {
				return contracts.Rejected("", contracts.Fail(5, "check_not_found", "Environment check was not found."))
			}
			inspection.Check = &r.Clone().View
		}
		return contracts.Success("", inspection)
	}
	id := input.RequestID
	digest := storage.RequestDigest(command, "", *input)
	entry, _ := e.requests.LoadOrStore(id, &sync.Mutex{})
	lock := entry.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if original, found, err := e.store.Original(id, digest); err != nil {
		return contracts.Rejected(id, fault(err))
	} else if found {
		return original
	}
	// All recognizable business refusals remain queryable by their original ID.
	reject := func(f *contracts.Fault) contracts.Response {
		c := storage.NewCheckCommand(command, "", *input)
		c.Status = "failed"
		c.Response = contracts.Rejected(id, f)
		if err := e.store.CommitCheck(nil, 0, &c); err != nil {
			return contracts.Rejected(id, contracts.Fail(6, "request_outcome_unknown", "Original rejection could not be persisted; inspect the same request_id."))
		}
		return c.Response
	}
	if invalid := contracts.ValidateInput(command, input.Input); invalid != nil {
		return reject(invalid)
	}
	if command == "environment-check start" {
		value := contracts.DecodeCheckInput(input.Input)
		if runtime.GOOS != "windows" {
			return reject(contracts.Fail(3, "unsupported_platform", "Environment checks require Windows native permission evidence."))
		}
		if value.Execution.Adapter == "orca-terminal" && value.Execution.Target.Kind == "orca_environment" {
			return reject(contracts.Fail(3, "unsupported_capability", "Full remote environment checks are unsupported; ordinary Orca native Runs use execution-host authentication."))
		}
		remote := value.Execution.Adapter == "orca-terminal"
		if !remote && (value.Execution.Adapter != "codex-app-server" || value.Execution.Target.Kind != "local") {
			return reject(contracts.Fail(3, "unsupported_capability", "Unsupported environment-check execution target."))
		}
		if remote {
			var settings contracts.OrcaSettings
			_ = json.Unmarshal(value.Execution.Settings, &settings)
			if settings.AgentLaunch.RuntimeExecutable == "" {
				return reject(contracts.Fail(3, "unsupported_capability", "Orca environment checks require an installed execution-host runtime."))
			}
		}
		if !codex.SupportsEnvironmentRepository(value.Repository.URL) {
			return reject(contracts.Fail(3, "unsupported_capability", "Check authentication currently requires an HTTPS github.com repository URL ending in .git."))
		}
		// The native provider owner currently implements GitHub's fixed REST
		// contract. A query-only command provider cannot prove PR write/close.
		if value.Publication != nil {
			if _, err := providerForInput(value.Repository, *value.Publication); err != nil {
				return reject(contracts.Fail(2, "invalid_input", "Publication configuration differs from the fixed repository or non-secret contract."))
			}
			if value.Publication.Provider != "command" {
				return reject(contracts.Fail(3, "unsupported_capability", "Environment PR probes currently require the GitHub command publication contract."))
			}
			var p contracts.CommandProviderSettings
			_ = json.Unmarshal(value.Publication.Settings, &p)
			if p.BaseURL != "https://github.com" || strings.TrimSuffix(value.Repository.URL, ".git") != "https://github.com/"+value.Publication.Repository {
				return reject(contracts.Fail(2, "invalid_input", "Publication identity differs from the fixed GitHub repository."))
			}
		}
		var settings contracts.CodexSettings
		if remote {
			var settingsRemote contracts.OrcaSettings
			_ = json.Unmarshal(value.Execution.Settings, &settingsRemote)
			if settingsRemote.AccessMode == "" {
				settingsRemote.AccessMode = "sandbox"
			}
			if settingsRemote.AgentLaunch.Executable == "" {
				settingsRemote.AgentLaunch.Executable = "codex"
			}
			value.Execution.Settings = canonical(settingsRemote)
			settings.AccessMode = settingsRemote.AccessMode
		} else {
			_ = json.Unmarshal(value.Execution.Settings, &settings)
		}
		if settings.AccessMode == "" {
			settings.AccessMode = "sandbox"
		}
		if !remote {
			value.Execution.Settings = canonical(settings)
		}
		execution, cleanup, _ := contracts.CheckDurations(value)
		now := time.Now().UTC()
		checkID = contracts.NewID()
		view := contracts.EnvironmentCheck{CheckID: checkID, Contract: contracts.EnvironmentCheckContract, Input: value, InputDigest: reliable.Digest(canonical(value)), ResourceDigest: resources.Digest(), CreatedAt: now, UpdatedAt: now, ExecutionDeadline: now.Add(execution), CleanupDeadline: now.Add(execution + cleanup), Status: "queued", Result: "unverified", CleanupStatus: "pending", Items: contracts.CheckManifest(value.Publication != nil), AllowedActions: []string{"stop"}, Excluded: []string{"build", "tests", "business_resources"}, Actual: contracts.CheckContext{AttemptID: contracts.NewID(), Generation: contracts.NewID()}, Resources: contracts.CheckResources{BaseRef: "refs/heads/steerline-check/" + checkID + "/base", HeadRef: "refs/heads/steerline-check/" + checkID + "/head", BaseSHA: strings.ToLower(value.Repository.BaseRevision)}}
		c := storage.NewCheckCommand(command, checkID, *input)
		r := &runstate.CheckRecord{View: view, Nonce: contracts.NewID(), Allowed: true, StartCommand: c.ID, CommandStates: map[string]string{c.ID: "pending"}, Operations: []b.Operation{}}
		e.mu.Lock()
		if e.ctx.Err() != nil {
			e.mu.Unlock()
			return reject(contracts.Fail(4, "service_unavailable", "Service is stopping."))
		}
		for _, prior := range e.checks {
			if prior.View.Input.Repository.URL == value.Repository.URL && (!prior.View.Resources.WriterStopped || prior.View.CleanupStatus != "completed" || checkUnknown(prior)) {
				e.mu.Unlock()
				return reject(contracts.Fail(5, "operation_unverified", "The original check has unresolved writers, objects or operations; inspect and settle it first."))
			}
		}
		if err := e.commitCheckLocked(r, &c); err != nil {
			e.mu.Unlock()
			return contracts.Rejected(id, contracts.Fail(6, "request_outcome_unknown", "Check acceptance is unverified; inspect the original request_id."))
		}
		e.launchCheckLocked(checkID)
		e.mu.Unlock()
		return c.Response
	}
	if command != "environment-check stop" {
		return reject(contracts.Fail(2, "invalid_arguments", "Unknown environment check command."))
	}
	var stop contracts.CheckStopInput
	_ = json.Unmarshal(input.Input, &stop)
	e.mu.Lock()
	w := e.checkWorkers[stop.CheckID]
	e.mu.Unlock()
	if w != nil {
		w.gate.Lock()
		defer w.gate.Unlock()
	}
	e.mu.Lock()
	r := e.checks[stop.CheckID]
	if r == nil {
		e.mu.Unlock()
		return reject(contracts.Fail(5, "check_not_found", "Original check was not found."))
	}
	if r.View.Input.Execution.Adapter == "orca-terminal" && r.View.Input.Execution.Target.Kind != "local" {
		e.mu.Unlock()
		return reject(contracts.Fail(3, "unsupported_capability", "Legacy remote checks are read-only; no helper actor is restarted. Settle original resources with the original build or manually."))
	}
	if r.View.Revision != stop.ExpectedRevision {
		e.mu.Unlock()
		return reject(contracts.Fail(5, "stale_revision", "Check revision changed; inspect before submitting a new stop."))
	}
	if e.ctx.Err() != nil {
		e.mu.Unlock()
		return reject(contracts.Fail(4, "service_unavailable", "Service is stopping."))
	}
	next := r.Clone()
	next.Allowed = false
	next.AgentSettled = r.AgentSettled || r.View.Status == "completed"
	next.View.Status = "stopping"
	c := storage.NewCheckCommand(command, stop.CheckID, *input)
	next.CommandStates[c.ID] = "pending"
	if err := e.commitCheckLocked(next, &c); err != nil {
		e.mu.Unlock()
		return contracts.Rejected(id, contracts.Fail(6, "request_outcome_unknown", "Stop acceptance is unverified; inspect the original request_id."))
	}
	if w != nil {
		w.cancel()
	} else {
		e.launchCheckLocked(stop.CheckID)
	}
	e.mu.Unlock()
	return c.Response
}

func checkUnknown(r *runstate.CheckRecord) bool {
	for _, op := range r.Operations {
		if op.Status == b.OperationInFlight || op.Status == b.OperationUnknown {
			return true
		}
	}
	return false
}

func (e *Engine) launchCheckLocked(id string) {
	if r := e.checks[id]; r.View.Input.Execution.Adapter == "orca-terminal" && r.View.Input.Execution.Target.Kind != "local" {
		return
	}
	ctx, cancel := context.WithDeadline(e.ctx, e.checks[id].View.ExecutionDeadline)
	w := &checkWorker{cancel: cancel, done: make(chan struct{})}
	e.checkWorkers[id] = w
	e.wait.Add(1)
	go func() {
		defer e.wait.Done()
		defer close(w.done)
		defer cancel()
		e.runCheck(ctx, id, w)
		e.mu.Lock()
		if e.checkWorkers[id] == w {
			delete(e.checkWorkers, id)
		}
		e.mu.Unlock()
	}()
}

func (e *Engine) checkSnapshot(id string) *runstate.CheckRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.checks[id].Clone()
}
func (e *Engine) updateCheck(id string, change func(*runstate.CheckRecord)) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.checks[id].Clone()
	change(next)
	return e.commitCheckLocked(next, nil)
}

func (e *Engine) checkIntent(id, action, callID string, request any) (int, error) {
	index := -1
	err := e.updateCheck(id, func(r *runstate.CheckRecord) {
		index = len(r.Operations)
		raw := canonical(request)
		now := time.Now().UTC()
		kind, protocol := b.BackendLocal, b.ProtocolLocalCodexAppServer
		target := b.ExecutionTarget{ID: "local", ExecutionHost: "local"}
		if r.View.Input.Execution.Adapter == "orca-terminal" {
			var settings contracts.OrcaSettings
			_ = json.Unmarshal(r.View.Input.Execution.Settings, &settings)
			if settings.Transport.Executable == "" {
				settings.Transport.Executable = "orca.exe"
			}
			kind, protocol = b.BackendOrca, b.ProtocolOrcaTerminalV3
			target = (orca.Profile{Executable: settings.Transport.Executable, Args: settings.Transport.Args, Environment: r.View.Input.Execution.Target.Environment}).Target()
		}
		r.Operations = append(r.Operations, b.Operation{Version: 1, ID: contracts.NewID(), RunID: id, AttemptID: r.View.Actual.AttemptID, Backend: kind, Protocol: protocol, Target: target, Action: action, IdempotencyKey: callID, Request: raw, PayloadDigest: reliable.Digest(raw), Status: b.OperationInFlight, StartedAt: now, UpdatedAt: now})
	})
	return index, err
}

func (e *Engine) settleCheckOperation(id string, index int, status b.OperationStatus) error {
	return e.updateCheck(id, func(r *runstate.CheckRecord) {
		r.Operations[index].Status = status
		r.Operations[index].UpdatedAt = time.Now().UTC()
	})
}

func (e *Engine) runCheck(ctx context.Context, id string, w *checkWorker) {
	defer e.finishCheck(id, w)
	r := e.checkSnapshot(id)
	if !r.Allowed {
		return
	} // restart/stop can only settle original resources
	fail := func(stage string, err error) {
		_ = e.updateCheck(id, func(r *runstate.CheckRecord) {
			uncertain := ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "timeout")
			r.Allowed = false
			r.View.Status = "failed"
			if uncertain {
				r.View.Status = "unverified"
			}
			reason := codex.EnvironmentDiagnostic(err)
			r.View.Diagnostic = stage + ": " + reason
			if len(r.View.Items) > 0 {
				r.View.Items[0].Stage = stage
				if !uncertain {
					r.View.Items[0].Status = "failed"
				}
			}
			r.CommandStates[r.StartCommand] = "failed"
			if uncertain {
				r.CommandStates[r.StartCommand] = "unknown"
			}
		})
	}
	if err := e.prepareCheckWorkspace(ctx, id, w); err != nil {
		fail("workspace_preparation_failed", err)
		return
	}
	r = e.checkSnapshot(id)
	port := w.port
	var err error
	if port == nil {
		port, err = codex.NewEnvironmentSession(codex.EnvironmentSetup{CheckID: id, Nonce: r.Nonce, Directory: checkDirectory(e.dir, id), AuthenticationStateDirectory: e.dir, Workspace: r.Workspace.Path, Input: r.View.Input, LinkedWriteArgs: r.LinkedWriteArgs})
	}
	if err != nil {
		fail("authentication_configuration_failed", err)
		return
	}
	w.port = port
	prepare, err := e.checkIntent(id, "prepare_session", "", r.View.Actual)
	if err != nil {
		return
	}
	w.gate.Lock()
	if !e.checkSnapshot(id).Allowed || ctx.Err() != nil {
		w.gate.Unlock()
		_ = e.settleCheckOperation(id, prepare, b.OperationRejected)
		return
	}
	binding, prepareErr := port.Prepare(ctx)
	// Persist even partial/late process receipts before observing cancellation.
	err = e.updateCheck(id, func(r *runstate.CheckRecord) {
		if binding.Workspace != "" {
			r.Workspace.Path = binding.Workspace
		}
		r.NativeRoot = binding.Process
		r.NativeOwner = e.identity
		r.Managed = binding.Process != nil
		r.Endpoint = binding.Endpoint
		r.View.Actual.SessionID = binding.SessionID
		r.View.Actual.NativeGeneration = binding.Generation
		if binding.SessionID != "" {
			r.View.Actual.Nonce = r.Nonce
		}
		r.View.Actual.Workspace = r.Workspace.Path
		if binding.Process != nil {
			r.View.Actual.PID = binding.Process.PID
			r.View.Actual.ProcessCreated = binding.Process.Created
			r.View.Actual.RuntimeID = binding.Process.RuntimeID
			r.View.Actual.Terminal = binding.Process.Terminal
			r.View.Actual.TerminalIncarnation = binding.Process.Incarnation
			r.View.Actual.WorktreeID = binding.Process.WorktreeID
		}
		if binding.Process != nil && binding.Process.PID > 0 {
			r.View.Actual.RuntimeVersion = codex.PolicyCodexVersion
			r.View.Actual.AuthenticationSource = "host-gh-repository-broker"
			var settings struct {
				AccessMode string `json:"access_mode"`
			}
			_ = json.Unmarshal(r.View.Input.Execution.Settings, &settings)
			r.View.Actual.AccessMode = settings.AccessMode
		}
	})
	w.gate.Unlock()
	if err != nil {
		return
	}
	if prepareErr != nil {
		_ = e.settleCheckOperation(id, prepare, b.OperationRejected)
		fail("native_preparation_failed", prepareErr)
		return
	}
	if err = e.settleCheckOperation(id, prepare, b.OperationConfirmed); err != nil {
		return
	}
	start, err := e.checkIntent(id, "start_turn", "", map[string]string{"nonce": r.Nonce, "session_id": binding.SessionID})
	if err != nil {
		return
	}
	w.gate.Lock()
	if !e.checkSnapshot(id).Allowed || ctx.Err() != nil {
		w.gate.Unlock()
		_ = e.settleCheckOperation(id, start, b.OperationRejected)
		return
	}
	turn, startErr := port.Start(ctx)
	err = e.updateCheck(id, func(r *runstate.CheckRecord) { r.View.Actual.TurnID = turn; r.View.Status = "running" })
	w.gate.Unlock()
	if err != nil {
		return
	}
	if startErr != nil {
		e.failCheckStart(id, start, turn, startErr)
		return
	}
	if err = e.settleCheckOperation(id, start, b.OperationConfirmed); err != nil {
		return
	}
	for {
		call, nextErr := port.Next(ctx)
		if nextErr != nil {
			_ = e.updateCheck(id, func(r *runstate.CheckRecord) {
				r.Allowed = false
				r.View.Status = "unverified"
				r.View.Diagnostic = codex.EnvironmentDiagnostic(nextErr)
				if r.View.Diagnostic == "preparation_unverified" {
					r.View.Diagnostic = "original_turn_unsettled"
				}
			})
			return
		}
		if call.Completed {
			_ = e.updateCheck(id, func(r *runstate.CheckRecord) {
				r.AgentSettled = true
				r.Allowed = false
				r.View.Result = contracts.CheckResult(r.View)
				r.View.Status = "completed"
				r.CommandStates[r.StartCommand] = "completed"
			})
			return
		}
		if err = e.executeCheckProbe(ctx, id, w, call); err != nil {
			_ = e.updateCheck(id, func(next *runstate.CheckRecord) {
				if next.Allowed {
					next.Allowed = false
					next.View.Status = "unverified"
					next.View.Diagnostic = "native_probe_dispatch_unverified: " + codex.EnvironmentDiagnostic(err)
					next.CommandStates[next.StartCommand] = "unknown"
				}
			})
			return
		}
	}
}

func (e *Engine) executeCheckProbe(ctx context.Context, id string, w *checkWorker, call codex.EnvironmentCall) error {
	w.gate.Lock()
	defer w.gate.Unlock()
	r := e.checkSnapshot(id)
	if !r.Allowed || ctx.Err() != nil || call.Nonce != r.Nonce {
		return errors.New("check_dispatch_revoked")
	}
	itemIndex := -1
	for i, item := range r.View.Items {
		if item.ID == call.Action && item.Required {
			itemIndex = i
			break
		}
	}
	if itemIndex < 0 {
		return errors.New("unexpected_check_action")
	}
	// Deduplicate by action as well as native call identity, including a second
	// call ID in the same model turn. Unknown originals are never re-executed.
	for _, op := range r.Operations {
		if op.Action == call.Action {
			status := "unverified"
			if op.Status == b.OperationConfirmed {
				status = "passed"
			}
			return w.port.Reply(call, status)
		}
	}
	for i := 0; i < itemIndex; i++ {
		if r.View.Items[i].Required && r.View.Items[i].Status != "passed" {
			return errors.New("upstream_probe_unresolved")
		}
	}
	index, err := e.checkIntent(id, call.Action, call.CallID, r.View.Resources)
	if err != nil {
		return err
	}
	facts, probeErr := w.port.Execute(ctx, call.Action, r.View.Resources)
	resources := r.View.Resources
	if call.Action == "commit" && facts["head_sha"] != "" {
		resources.HeadSHA = facts["head_sha"]
	}
	if call.Action == "pr_create" {
		if n := facts["pr_number"]; n != "" {
			_, _ = fmt.Sscan(n, &resources.PRNumber)
			resources.PRURL = facts["pr_url"]
		}
	}
	verified, verifyErr := w.port.Verify(ctx, call.Action, resources)
	status := b.OperationConfirmed
	itemStatus := "passed"
	if probeErr != nil || verifyErr != nil {
		status = b.OperationUnknown
		itemStatus = "unverified"
		if probeErr != nil && probeErr.Error() == "native_probe_failed" && !externalCheckAction(call.Action) {
			status = b.OperationRejected
			itemStatus = "failed"
		}
	}
	if probeErr != nil && probeErr.Error() == "native_probe_rejected" {
		itemStatus = "failed"
	}
	if call.Action == "pr_close" && probeErr == nil && verified["pr_closed"] != "True" {
		status = b.OperationUnknown
		itemStatus = "unverified"
	}
	err = e.updateCheck(id, func(next *runstate.CheckRecord) {
		next.View.Resources = resources
		if itemStatus == "passed" && call.Action == "base_push" {
			next.View.Resources.BaseOwned = true
		}
		if itemStatus == "passed" && call.Action == "head_push" {
			next.View.Resources.HeadOwned = true
		}
		if call.Action == "pr_close" && itemStatus == "passed" {
			next.View.Resources.PRClosed = true
		}
		if call.Action == "permissions" && itemStatus == "passed" {
			next.View.Actual.Account = facts["account"]
			next.View.Actual.Sandbox = facts["sandbox"]
			next.View.Actual.ApprovalPolicy = facts["approval_policy"]
		}
		for k, v := range verified {
			if facts == nil {
				facts = map[string]string{}
			}
			facts["owner_"+k] = v
		}
		op := &next.Operations[index]
		op.Status = status
		op.UpdatedAt = time.Now().UTC()
		next.View.Items[itemIndex].Status = itemStatus
		next.View.Items[itemIndex].Stage = "native_probe"
		if facts["failure_stage"] == "authorization_denied" {
			next.View.Items[itemIndex].Stage = "authorization_denied"
		}
		next.View.Items[itemIndex].Evidence = append(next.View.Items[itemIndex].Evidence, contracts.CheckEvidence{CheckID: id, SessionID: next.View.Actual.SessionID, TurnID: next.View.Actual.TurnID, Nonce: next.Nonce, OperationID: op.ID, ToolCallID: call.CallID, Source: "native_tool_and_owner_verification", Verified: itemStatus == "passed", Facts: facts})
		if itemStatus != "passed" {
			next.Allowed = false
			next.View.Status = itemStatus
			next.View.Diagnostic = call.Action + "_" + itemStatus
			failure := probeErr
			if failure == nil {
				failure = verifyErr
			}
			if category := codex.EnvironmentDiagnostic(failure); category != "" && category != "preparation_unverified" {
				next.View.Diagnostic += ": " + category
			}
		}
	})
	if err != nil {
		return err
	}
	if err = w.port.Reply(call, itemStatus); err != nil {
		_ = e.updateCheck(id, func(next *runstate.CheckRecord) {
			next.Allowed = false
			if itemStatus == "passed" {
				next.View.Status = "unverified"
				next.View.Diagnostic = "native_reply_unverified: " + codex.EnvironmentDiagnostic(err)
				next.CommandStates[next.StartCommand] = "unknown"
			}
		})
		return err
	}
	if itemStatus != "passed" {
		return errors.New("probe not verified")
	}
	return nil
}

func (e *Engine) prepareCheckWorkspace(ctx context.Context, id string, w *checkWorker) error {
	w.gate.Lock()
	defer w.gate.Unlock()
	r := e.checkSnapshot(id)
	if !r.Allowed || ctx.Err() != nil {
		return errors.New("check preparation revoked")
	}
	if r.View.Input.Execution.Adapter == "orca-terminal" {
		index, err := e.checkIntent(id, "prepare_workspace", "", map[string]string{"target": r.View.Input.Execution.Target.Environment})
		if err != nil {
			return err
		}
		port, err := orca.NewLocalTerminalEnvironmentSession(codex.RemoteCheckSetup{CheckID: id, Nonce: r.Nonce, Input: r.View.Input, ExecutionDeadline: r.View.ExecutionDeadline, CleanupDeadline: r.View.CleanupDeadline}, r.Endpoint)
		if err != nil {
			return err
		}
		w.port = port
		binding, err := port.Initialize(ctx)
		if binding.Process != nil {
			saveErr := e.updateCheck(id, func(next *runstate.CheckRecord) {
				next.NativeRoot = binding.Process
				next.Endpoint = binding.Endpoint
				next.NativeOwner = e.identity
				next.Managed = true
			})
			if saveErr != nil {
				return saveErr
			}
		}
		if err != nil {
			return err
		}
		return e.settleCheckOperation(id, index, b.OperationConfirmed)
	}
	root := checkDirectory(e.dir, id)
	// Exact paths and ownership intent precede creation; no shared cache is
	// removed during settlement, and a restart never retries this clone.
	if err := e.updateCheck(id, func(r *runstate.CheckRecord) {
		r.RepositoryPath = filepath.Join(root, "r")
		r.Workspace = b.WorkspaceIntent{Path: filepath.Join(root, "w"), Branch: "steerline-check/" + id + "/head", BaseCommit: r.View.Resources.BaseSHA}
	}); err != nil {
		return err
	}
	index, err := e.checkIntent(id, "prepare_workspace", "", map[string]string{"directory": root})
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(root, 0700); err != nil {
		_ = e.settleCheckOperation(id, index, b.OperationRejected)
		return err
	}
	if err = e.updateCheck(id, func(r *runstate.CheckRecord) { r.DirectoryOwned = true }); err != nil {
		return err
	}
	if err = process.New(30*time.Second).WriteExecutionFile(filepath.Join(root, ".check-owner"), []byte(id+"\n"+r.Nonce+"\n")); err != nil {
		return err
	}
	r = e.checkSnapshot(id)
	git := gitHost()
	source := r.View.Input.Repository.URL
	if r.View.Input.Repository.LocalPath != "" {
		source = r.View.Input.Repository.LocalPath
		out, err := git.Output(ctx, source, "remote", "get-url", r.View.Input.Repository.Remote)
		if err != nil || strings.TrimSpace(string(out)) != r.View.Input.Repository.URL {
			_ = e.settleCheckOperation(id, index, b.OperationRejected)
			return errors.New("repository_identity_mismatch")
		}
	}
	if err = workspace.CloneIsolated(ctx, git, e.dir, source, r.RepositoryPath, r.View.Input.Repository.Remote, r.View.Input.Repository.BaseRevision); err != nil {
		_ = e.settleCheckOperation(id, index, b.OperationRejected)
		return err
	}
	if _, err = git.Output(ctx, r.RepositoryPath, "remote", "set-url", r.View.Input.Repository.Remote, r.View.Input.Repository.URL); err != nil {
		return err
	}
	repository, err := workspace.ResolveRepository(ctx, git, r.RepositoryPath)
	if err != nil {
		return err
	}
	if err = e.updateCheck(id, func(r *runstate.CheckRecord) { r.RepositoryCommon = repository.CommonDir }); err != nil {
		return err
	}
	if _, err = workspace.Prepare(ctx, git, repository, root, &r.Workspace); err != nil {
		return err
	}
	roots, err := workspace.ResolveLinkedResources(ctx, git, repository, &r.Workspace)
	if err != nil {
		return err
	}
	args, err := codex.LinkedWorktreeWriteArgs(roots.Objects, roots.Refs, roots.Logs, roots.Admin)
	if err != nil {
		return err
	}
	if err = e.updateCheck(id, func(r *runstate.CheckRecord) { r.LinkedWriteArgs = args }); err != nil {
		return err
	}
	return e.settleCheckOperation(id, index, b.OperationConfirmed)
}

func (e *Engine) finishCheck(id string, w *checkWorker) {
	w.gate.Lock()
	defer w.gate.Unlock()
	r := e.checkSnapshot(id)
	if w.port == nil && r.View.Input.Execution.Adapter == "orca-terminal" && r.Endpoint != "" && !r.View.Resources.WriterStopped {
		port, err := orca.NewLocalTerminalEnvironmentSession(codex.RemoteCheckSetup{CheckID: id, Nonce: r.Nonce, Input: r.View.Input, ExecutionDeadline: r.View.ExecutionDeadline, CleanupDeadline: r.View.CleanupDeadline}, r.Endpoint)
		if err == nil {
			w.port = port
		}
	}
	_ = e.updateCheck(id, func(r *runstate.CheckRecord) {
		r.Allowed = false
		if !r.AgentSettled {
			r.View.Result = contracts.CheckResult(r.View)
			if r.View.Result == "passed" {
				r.View.Result = "unverified"
			}
		}
		r.View.CleanupStatus = "pending"
	})
	// The accepted cleanup deadline is never reset by process restart or stop.
	_, cleanupBudget, _ := contracts.CheckDurations(r.View.Input)
	cleanupEnd := time.Now().Add(cleanupBudget)
	if r.View.CleanupDeadline.Before(cleanupEnd) {
		cleanupEnd = r.View.CleanupDeadline
	}
	ctx, cancel := context.WithDeadline(context.Background(), cleanupEnd)
	defer cancel()
	resources := r.View.Resources
	externalSettled := true
	for _, op := range r.Operations {
		if op.Action == "base_push" && op.Status != b.OperationRejected && !resources.BaseDeleted || op.Action == "head_push" && op.Status != b.OperationRejected && !resources.HeadDeleted || op.Action == "pr_create" && op.Status != b.OperationRejected && !resources.PRClosed {
			externalSettled = false
		}
	}
	if w.port != nil && !externalSettled {
		externalSettled = true
		for _, op := range r.Operations {
			if (op.Action == "base_push" || op.Action == "head_push" || op.Action == "pr_create") && (op.Status == b.OperationUnknown || op.Status == b.OperationInFlight) {
				externalSettled = false
			}
		}
		if externalSettled {
			for _, action := range []string{"cleanup_pr_close", "cleanup_head", "cleanup_base"} {
				needed := action == "cleanup_pr_close" && resources.PRNumber > 0 && !resources.PRClosed || action == "cleanup_head" && resources.HeadOwned && !resources.HeadDeleted || action == "cleanup_base" && resources.BaseOwned && !resources.BaseDeleted
				if !needed {
					continue
				}
				index, err := e.checkIntent(id, action, "", resources)
				if err != nil {
					externalSettled = false
					break
				}
				var cleanupErr error
				resources, cleanupErr = w.port.Cleanup(ctx, action, resources)
				status := b.OperationConfirmed
				if cleanupErr != nil {
					status = b.OperationUnknown
					externalSettled = false
				}
				_ = e.settleCheckOperation(id, index, status)
				_ = e.updateCheck(id, func(r *runstate.CheckRecord) { r.View.Resources = resources })
				if cleanupErr != nil {
					break
				}
			}
		}
	}

	stopped := r.View.Resources.WriterStopped || r.NativeRoot == nil && !checkUnknown(r)
	if w.port != nil {
		if remote, ok := w.port.(*orca.LocalTerminalEnvironmentSession); ok {
			remote.SetCleanupScope(externalSettled)
		}
		index, err := e.checkIntent(id, "stop_original_writer", "", map[string]any{"process": r.NativeRoot, "owner": r.NativeOwner, "session_id": r.View.Actual.SessionID})
		if err == nil {
			stopped = w.port.Stop(ctx)
			status := b.OperationUnknown
			if stopped {
				status = b.OperationConfirmed
			}
			_ = e.settleCheckOperation(id, index, status)
		} else {
			stopped = w.port.Stop(ctx)
		}
	} else if r.NativeRoot != nil && !stopped && r.View.Input.Execution.Adapter != "orca-terminal" {
		stopped = r.Managed && r.NativeOwner != nil && process.NewServiceFacts().Probe(r.NativeOwner).State == "absent" && process.NewServiceFacts().Probe(r.NativeRoot).State == "absent"
	}
	// Preparation with no native launch needs no native writer evidence.
	if r.NativeRoot == nil {
		stopped = true
		for _, op := range r.Operations {
			if (op.Action == "prepare_session" || op.Action == "prepare_workspace" && r.View.Input.Execution.Adapter == "orca-terminal") && (op.Status == b.OperationInFlight || op.Status == b.OperationUnknown) {
				stopped = false
			}
		}
	}
	resources.WriterStopped = stopped
	if stopped {
		_ = e.updateCheck(id, func(r *runstate.CheckRecord) {
			for i := range r.Operations {
				op := &r.Operations[i]
				if op.Action == "stop_original_writer" && (op.Status == b.OperationUnknown || op.Status == b.OperationInFlight) {
					op.Status = b.OperationConfirmed
					op.UpdatedAt = time.Now().UTC()
				}
			}
		})
	}
	localRemoved := r.View.Resources.LocalRemoved || (!r.DirectoryOwned && !checkUnknown(r))
	if remote, ok := w.port.(*orca.LocalTerminalEnvironmentSession); ok {
		localRemoved = remote.LocalRemoved()
	}
	if stopped && externalSettled && r.DirectoryOwned && !localRemoved {
		root := checkDirectory(e.dir, id)
		actual, err := filepath.EvalSymlinks(root)
		if errors.Is(err, os.ErrNotExist) {
			localRemoved = true
		} else if err == nil && strings.EqualFold(filepath.Clean(actual), filepath.Clean(root)) && strings.EqualFold(filepath.Dir(actual), filepath.Join(e.dir, "c")) && checkDirectoryOwner(root, id, r.Nonce) {
			// Only the private directory whose creation was durably owned above.
			localRemoved = os.RemoveAll(root) == nil
		}
	}
	resources.LocalRemoved = localRemoved
	_ = e.updateCheck(id, func(next *runstate.CheckRecord) {
		next.View.Resources = resources
		next.Allowed = false
		next.View.AllowedActions = []string{"stop"}
		next.View.CleanupStatus = "residual"
		if externalSettled && stopped && localRemoved && !checkUnknown(next) {
			next.View.CleanupStatus = "completed"
			next.View.AllowedActions = []string{}
		}
		if next.View.Status == "queued" || next.View.Status == "running" || next.View.Status == "stopping" {
			next.View.Status = "stopped"
		}
		for command, status := range next.CommandStates {
			if status == "pending" {
				if command == next.StartCommand {
					next.CommandStates[command] = "failed"
				} else if next.View.CleanupStatus == "completed" {
					next.CommandStates[command] = "completed"
				} else {
					next.CommandStates[command] = "unknown"
				}
			}
		}
	})
}

func checkDirectory(dir, id string) string { return filepath.Join(dir, "c", compactDirectoryKey(id)) }

func externalCheckAction(action string) bool {
	return action == "base_push" || action == "head_push" || action == "pr_create" || action == "pr_close"
}
func checkDirectoryOwner(root, id, nonce string) bool {
	file, err := os.Open(filepath.Join(root, ".check-owner"))
	if err != nil {
		return false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 257))
	return err == nil && string(raw) == id+"\n"+nonce+"\n"
}

// A rejected external command with an independently observed original object
// is an uncertain effect, never evidence that there was nothing to settle.
func reconcileCheckOwnership(r *runstate.CheckRecord) *runstate.CheckRecord {
	var next *runstate.CheckRecord
	for i, op := range r.Operations {
		if !externalCheckAction(op.Action) || op.Status != b.OperationRejected {
			continue
		}
		for _, item := range r.View.Items {
			for _, evidence := range item.Evidence {
				if evidence.OperationID == op.ID && (evidence.Facts["owner_sha"] != "" || evidence.Facts["owner_pr_number"] != "") {
					if next == nil {
						next = r.Clone()
					}
					next.Operations[i].Status = b.OperationUnknown
					next.Allowed = false
					next.View.CleanupStatus = "residual"
					next.View.AllowedActions = []string{"stop"}
					next.View.Diagnostic = "external_write_outcome_unverified"
				}
			}
		}
	}
	return next
}

func (e *Engine) failCheckStart(id string, start int, turn string, startErr error) {
	var rejected *codex.EnvironmentInputRejectedError
	inputNotSent := turn == "" && errors.As(startErr, &rejected)
	disposition := b.OperationUnknown
	if inputNotSent {
		disposition = b.OperationRejected
	}
	_ = e.settleCheckOperation(id, start, disposition)
	_ = e.updateCheck(id, func(r *runstate.CheckRecord) {
		r.Allowed = false
		r.View.Status = "unverified"
		r.View.Diagnostic = "native_input_unverified: " + codex.EnvironmentDiagnostic(startErr)
		r.CommandStates[r.StartCommand] = "unknown"
		if inputNotSent {
			r.View.Status = "failed"
			r.View.Diagnostic = "native_input_rejected: " + codex.EnvironmentDiagnostic(startErr)
			r.CommandStates[r.StartCommand] = "failed"
		}
	})
}
