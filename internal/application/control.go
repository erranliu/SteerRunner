package application

import (
	"context"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"time"
)

func controlScope(r *runstate.Record, question string) string {
	return "scope:" + reliable.Digest(canonical([]string{r.ID, r.Snapshot.Session.AttemptID, r.Snapshot.ResourceGeneration, r.Snapshot.Session.Generation, r.Snapshot.Session.SessionID, r.Snapshot.Session.CurrentCommandID, r.Snapshot.Session.CurrentTurnID, r.Workflow.StepID, question, reliable.Digest(canonical(r.Snapshot.QuestionRoute)), reliable.Digest(canonical(r.Question)), r.Workflow.ImplementationResultID, r.Workflow.AuditResultID}))
}

func (e *Engine) answer(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	saved := r.PendingAnswer
	if saved == nil {
		return false
	}
	if saved.Epoch != e.epoch && saved.Epoch != controlScope(r, saved.QuestionID) {
		e.block(id, "fresh_reply_required", errors.New("Saved answers from a prior service require a fresh reply to the current question"), false)
		return false
	}
	if r.Snapshot.QuestionRoute == nil {
		if r.Snapshot.Session.CurrentTurnID != r.Snapshot.Session.LastStoppedTurnID {
			return false
		}
		return e.feedbackValue(id, w, p, saved, true)
	}
	answers, err := nativeAnswers(r.Snapshot.PendingQuestions, saved.Text)
	if err != nil {
		e.failControl(id, saved.CommandID, err)
		return false
	}
	owner, ok := p.(b.LifecycleAnswerInputPort)
	if !ok {
		e.failControl(id, saved.CommandID, errors.New("Original native answer input preparation is unavailable"))
		return false
	}
	parts, err := owner.PrepareLifecycleAnswer(*r.Snapshot.QuestionRoute, r.Snapshot.Session, b.Answer{Answers: answers}, saved.Text)
	if err != nil {
		e.failControl(id, saved.CommandID, err)
		return false
	}
	_, budget, err := m.FreezeInput(parts...)
	if err != nil {
		e.failControl(id, saved.CommandID, err)
		return false
	}
	input := b.LifecycleInput{Feedback: &b.FeedbackInput{Budget: budget, DecisionID: saved.CommandID, QuestionID: saved.QuestionID, Message: saved.Text, Answer: &b.Answer{Answers: answers}}}
	index, err := e.saveIntent(id, r, b.LifecycleCandidate{Action: b.LifecycleDeliverFeedback, Input: input})
	if err != nil {
		return false
	}
	return e.invoke(id, w, p, index, false)
}
func (e *Engine) resumeRun(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	command := r.PendingResume
	deadline := time.Now().Add(30 * time.Second)
	manual := r.ManualPause != nil
	if manual {
		deadline = r.ManualPause.ResumeDeadline
		if deadline.IsZero() {
			deadline = r.UpdatedAt.Add(30 * time.Second)
		}
		if !time.Now().Before(deadline) {
			e.failControl(id, command, errors.New("Manual reconciliation deadline exhausted; use a new request_id and current revision for another read-only query"))
			return false
		}
		if !e.reconcileManual(id, w, p, command, deadline) {
			return false
		}
		e.mu.Lock()
		r = e.runs[id].Clone()
		e.mu.Unlock()
	}
	if r.Request.Conversation != nil || r.Request.Generate != nil {
		if err := verifyContentDirectory(r); err != nil {
			e.failControl(id, command, err)
			return false
		}
		if err := verifyFrozenMaterials(r); err != nil {
			e.failControl(id, command, err)
			return false
		}
	}
	if unknown(r) {
		e.failControl(id, command, errors.New("Original operations require reconciliation before any resume"))
		return false
	}
	if r.Snapshot.Session.SessionID == "" && r.Snapshot.Ref.External.Kind != "" {
		e.failControl(id, command, errors.New("An original native resource exists without a verified session; resume cannot create a substitute session"))
		return false
	}
	if r.Snapshot.Session.SessionID != "" {

		// Continuation requires the original resource. No session is rebound.
		needsInput := r.NextStage != nil || r.Snapshot.PendingTurn != nil
		if state, _ := r.Workflow.CurrentState(); state.Kind == "parse" {
			needsInput = true
		}
		ctx, cancel := context.WithDeadline(e.ctx, deadline)
		w.queryCancel = cancel
		w.gate.Unlock()
		err := e.qualifyContinuationContext(ctx, r, w, p, needsInput, func(facts b.LifecycleRecoveryEvidence, err error) error {
			return errors.Join(err, errors.New("The original session or prior writers cannot be verified for resume: "+facts.Diagnostic))
		})
		w.gate.Lock()
		w.queryCancel = nil
		cancel()
		if err != nil {
			e.failControl(id, command, err)
			return false
		}
	}
	e.mu.Lock()
	current := e.runs[id].Clone()
	e.mu.Unlock()
	if current.Revision != r.Revision || current.PendingResume != command || current.PendingStop != "" || current.Closed || e.ctx.Err() != nil {
		return false
	}
	if manual && r.Snapshot.Ref.External.Kind == "" && r.Snapshot.Session.SessionID == "" {
		e.failControl(id, command, errors.New("Original native resource is unavailable; create a new Run explicitly"))
		return false
	}
	businessPause := manual && (r.ManualPause.Status == "blocked" || r.ManualPause.Status == "waiting_input")
	businessDiagnostic := ""
	if manual {
		r.Status, r.Phase = r.ManualPause.Status, r.ManualPause.Phase
		if businessPause {
			businessDiagnostic = r.ManualPause.Diagnostic
		}
		r.ManualPause = nil
	}
	r.Allowed = true
	r.ResumeRequired = false
	r.PendingResume = ""
	r.Diagnostic = businessDiagnostic
	if r.Question != nil {
		r.Status, r.Phase = "waiting_input", "question"
		businessPause = true
	}
	r.StopStatus = ""
	if err := func() error {
		if businessPause {
			return nil
		}
		return resumeWorkflow(r)
	}(); err != nil {
		e.failControl(id, command, err)
		return false
	}
	r.CommandStates[command] = "completed"
	e.mu.Lock()
	if e.runs[id].Revision != r.Revision {
		e.mu.Unlock()
		return false
	}
	err := e.commitLocked(r, nil, "resume_qualified")
	e.mu.Unlock()
	if err == nil && manual && r.Result != nil {
		return e.finishWithGate(id, *r.Result, &w.gate)
	}
	return err == nil
}
func (e *Engine) stopRun(id string, w *worker, p b.LifecyclePort) bool {
	w.gate.Lock()
	defer w.gate.Unlock()
	collect := func(ctx context.Context) {
		w.gate.Unlock()
		defer w.gate.Lock()
		e.sealResult(ctx, id)
	}
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	// An independently dispatched creation may finish after task revocation.
	// Recover only its original receipt so stop can address the exact resource.
	// Reconciliation cannot dispatch another create or restore input authority.
	for index, intent := range r.Intents {
		if intent.NeedsReconciliation() && (intent.Request.Operation.Action == string(b.LifecyclePrepareWorkspace) || intent.Request.Operation.Action == string(b.LifecycleCreateResource)) {
			if e.invoke(id, w, p, index, true) {
				return true
			}
			e.mu.Lock()
			r = e.runs[id].Clone()
			e.mu.Unlock()
			if r.Snapshot.Ref.External.Kind == "" {
				return false
			}
			break
		}
	}
	command := r.PendingStop
	if r.Snapshot.Ref.External.Kind == "" && !unknown(r) && codex.ExecutionOwner(r.Snapshot) == nil {
		e.mu.Lock()
		next := e.runs[id].Clone()
		next.StopStatus, next.Status, next.Phase = "stopped", "stopped", "stopped"
		settleContentStatus(next)
		next.PendingStop = ""
		next.CommandStates[command] = "completed"
		_ = e.commitLocked(next, nil, "run_stopped")
		e.mu.Unlock()
		collect(e.ctx)
		return false
	}
	var index = -1
	for i, intent := range r.Intents {
		if intent.Request.Operation.Action == string(b.LifecycleStopExecution) && intent.Request.Operation.IdempotencyKey == command {
			index = i
		}
	}
	freshStop := index < 0
	if freshStop {
		var err error
		index, err = e.saveIntent(id, r, b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{TurnID: r.Snapshot.Session.CurrentTurnID}}})
		if err != nil {
			return false
		}
	}
	e.mu.Lock()
	r = e.runs[id].Clone()
	e.mu.Unlock()
	request := r.Intents[index].Request.Clone()
	// Auxiliary creation receipts can arrive after this stop intent was saved.
	// Import only committed opaque facts of the exact original resource; keep
	// the persisted stop request, payload and target unchanged.
	if request.Snapshot.Ref != r.Snapshot.Ref || request.Snapshot.Session.AttemptID != r.Snapshot.Session.AttemptID ||
		request.Snapshot.ResourceGeneration != r.Snapshot.ResourceGeneration || request.Snapshot.ConfigurationDigest != r.Snapshot.ConfigurationDigest {
		return false
	}
	request.Snapshot.Implementation = r.Snapshot.Clone().Implementation
	if r.StopEvidenceScope == orca.LocalCodexStopScope || r.StopEvidenceScope == orca.AgentSessionStopScope {
		request.Snapshot.Operations = r.Snapshot.Clone().Operations
	}
	stopBudget := 15 * time.Second
	if r.Request.Execution.Adapter == "orca-terminal" && r.StopEvidenceScope != orca.LocalCodexStopScope {
		// One original stop needs multiple serialized CLI identity/closure
		// queries. Allow the shared queue without timing out before its first
		// close. This remains finite and is cancelled by service shutdown.
		stopBudget = 60 * time.Second
	}
	if r.Request.Execution.Adapter == "orca-terminal" && r.Request.Execution.Target.Kind == "orca_environment" {
		// Remote stop includes serialized CLI inventory, original-identity
		// inspection, close and confirmation. The former 15s outer deadline
		// could cancel a 40s CLI call after dispatch and manufacture an unknown
		// close outcome. Keep one bounded operation budget across those calls.
		stopBudget = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(e.ctx, stopBudget)
	defer cancel()
	var receipt b.LifecycleReceipt
	var err error
	if r.Intents[index].Receipt != nil || r.StopEvidenceScope != "" && !freshStop {
		if e.writersAbsent(r, w, r.Snapshot.Ref.External) {
			receipt = b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true, WritersAbsent: true}}
		} else {
			receipt, _, err = p.Reconcile(ctx, request.Clone())
		}
	} else if r.Request.Execution.Adapter == "codex-app-server" && e.launchOwner == nil {
		if w.handle == nil {
			err = errors.New("The original owned writer handle is unavailable; only identity reconciliation is allowed")
			receipt.Disposition = b.OperationUnknown
		} else {
			receipt.State, err = p.ShutdownLifecycle(ctx, r.Snapshot.Clone(), w.handle)
			receipt.Disposition = b.OperationUnknown
			if err == nil && receipt.State.Stopped {
				receipt.Disposition = b.OperationConfirmed
			}
		}
	} else {
		receipt, err = p.StopLifecycle(ctx, request.Clone(), w.handle)
	}
	defer func() {
		if w.observer != nil {
			_ = w.observer.Close(context.Background())
			w.observer = nil
		}
	}()
	// Deferred after the stop commit and unlock, before observer/resource release.
	// The original stop deadline remains in force throughout collection.
	defer collect(ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	prior := e.runs[id]
	next := prior.Clone()
	status, diagnostic := operation.Outcome(receipt, err)
	if next.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 && len(receipt.Implementation) > 0 {
		if mergeErr := merge(next, receipt); mergeErr != nil {
			status, diagnostic = string(b.OperationUnknown), mergeErr.Error()
		}
	}
	next.Intents[index].SettleStop(next.Snapshot.Operations, receipt, b.OperationStatus(status))
	if err == nil && receipt.State.Stopped {
		next.StopStatus, next.Status, next.Phase = "stopped", "stopped", "stopped"
		settleContentStatus(next)
		next.PendingStop = ""
		next.CommandStates[command] = "completed"
	} else {
		next.StopStatus = "unknown"
		next.Status = "reconciling"
		next.CommandStates[command] = "unknown"
		var backendErr *b.BackendError
		if errors.As(err, &backendErr) && backendErr.Code == b.BackendUnsupported {
			next.StopStatus = "unsupported"
			next.PendingStop = ""
			next.CommandStates[command] = "failed"
		}
	}
	next.Diagnostic = e.redact(next, diagnostic)
	if err == nil && receipt.State.Stopped {
		if terminal, ok := p.(*orca.Lifecycle); ok && terminal.StopEvidenceScope() != "" {
			next.Diagnostic = orca.NativeStopDiagnostic
			if next.StopEvidenceScope == orca.LocalCodexStopScope {
				next.Diagnostic = orca.LocalCodexStopDiagnostic
			}
		}
	}
	// Continue probing an unresolved stop, but only publish a new revision when
	// its durable facts change. Otherwise slow callers can never pass CAS while
	// the worker repeatedly observes the same unreachable original resource.
	if reflect.DeepEqual(next, prior) {
		return false
	}
	_ = e.commitLocked(next, nil, "stop_"+next.StopStatus)
	return false
}

