package application

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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
	input := b.LifecycleInput{Feedback: &b.FeedbackInput{DecisionID: saved.CommandID, QuestionID: saved.QuestionID, Message: saved.Text, Answer: &b.Answer{Answers: answers}}}
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
	if r.Request.Conversation != nil || r.Request.Generate != nil {
		if err := verifyContentDirectory(r); err != nil {
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

		// Idle restoration is deferred until input is needed; parse completion
		// may select an existing fallback after resume qualification.
		needsInput := r.NextStage != nil || r.Snapshot.PendingTurn != nil
		if state, _ := r.Workflow.CurrentState(); state.Kind == "parse" {
			needsInput = true
		}
		if err := e.qualifyContinuation(r, w, p, needsInput, func(facts b.LifecycleRecoveryEvidence, err error) error {
			return errors.Join(err, errors.New("The original session or prior writers cannot be verified for resume: "+facts.Diagnostic))
		}); err != nil {
			e.failControl(id, command, err)
			return false
		}
	}
	r.Allowed = true
	r.ResumeRequired = false
	r.PendingResume = ""
	r.Diagnostic = ""
	r.StopStatus = ""
	if err := resumeWorkflow(r); err != nil {
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
	return err == nil
}
func (e *Engine) stopRun(id string, w *worker, p b.LifecyclePort) bool {
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
	if r.Snapshot.Ref.External.Kind == "" && !unknown(r) {
		e.mu.Lock()
		next := e.runs[id].Clone()
		next.StopStatus, next.Status, next.Phase = "stopped", "stopped", "stopped"
		settleContentStatus(next)
		next.PendingStop = ""
		next.CommandStates[command] = "completed"
		_ = e.commitLocked(next, nil, "run_stopped")
		e.mu.Unlock()
		return false
	}
	var index = -1
	for i, intent := range r.Intents {
		if intent.Request.Operation.Action == string(b.LifecycleStopExecution) && intent.Request.Operation.IdempotencyKey == command {
			index = i
		}
	}
	if index < 0 {
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
	ctx, cancel := context.WithTimeout(e.ctx, 15*time.Second)
	defer cancel()
	var receipt b.LifecycleReceipt
	var err error
	if r.Intents[index].Receipt != nil {
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
	if w.observer != nil {
		_ = w.observer.Close(context.Background())
		w.observer = nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	prior := e.runs[id]
	next := prior.Clone()
	status, diagnostic := operation.Outcome(receipt, err)
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

// Called only after original-session/idle/writer-absence verification.
func (e *Engine) restoreOriginalAttempt(r *runstate.Record, w *worker, p b.LifecyclePort) error {
	previous := r.Snapshot.Clone()
	r.Attempts = append(r.Attempts, runstate.Attempt{Snapshot: previous, EndedAt: time.Now().UTC()})
	preservePendingStage(r)
	r.Snapshot.Ref.AttemptID = contracts.NewID()
	r.Snapshot.Ref.External = b.ProcessIdentity{}
	r.Snapshot.Ref.Generation = ""
	r.Snapshot.ResourceGeneration = contracts.NewID()
	r.Snapshot.Session.AttemptID = r.Snapshot.Ref.AttemptID
	r.Snapshot.Session.AttemptNumber++
	r.Snapshot.Session.Generation = ""
	r.Snapshot.Session.LastStoppedTurnID = r.Snapshot.Session.CurrentTurnID
	r.Snapshot.PendingTurn = nil
	r.Snapshot.Session.PendingCommandID = ""
	r.Snapshot.Session.PendingTurnID = ""
	r.Snapshot.Operations = nil
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	binding, err := p.BindRecovery(ctx, previous, r.Snapshot.Clone())
	cancel()
	if err != nil {
		return err
	}
	r.Snapshot.Implementation = []b.ImplementationDataEnvelope{binding}
	r.Snapshot.QuestionRoute = nil
	r.Snapshot.PolicyResources = nil
	if owner, ok := w.port.(b.LifecyclePreparationOwner); ok {
		owner.ReleasePreparation()
	}
	w.port = nil
	w.handle = nil
	return nil
}
func (e *Engine) failControl(id, command string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
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
	e.stopParserForShutdown(id, w)
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
	if err == nil && facts.Stopped && parserResourcesSettled(next) {
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
