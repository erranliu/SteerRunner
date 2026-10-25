package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
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
	e.mu.Lock()
	snapshot := e.runs[id]
	verify := snapshot.PendingPRURL != "" && snapshot.StopStatus == "" && snapshot.PendingStop == "" && snapshot.PendingResume == "" && snapshot.Allowed && !snapshot.ResumeRequired && !unknown(snapshot)
	e.mu.Unlock()
	if verify {
		return e.verifyPublication(id)
	}
	w.gate.Lock()
	defer w.gate.Unlock()
	if e.ctx.Err() != nil {
		return false
	}
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	if orca.LegacyRemoteProfile(r.Profile) && r.PendingStop == "" {
		return false
	}
	if r.PendingStop == "" {
		if err := validateOperationVersions(r); err != nil {
			e.block(id, "recovery_contract_unsupported", err, true)
			return false
		}
	}
	if handled, progress := e.advanceParser(id, w, r); handled {
		return progress
	}
	if r.Status == "pr_ready" && r.PendingResume == "" && r.PendingStop == "" && r.PendingFeedback == nil {
		return false
	}
	if r.PendingStop == "" && r.Request.ProgressFile != nil && r.Snapshot.Workspace.Path != "" && (r.Progress == nil || !r.Progress.Initialized) {
		if err := e.prepareProgress(r); err != nil {
			e.logger.Printf("progress initialization unavailable run=%s", id)
			return false
		}
		e.mu.Lock()
		r = e.runs[id].Clone()
		e.mu.Unlock()
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
			if intent.NeedsReconciliation() {
				if intent.Request.Operation.Status == b.OperationAccepted && intent.Receipt != nil && intent.Receipt.State.ResourcePending && !r.PreparationDeadline.IsZero() && time.Now().After(r.PreparationDeadline) {
					e.block(id, "preparation_deadline", errors.New("The original native resource did not become ready within its preparation budget"), false)
				}
				return e.invoke(id, w, p, i, true)
			}
		}
	}
	if r.PendingResume != "" {
		if err := validateWorkflow(r); err != nil {
			e.failControl(id, r.PendingResume, err)
			return false
		}
		return e.resumeRun(id, w, p)
	}
	if err := validateWorkflow(r); err != nil {
		e.block(id, "workflow_configuration", err, false)
		return false
	}
	if r.PR == nil && len(r.PRClues) > 0 && r.StopStatus == "" && !time.Now().Before(r.PRClueProbeAt) {
		return e.probePRClues(id)
	}
	if r.PendingAnswer != nil && r.Allowed && !r.ResumeRequired {
		return e.answer(id, w, p)
	}
	if r.PendingInterrupt != "" && r.Allowed && !r.ResumeRequired {
		found := false
		for _, intent := range r.Intents {
			if intent.Request.Operation.IdempotencyKey == r.PendingInterrupt {
				found = true
			}
		}
		if !found {
			return e.interruptContent(id, w, p, r)
		}
	}
	if r.Snapshot.Session.PendingTurnID != "" || r.Snapshot.Session.CurrentTurnID != "" && r.Snapshot.Session.CurrentTurnID != r.Snapshot.Session.LastStoppedTurnID {
		return e.attachObservation(id, w, p, r)
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
	return e.prepareRun(id, w, p, r)
}