func (e *Engine) failControl(id, command string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if r.PendingResume != command && r.PendingStop != "" || r.Closed {
		return
	}
	r.CommandStates[command] = "failed"
	r.Diagnostic = e.redact(r, err.Error())
	r.PendingResume = ""
	if r.PendingFeedback != nil && r.PendingFeedback.CommandID == command {
		r.PendingFeedback = nil
	}
	if r.PendingAnswer != nil && r.PendingAnswer.CommandID == command {
		r.PendingAnswer = nil
	}
	r.Status = "blocked"
	r.Phase = "control_rejected"
	_ = e.commitLocked(r, nil, "control_rejected")
}
func (e *Engine) closeWorker(id string, w *worker) {
	e.mu.Lock()
	parent := e.shutdownContext
	e.mu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	// Only a previously confirmed Run end qualifies, never mere service exit.
	// Result collection also respects the service shutdown parent's deadline.
	e.sealResult(parent, id)
	if w.observer != nil {
		_ = w.observer.Close(context.Background())
		w.observer = nil
	}
	if w.port == nil {
		return
	}
	if owner, ok := w.port.(b.LifecyclePreparationOwner); ok {
		owner.ReleasePreparation()
	}
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	facts, err := w.port.ShutdownLifecycle(ctx, r.Snapshot.Clone(), w.handle)
	cancel()
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[id].Clone()
	if err == nil && facts.Stopped {
		next.StopStatus = "stopped"
	} else if err != nil {
		e.logger.Printf("service observation detach remains unverified run=%s diagnostic=%s", id, e.redact(next, err.Error()))
	}
	if reflect.DeepEqual(next, e.runs[id]) {
		return
	}
	_ = e.commitLocked(next, nil, "service_observation_detached")
}
func (e *Engine) writersAbsent(r *runstate.Record, w *worker, identity b.ProcessIdentity) bool {
	if r.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 {
		return false
	}
	if identity == r.Snapshot.Ref.External && codexOwnerStopped(r) {
		return true
	}
	if w.handle != nil && w.handle.Identity() == identity {
		if tree, ok := w.handle.(b.ProcessTreeHandle); ok {
			return tree.WritersAbsent()
		}
	}
	// Managed execution Jobs have non-inheritable kill-on-close handles owned
	// only by their recorded service. Prove that owner exited as well as the
	// exact root before relying on the Job's enforced tree teardown.
	return managedRecordWritersAbsent(r, identity, func(id *b.ProcessIdentity) bool {
		return process.NewServiceFacts().Probe(id).State == "absent"
	})
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
