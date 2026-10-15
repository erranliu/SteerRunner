package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"os"
	"strings"
	"time"
)

func (e *Engine) run(id string, w *worker) {
	defer e.closeWorker(id, w)
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		var events <-chan b.LifecycleEvent
		var done <-chan error
		if w.observer != nil {
			events = w.observer.Events()
			done = w.observer.Done()
		}
		select {
		case <-e.ctx.Done():
			return
		case event, ok := <-events:
			if ok {
				err := e.acceptEvent(id, event)
				if event.Persisted != nil {
					event.Persisted <- err
				}
				if event.Processed != nil {
					event.Processed <- err
				}
				if err != nil {
					e.logger.Printf("event commit unavailable run=%s", id)
				}
			}
		case _, ok := <-done:
			if ok || w.observer != nil {
				if w.observer != nil {
					_ = w.observer.Close(context.Background())
					w.observer = nil
				}
				e.signal(w)
			}
		case <-w.wake:
			for i := 0; i < reliable.PreparationStepLimit; i++ {
				if !e.advance(id, w) {
					break
				}
			}
		case <-timer.C:
			e.signal(w)
		}
	}
}
func (e *Engine) advance(id string, w *worker) bool {
	w.gate.Lock()
	defer w.gate.Unlock()
	if e.ctx.Err() != nil {
		return false
	}
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	if r.Status == "pr_ready" && r.PendingResume == "" && r.PendingStop == "" && r.PendingFeedback == nil {
		return false
	}
	p, err := e.port(r, w)
	if err != nil {
		e.block(id, "backend_configuration", err, false)
		return false
	}
	if r.PendingStop != "" {
		return e.stopRun(id, w, p)
	}
	if r.StopStatus == "stopped" && r.PendingResume == "" {
		return false
	}
	if unknown(r) {
		for i, intent := range r.Intents {
			if intent.Request.Operation.Status == b.OperationInFlight || intent.Request.Operation.Status == b.OperationUnknown {
				return e.invoke(id, w, p, i, true)
			}
		}
	}
	if r.PendingResume != "" {
		return e.resumeRun(id, w, p)
	}
	if r.PendingPRURL != "" && r.StopStatus == "" {
		return e.verifyPublication(id)
	}
	if r.PR == nil && len(r.PRClues) > 0 && r.StopStatus == "" && !time.Now().Before(r.PRClueProbeAt) {
		return e.probePRClues(id)
	}
	if r.PendingAnswer != nil && r.Allowed && !r.ResumeRequired {
		return e.answer(id, w, p)
	}
	if r.Snapshot.Session.PendingTurnID != "" || r.Snapshot.Session.CurrentTurnID != "" && r.Snapshot.Session.CurrentTurnID != r.Snapshot.Session.LastStoppedTurnID {
		if r.PendingAnswer != nil && r.Allowed && !r.ResumeRequired {
			return e.answer(id, w, p)
		}
		if w.observer != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
		receipt, observer, err := p.ObserveLifecycle(ctx, r.Snapshot.Clone())
		cancel()
		if err != nil {
			if observer != nil {
				_ = observer.Close(context.Background())
			}
			e.block(id, "observation_unverified", err, true)
			return false
		}
		if !e.applyObservation(id, r, receipt) {
			if observer != nil {
				_ = observer.Close(context.Background())
			}
			return false
		}
		if observer != nil {
			w.observer = observer
			if h := observer.StopHandle(); h != nil {
				e.retainHandle(w, h)
			}
		}
		if receipt.TurnEvidence != nil {
			return e.finish(id, *receipt.TurnEvidence)
		}
		return false
	}
	if !r.Allowed || r.ResumeRequired {
		return false
	}
	if r.Request.Execution.Adapter == "codex-app-server" && r.Snapshot.Workspace.Path != "" {
		if err := e.verifyPolicyResources(r); err != nil {
			e.block(id, "permission_resources_unverified", err, false)
			return false
		}
		if e.runsPolicyMissing(id) {
			e.mu.Lock()
			err := e.commitLocked(r, nil, "permission_resources_bound")
			e.mu.Unlock()
			return err == nil
		}
	}
	if r.PendingFeedback != nil {
		return e.feedback(id, w, p)
	}
	if r.Status == "blocked" || r.Status == "waiting_input" || r.Status == "stopped" {
		return false
	}
	if r.Snapshot.Workspace.Path != "" && r.Snapshot.PendingTurn == nil && len(r.Workflow.StageInputs) == 0 {
		if err := materialize(r); err != nil {
			e.block(id, "materialization_failed", err, false)
			return false
		}
		if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
			e.block(id, "input_failed", err, false)
			return false
		}
		if err := e.attachWriteRoots(r); err != nil {
			e.block(id, "workspace_unverified", err, false)
			return false
		}
		e.mu.Lock()
		err := e.commitLocked(r, nil, "input_frozen")
		e.mu.Unlock()
		return err == nil
	}
	if r.Snapshot.PendingTurn == nil && r.Snapshot.Workspace.Path != "" {
		return false
	}
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	candidate, err := reliable.Next(ctx, p, r.Snapshot.Clone())
	cancel()
	if err != nil {
		e.block(id, "preparation_blocked", err, false)
		return false
	}
	if candidate.Done {
		return false
	}
	if r.PreparationDeadline.IsZero() {
		r.PreparationDeadline = time.Now().UTC().Add(3 * time.Minute)
	}
	if time.Now().After(r.PreparationDeadline) && candidate.Action != b.LifecycleStartTurn && candidate.Action != b.LifecycleSubmitInput {
		e.block(id, "preparation_deadline", errors.New("The original preparation budget is exhausted"), false)
		return false
	}
	i, err := e.saveIntent(id, r, candidate)
	if err != nil {
		return false
	}
	return e.invoke(id, w, p, i, false)
}