func (e *Engine) saveIntent(id string, r *runstate.Record, candidate b.LifecycleCandidate) (int, error) {
	intent, err := operation.New(id, r.Snapshot, candidate, contracts.NewID(), contracts.NewID(), time.Now().UTC())
	if err != nil {
		return 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if current.Revision != r.Revision || current.Snapshot.Session.AttemptID != r.Snapshot.Session.AttemptID || b.LifecycleBindingDigest(current.Snapshot) != b.LifecycleBindingDigest(r.Snapshot) {
		return 0, reliable.ErrStale
	}
	next := current.Clone()
	next.PreparationDeadline = r.PreparationDeadline
	snapshotOperation := intent.Request.Operation
	events := []string{"operation_committed"}
	if candidate.Action == b.LifecycleStopExecution {
		// Fix the command association in the same transaction as the intent.
		// Keep the snapshot's existing independent key representation.
		intent.Request.Operation.IdempotencyKey = current.PendingStop
		events = append(events, "stop_intent_bound")
	}
	next.Intents = append(next.Intents, intent)
	if candidate.Action == b.LifecycleStopObservation && current.PendingInterrupt != "" {
		next.Intents[len(next.Intents)-1].Request.Operation.IdempotencyKey = current.PendingInterrupt
	}
	next.Snapshot.Operations = append(next.Snapshot.Operations, snapshotOperation)
	next.Phase = string(candidate.Action)
	if err := e.commitLocked(next, nil, events...); err != nil {
		return 0, err
	}
	return len(next.Intents) - 1, nil
}
func (e *Engine) invoke(id string, w *worker, p b.LifecyclePort, index int, reconcile bool) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	intent := r.Intents[index]
	mode := operation.ExecuteNewIntent
	if reconcile {
		mode = operation.ReconcileOriginalIntent
	}
	if !reconcile && !r.Allowed && intent.Request.Operation.Action != string(b.LifecycleStopExecution) {
		return false
	}
	request := intent.Request.Clone()
	if reconcile {
		// The operation and its input stay immutable. Reconciliation needs the
		// committed facts returned by that operation (accepted launch, original
		// native resource/session), rather than its pre-effect observation.
		if request.Snapshot.Session.AttemptID != r.Snapshot.Session.AttemptID || request.Snapshot.ResourceGeneration != r.Snapshot.ResourceGeneration || request.Snapshot.ConfigurationDigest != r.Snapshot.ConfigurationDigest {
			return false
		}
		request.Snapshot = r.Snapshot.Clone()
	}
	ctx, cancel := context.WithTimeout(e.ctx, 90*time.Second)
	// Clone normalizes nil/empty implementation lists on both sides. Storage
	// round trips may change that representation without changing any identity.
	if intent.PreparationQuery != nil && intent.Receipt == nil && len(intent.Request.Operation.Receipt) == 0 && r.WorkspaceFact == nil && r.Snapshot.Workspace.Path == "" && b.LifecycleBindingDigest(r.Snapshot.Clone()) == b.LifecycleBindingDigest(intent.Request.Snapshot.Clone()) {
		ctx = b.WithPreparationRecovery(ctx, r.Allowed && !r.ResumeRequired && r.PendingStop == "")
	}
	if !reconcile && request.Input.Turn != nil {
		if w.ownedInputs == nil {
			w.ownedInputs = map[string]bool{}
		}
		w.ownedInputs[request.Input.Turn.CommandID] = true
	}
	result := operation.Invoke(ctx, p, p, request, mode)
	cancel()
	// Shutdown cancels this read, not the durable Run authorization. Keep the
	// original in-flight operation so the next eligible generation can continue.
	if e.ctx.Err() != nil && errors.Is(result.Err, e.ctx.Err()) && result.Receipt.Disposition == b.OperationRejected {
		e.mu.Lock()
		query := e.runs[id].Intents[index].PreparationQuery
		e.mu.Unlock()
		if query != nil {
			return false
		}
	}
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
	status, diagnostic := operation.Outcome(result.Receipt, result.Err)
	if result.Uncertain {
		status = string(b.OperationUnknown)
	}
	if err := merge(next, result.Receipt); err != nil {
		status = string(b.OperationUnknown)
		diagnostic = "Native receipt did not match the original execution identity"
	}
	savedReceipt := result.Receipt.Clone()
	savedReceipt.Operation.Diagnostic = e.redact(next, savedReceipt.Operation.Diagnostic)
	next.Intents[index].Settle(next.Snapshot.Operations, savedReceipt, mode, b.OperationStatus(status), e.redact(next, diagnostic), time.Now().UTC())
	savedReceipt = *next.Intents[index].Receipt
	op := next.Intents[index].Request.Operation
	preparationStopped := next.PendingStop != "" && next.Intents[index].PreparationQuery != nil
	if !preparationStopped {
		next.Diagnostic = op.Diagnostic
	}
	resourcePending := status == string(b.OperationAccepted) && savedReceipt.State.ResourcePending
	if reconcile && (status == string(b.OperationUnknown) || resourcePending) && intent.Request.Operation.Status == b.OperationStatus(status) && intent.Request.Operation.Diagnostic == op.Diagnostic && b.LifecycleBindingDigest(next.Snapshot) == b.LifecycleBindingDigest(r.Snapshot) {
		e.mu.Unlock()
		return false
	}
	progress := status == "confirmed" || status == "accepted"
	if intent.Request.Operation.Action == string(b.LifecycleStopObservation) && next.PendingInterrupt != "" && intent.Request.Operation.IdempotencyKey == next.PendingInterrupt {
		command := next.PendingInterrupt
		switch status {
		case "rejected":
			next.CommandStates[command] = "failed"
			next.PendingInterrupt = ""
		case "confirmed", "accepted":
			next.CommandStates[command] = "running"
		default:
			next.CommandStates[command] = "unknown"
		}
	}
	if !progress && !preparationStopped {
		next.Status = "reconciling"
		if status == "rejected" {
			next.Status = "blocked"
			next.Allowed, next.ResumeRequired = false, true
		}
		if status == "rejected" && next.Request.Generate != nil {
			next.ContentResult = &contracts.ContentResult{Status: "failed", Diagnostic: next.Diagnostic}
			closeContent(next, "generate:rejected:"+op.ID)
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
		return e.finishWithGate(id, *savedReceipt.TurnEvidence, &w.gate)
	}
	return progress && !resourcePending
}

func merge(r *runstate.Record, receipt b.LifecycleReceipt) error {
	preserveObservation := r.Status == "running" && r.Phase == observationUnverified &&
		receipt.State.TurnID == r.Snapshot.Session.CurrentTurnID
	snapshot, started, err := reliable.ApplySessionReceipt(r.Snapshot, r.WorkspaceFact, receipt, r.NativeStarted)
	if err != nil {
		return err
	}
	r.Snapshot, r.NativeStarted = snapshot, started
	if receipt.Workspace != nil {
		f := *receipt.Workspace
		r.WorkspaceFact = &f
	}
	if receipt.State.TurnID != "" && receipt.State.InputAccepted && !preserveObservation {
		r.Status = "running"
		r.Phase = "observing"
		if !r.NativeStarted {
			r.Phase = "input_accepted_waiting_start"
		}
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
	if r.Request.Generate != nil && !uncertain && !unknown(r) && !r.Closed {
		r.ContentResult = &contracts.ContentResult{Status: "failed", Diagnostic: message}
		closeContent(r, "generate:blocked:"+phase)
	}
	if commitErr := e.commitLocked(r, nil, "execution_blocked"); commitErr != nil {
		e.logger.Printf("cannot commit blocker run=%s", id)
	}
}
func (e *Engine) redact(r *runstate.Record, text string) string {
	return redactDiagnostic(r, text)
}

func redactDiagnostic(r *runstate.Record, text string) string {
	var refs struct {
		AuthenticationRefs []string `json:"authentication_refs"`
	}
	_ = json.Unmarshal(r.Request.Publication.Settings, &refs)
	for _, name := range append(refs.AuthenticationRefs, "GH_TOKEN", "GITHUB_TOKEN", "GITEA_TOKEN") {
		if secret := os.Getenv(name); secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	if len(text) > 4096 {
		text = text[:4096]
	}
	return text
}
