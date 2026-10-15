package application

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"strings"
	"time"
)

func (e *Engine) answer(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	saved := r.PendingAnswer
	if saved == nil {
		return false
	}
	if saved.Epoch != e.epoch {
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
func (e *Engine) feedback(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	return e.feedbackValue(id, w, p, r.PendingFeedback, false)
}
func (e *Engine) feedbackValue(id string, w *worker, p b.LifecyclePort, answer *runstate.Answer, businessAnswer bool) bool {
	if answer == nil {
		return false
	}
	if answer.Epoch != e.epoch {
		e.block(id, "fresh_feedback_required", errors.New("Saved feedback from a prior service requires a fresh user request"), false)
		return false
	}
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	facts, err := p.VerifyManualRecovery(ctx, r.Snapshot.Clone())
	cancel()
	if err != nil || !reliable.CanContinueOriginal(facts) {
		e.failControl(id, answer.CommandID, errors.New("The original session is not verifiably idle and available for feedback"))
		return false
	}
	step, retry, err := workflow.FeedbackStep(r.Workflow, workflow.FeedbackIntent{Option: "implement", Continue: true})
	if err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}
	r.Workflow.ReportRetry = retry
	if err := queueInput(r, workflow.StageInput{StepID: step, FeedbackDecisionID: answer.CommandID}, answer.Text); err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}
	r.Question = nil
	r.Snapshot.QuestionRoute = nil
	r.Snapshot.PendingQuestions = nil
	r.CommandStates[answer.CommandID] = "running"
	if businessAnswer {
		r.PendingAnswer = nil
	} else {
		r.PendingFeedback = nil
	}
	e.mu.Lock()
	err = e.commitLocked(r, nil, "feedback_input_frozen")
	e.mu.Unlock()
	return err == nil
}
func (e *Engine) resumeRun(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	command := r.PendingResume
	if unknown(r) {
		e.failControl(id, command, errors.New("Original operations require reconciliation before any resume"))
		return false
	}
	if r.Snapshot.Session.SessionID == "" && r.Snapshot.Ref.External.Kind != "" {
		e.failControl(id, command, errors.New("An original native resource exists without a verified session; resume cannot create a substitute session"))
		return false
	}
	if r.Snapshot.Session.SessionID != "" {
		ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
		facts, err := p.VerifyManualRecovery(ctx, r.Snapshot.Clone())
		cancel()
		if err != nil || !reliable.CanContinueOriginal(facts) {
			e.failControl(id, command, errors.Join(err, errors.New("The original session or prior writers cannot be verified for resume: "+facts.Diagnostic)))
			return false
		}
		if facts.ProcessAbsent {
			if err := reliable.ValidateRestoration(facts, r.Snapshot.Session.SessionID, false); err != nil {
				e.failControl(id, command, err)
				return false
			}
			previous := r.Snapshot.Clone()
			r.Attempts = append(r.Attempts, runstate.Attempt{Snapshot: previous, EndedAt: time.Now().UTC()})
			if r.NextStage == nil && r.Snapshot.PendingTurn != nil {
				if prior := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID); prior != nil {
					copy := *prior
					copy.CommandID = ""
					copy.Prompt = ""
					r.NextStage = &copy
				}
			}
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
			ctx, cancel = context.WithTimeout(e.ctx, 10*time.Second)
			binding, err := p.BindRecovery(ctx, previous, r.Snapshot.Clone())
			cancel()
			if err != nil {
				e.failControl(id, command, err)
				return false
			}
			r.Snapshot.Implementation = []b.ImplementationDataEnvelope{binding}
			r.Snapshot.QuestionRoute = nil
			r.Snapshot.PolicyResources = nil
			if owner, ok := w.port.(b.LifecyclePreparationOwner); ok {
				owner.ReleasePreparation()
			}
			w.port = nil
			w.handle = nil
		}
	}
	r.Allowed = true
	r.ResumeRequired = false
	r.PendingResume = ""
	r.Diagnostic = ""
	r.StopStatus = ""
	if r.NextStage != nil {
		input := *r.NextStage
		r.NextStage = nil
		if err := queueInput(r, input, ""); err != nil {
			e.failControl(id, command, err)
			return false
		}
	} else if r.Status != "pr_ready" && r.Question == nil && r.Snapshot.PendingTurn == nil && r.Result != nil {
		step, retry, err := workflow.FeedbackStep(r.Workflow, workflow.FeedbackIntent{Continue: true, Option: "implement"})
		if err != nil {
			e.failControl(id, command, err)
			return false
		}
		r.Workflow.ReportRetry = retry
		if err := queueInput(r, workflow.StageInput{StepID: step}, ""); err != nil {
			e.failControl(id, command, err)
			return false
		}
	} else if r.Status != "pr_ready" && r.Question == nil {
		r.Status = "running"
		r.Phase = "resumed"
	}
	r.CommandStates[command] = "completed"
	e.mu.Lock()
	err := e.commitLocked(r, nil, "resume_qualified")
	e.mu.Unlock()
	return err == nil
}
func (e *Engine) stopRun(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	command := r.PendingStop
	if r.Snapshot.Ref.External.Kind == "" && !unknown(r) {
		e.mu.Lock()
		next := e.runs[id].Clone()
		next.StopStatus, next.Status, next.Phase = "stopped", "stopped", "stopped"
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
		e.mu.Lock()
		r = e.runs[id].Clone()
		r.Intents[index].Request.Operation.IdempotencyKey = command
		_ = e.commitLocked(r, nil, "stop_intent_bound")
		e.mu.Unlock()
	}
	e.mu.Lock()
	r = e.runs[id].Clone()
	e.mu.Unlock()
	request := r.Intents[index].Request
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
	} else if r.Request.Execution.Adapter == "codex-app-server" {
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
	next := e.runs[id].Clone()
	status, diagnostic := reliable.Outcome(receipt, err)
	next.Intents[index].Request.Operation.Status = b.OperationStatus(status)
	receipt.Operation = request.Operation
	next.Intents[index].Receipt = &receipt
	for i := range next.Snapshot.Operations {
		if next.Snapshot.Operations[i].ID == request.Operation.ID {
			next.Snapshot.Operations[i].Status = b.OperationStatus(status)
		}
	}
	if err == nil && receipt.State.Stopped {
		next.StopStatus, next.Status, next.Phase = "stopped", "stopped", "stopped"
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
	_ = e.commitLocked(next, nil, "stop_"+next.StopStatus)
	return false
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
	next.Allowed = false
	next.ResumeRequired = true
	if err == nil && facts.Stopped {
		next.StopStatus = "stopped"
	} else if err != nil {
		next.Diagnostic = e.redact(next, err.Error())
	}
	_ = e.commitLocked(next, nil, "service_observation_detached")
}
func (e *Engine) writersAbsent(r *runstate.Record, w *worker, identity b.ProcessIdentity) bool {
	if w.handle != nil && w.handle.Identity() == identity {
		if tree, ok := w.handle.(b.ProcessTreeHandle); ok {
			return tree.WritersAbsent()
		}
	}
	// Managed execution Jobs have non-inheritable kill-on-close handles owned
	// only by their recorded service. Prove that owner exited as well as the
	// exact root before relying on the Job's enforced tree teardown.
	if !r.NativeManaged || r.NativeOwner == nil {
		return false
	}
	if process.NewServiceFacts().Probe(r.NativeOwner).State != "absent" {
		return false
	}
	return process.InspectProcess(&identity) == "process absent"
}

// Validate the complete native question set before persisting any input intent.
func nativeAnswers(questions []b.Question, text string) (map[string][]string, error) {
	answers := map[string][]string{}
	if len(questions) == 0 {
		return nil, errors.New("Original native questions are missing")
	}
	if len(questions) == 1 {
		answers[questions[0].ID] = []string{text}
	} else if contracts.DecodeStrict([]byte(text), &answers) != nil {
		return nil, errors.New("Multiple questions require unambiguous JSON keyed by original question ID")
	}
	if len(answers) != len(questions) {
		return nil, errors.New("Answer must cover exactly the original native questions")
	}
	seen := map[string]bool{}
	for _, q := range questions {
		if q.ID == "" || seen[q.ID] {
			return nil, errors.New("Original native question identity is ambiguous")
		}
		seen[q.ID] = true
		values, ok := answers[q.ID]
		if !ok || len(values) == 0 || len(values) > 2 || len(values) > 1 && !q.AllowFreeText {
			return nil, errors.New("Answer differs from the original native question")
		}
		for i, value := range values {
			if strings.TrimSpace(value) == "" {
				return nil, errors.New("Native answer cannot be empty")
			}
			offered := false
			for _, option := range q.Options {
				if value == option.ID || value == option.Label {
					values[i] = option.Label
					offered = true
					break
				}
			}
			if !offered && !q.AllowFreeText {
				return nil, errors.New("Answer is not an offered option")
			}
		}
	}
	return answers, nil
}