func (e *Engine) saveIntent(id string, r *runstate.Record, candidate b.LifecycleCandidate) (int, error) {
	value, err := reliable.InputValue(candidate.Action, candidate.Input)
	if err != nil {
		return 0, err
	}
	raw := canonical(value)
	ref := r.Snapshot.Ref
	now := time.Now().UTC()
	op := b.Operation{Version: 1, ID: contracts.NewID(), RunID: id, AttemptID: r.Snapshot.Session.AttemptID, Backend: ref.Backend, Protocol: ref.Protocol, Target: ref.Target, Action: string(candidate.Action), IdempotencyKey: contracts.NewID(), Request: raw, PayloadDigest: reliable.Digest(raw), Status: b.OperationInFlight, StartedAt: now, UpdatedAt: now, Ref: &ref, LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, ConfigurationDigest: r.Snapshot.ConfigurationDigest, ResourceGeneration: r.Snapshot.ResourceGeneration, InputBindingDigest: b.LifecycleBindingDigest(r.Snapshot), Ref: ref}}
	request := b.LifecycleRequest{Snapshot: r.Snapshot.Clone(), Operation: op, Input: candidate.Input.Clone()}
	if err := request.Validate(); err != nil {
		return 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if current.Snapshot.Session.AttemptID != r.Snapshot.Session.AttemptID || b.LifecycleBindingDigest(current.Snapshot) != b.LifecycleBindingDigest(r.Snapshot) {
		return 0, reliable.ErrStale
	}
	next := current.Clone()
	next.PreparationDeadline = r.PreparationDeadline
	next.Intents = append(next.Intents, runstate.Intent{Request: request, Mode: string(reliable.ExecuteNewIntent)})
	next.Snapshot.Operations = append(next.Snapshot.Operations, op)
	next.Phase = string(candidate.Action)
	if err := e.commitLocked(next, nil, "operation_committed"); err != nil {
		return 0, err
	}
	return len(next.Intents) - 1, nil
}
func (e *Engine) invoke(id string, w *worker, p b.LifecyclePort, index int, reconcile bool) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	intent := r.Intents[index]
	mode := reliable.ExecuteNewIntent
	if reconcile {
		mode = reliable.ReconcileOriginalIntent
	}
	if !reconcile && !r.Allowed && intent.Request.Operation.Action != string(b.LifecycleStopExecution) {
		return false
	}
	ctx, cancel := context.WithTimeout(e.ctx, 90*time.Second)
	result := reliable.Invoke(ctx, p, p, intent.Request, mode)
	cancel()
	if result.Observer != nil {
		if w.observer != nil {
			_ = w.observer.Close(context.Background())
		}
		w.observer = result.Observer
		if h := w.observer.StopHandle(); h != nil {
			e.retainHandle(w, h)
		}
	}
	if result.Receipt.Handle != nil {
		e.retainHandle(w, result.Receipt.Handle)
	}
	e.mu.Lock()
	next := e.runs[id].Clone()
	if h, ok := w.handle.(b.OwnedProcessHandle); ok && h.OwnedProcess() {
		next.NativeManaged = true
		next.NativeOwner = e.identity
	}
	if next.Snapshot.Session.AttemptID != intent.Request.Snapshot.Session.AttemptID {
		e.mu.Unlock()
		return false
	}
	status, diagnostic := reliable.Outcome(result.Receipt, result.Err)
	if result.Uncertain {
		status = string(b.OperationUnknown)
	}
	if err := merge(next, result.Receipt); err != nil {
		status = string(b.OperationUnknown)
		diagnostic = "Native receipt did not match the original execution identity"
	}
	op := intent.Request.Operation
	op.Status = b.OperationStatus(status)
	op.UpdatedAt = time.Now().UTC()
	op.Diagnostic = e.redact(next, diagnostic)
	savedReceipt := result.Receipt.Clone()
	savedReceipt.Operation.Diagnostic = e.redact(next, savedReceipt.Operation.Diagnostic)
	if intent.Request.Operation.Action != string(b.LifecyclePrepareWorkspace) {
		savedReceipt.Operation.Receipt = nil
	}
	op.Receipt = canonical(savedReceipt)
	next.Intents[index].Request.Operation = op
	next.Intents[index].Receipt = &savedReceipt
	next.Intents[index].Mode = string(mode)
	for i := range next.Snapshot.Operations {
		if next.Snapshot.Operations[i].ID == op.ID {
			next.Snapshot.Operations[i] = op
		}
	}
	next.Diagnostic = op.Diagnostic
	if reconcile && status == string(b.OperationUnknown) && intent.Request.Operation.Status == b.OperationUnknown && intent.Request.Operation.Diagnostic == op.Diagnostic && b.LifecycleBindingDigest(next.Snapshot) == b.LifecycleBindingDigest(r.Snapshot) {
		e.mu.Unlock()
		return false
	}
	progress := status == "confirmed" || status == "accepted"
	if !progress {
		next.Status = "reconciling"
		if status == "rejected" {
			next.Status = "blocked"
			next.Allowed, next.ResumeRequired = false, true
		}
		next.Phase = "operation_" + status
	}
	if progress && intent.Request.Input.Feedback != nil {
		if next.PendingAnswer != nil {
			next.CommandStates[next.PendingAnswer.CommandID] = "completed"
			next.PendingAnswer = nil
		}
		next.Question = nil
		next.Snapshot.QuestionRoute = nil
		next.Snapshot.PendingQuestions = nil
		next.Status = "running"
	}
	if progress && next.Snapshot.Session.CurrentCommandID != "" {
		next.CommandStates[next.SubmitCommandID] = "completed"
	}
	if progress && intent.Request.Input.Turn != nil && result.Receipt.State.InputAccepted {
		if stage := workflow.Input(next.Workflow, intent.Request.Input.Turn.CommandID); stage != nil && stage.FeedbackDecisionID != "" {
			next.CommandStates[stage.FeedbackDecisionID] = "completed"
		}
	}
	err := e.commitLocked(next, nil, "operation_"+status)
	e.mu.Unlock()
	if err != nil {
		return false
	}
	if savedReceipt.TurnEvidence != nil {
		return e.finish(id, *savedReceipt.TurnEvidence)
	}
	return progress
}

func (e *Engine) retainHandle(w *worker, h b.StopHandle) {
	if h == nil {
		return
	}
	if owned, ok := w.handle.(b.OwnedProcessHandle); ok && owned.OwnedProcess() {
		if incoming, ok := h.(b.OwnedProcessHandle); !ok || !incoming.OwnedProcess() {
			return
		}
	}
	w.handle = h
}
func merge(r *runstate.Record, receipt b.LifecycleReceipt) error {
	s := r.Snapshot
	binding := reliable.BindingSnapshot{ProjectIdentity: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, CurrentAttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigurationDigest: s.ConfigurationDigest, Target: s.Ref.Target, SessionID: s.Session.SessionID, SessionGeneration: s.Session.Generation, PendingCommandID: s.Session.PendingCommandID, PendingTurnID: s.Session.PendingTurnID, CurrentTurnID: s.Session.CurrentTurnID, ImplementationData: s.Implementation}
	if s.Ref.External.Kind != "" {
		identity := s.Ref.External
		binding.Process = &identity
	}
	if r.WorkspaceFact != nil {
		f := r.WorkspaceFact
		binding.Workspace = reliable.Workspace{InspectionScope: f.InspectionScope, Repository: f.Repository, Worktree: f.Worktree, Branch: f.Branch, Base: f.Base, Head: f.Head, Dirty: f.Dirty, VerifiedAt: f.CheckedAt}
	}
	updated, err := reliable.MergeReceipt(binding, receipt)
	if err != nil {
		return err
	}
	r.Snapshot.Implementation = updated.ImplementationData
	r.Snapshot.Ref.Target = updated.Target
	r.Snapshot.Session.Target = updated.Target
	r.Snapshot.Session.SessionID = updated.SessionID
	r.Snapshot.Session.Generation = updated.SessionGeneration
	if updated.Process != nil {
		r.Snapshot.Ref.External = *updated.Process
	}
	if receipt.Workspace != nil {
		f := *receipt.Workspace
		r.WorkspaceFact = &f
		r.Snapshot.Workspace = b.WorkspaceIntent{Path: f.Worktree, Branch: f.Branch, BaseCommit: f.Base}
		r.Snapshot.Ref.WorkspacePath = f.Worktree
		r.Snapshot.Ref.WorkspaceBranch = f.Branch
	}
	if receipt.State.TurnID != "" && receipt.State.InputAccepted {
		if r.Snapshot.Session.PendingCommandID != "" {
			if receipt.State.TurnStarted {
				r.Snapshot.Session.CurrentTurnID = receipt.State.TurnID
				r.Snapshot.Session.CurrentCommandID = r.Snapshot.Session.PendingCommandID
				r.Snapshot.Session.PendingCommandID = ""
				r.Snapshot.Session.PendingTurnID = ""
				r.NativeStarted = true
			} else {
				r.Snapshot.Session.PendingTurnID = receipt.State.TurnID
				r.NativeStarted = false
			}
		}
		r.Snapshot.PendingTurn = nil
		r.Status = "running"
		r.Phase = "observing"
		if !r.NativeStarted {
			r.Phase = "input_accepted_waiting_start"
		}
	}
	return nil
}
func (e *Engine) applyObservation(id string, original *runstate.Record, receipt b.LifecycleReceipt) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[id].Clone()
	if reliable.ValidateResultBinding(next.Snapshot, original.Snapshot) != nil {
		return false
	}
	if merge(next, receipt) != nil {
		return false
	}
	return e.commitLocked(next, nil, "observation_attached") == nil
}
func (e *Engine) attachWriteRoots(r *runstate.Record) error {
	if r.Request.Execution.Adapter != "codex-app-server" {
		return nil
	}
	roots, err := workspace.ResolveLinkedResources(e.ctx, gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, &r.Snapshot.Workspace)
	if err != nil {
		return err
	}
	args, err := codex.LinkedWorktreeWriteArgs(roots.Objects, roots.Refs, roots.Logs, roots.Admin)
	if err != nil {
		return err
	}
	r.LinkedWriteArgs = args
	if r.Snapshot.PendingTurn != nil {
		r.Snapshot.PendingTurn.LinkedWorktreeArgs = args
	}
	return nil
}
func (e *Engine) block(id, phase string, err error, uncertain bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	message := e.redact(r, err.Error())
	if r.Diagnostic == message && r.Phase == phase {
		return
	}
	r.Phase, r.Diagnostic = phase, message
	r.Status = "blocked"
	if uncertain {
		r.Status = "reconciling"
	} else if phase != "fresh_reply_required" && phase != "fresh_feedback_required" {
		r.Allowed, r.ResumeRequired = false, true
	}
	if commitErr := e.commitLocked(r, nil, "execution_blocked"); commitErr != nil {
		e.logger.Printf("cannot commit blocker run=%s", id)
	}
}
func (e *Engine) redact(r *runstate.Record, text string) string {
	var refs struct {
		AuthenticationRefs []string `json:"authentication_refs"`
	}
	_ = json.Unmarshal(r.Request.Publication.Settings, &refs)
	for _, name := range refs.AuthenticationRefs {
		if secret := os.Getenv(name); secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	if len(text) > 4096 {
		text = text[:4096]
	}
	return text
}
