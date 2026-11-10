package application

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func (e *Engine) feedback(id string, w *worker, p b.LifecyclePort) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	return e.feedbackValue(id, w, p, r.PendingFeedback, false)
}

// Stop revokes unsent follow-ups. A committed native intent remains original
// even when its receipt has not established whether the input was accepted.
func revokeDevelopmentInputs(r *runstate.Record) {
	if !r.DevelopmentMessage() {
		return
	}
	if r.PendingFeedback != nil {
		r.CommandStates[r.PendingFeedback.CommandID] = "failed"
		r.PendingFeedback = nil
	}
	if answer := r.PendingAnswer; answer != nil {
		sent := false
		for _, intent := range r.Intents {
			if feedback := intent.Request.Input.Feedback; feedback != nil && feedback.DecisionID == answer.CommandID && intent.Request.Operation.Status != b.OperationRejected {
				sent = true
				if intent.NeedsReconciliation() && r.CommandStates[answer.CommandID] != "completed" {
					r.CommandStates[answer.CommandID] = "unknown"
				}
			}
		}
		if !sent {
			r.CommandStates[answer.CommandID] = "failed"
			r.PendingAnswer = nil
		}
	}
	turn := r.Snapshot.PendingTurn
	if turn == nil || turn.Purpose != "message" {
		return
	}
	input := workflow.Input(r.Workflow, turn.CommandID)
	if input == nil {
		return
	}
	for _, intent := range r.Intents {
		if nativeInputAction(b.LifecycleAction(intent.Request.Operation.Action)) && intent.Request.Input.Turn != nil && intent.Request.Input.Turn.CommandID == turn.CommandID && intent.Request.Operation.Status != b.OperationRejected {
			if intent.NeedsReconciliation() && r.CommandStates[input.FeedbackDecisionID] != "completed" {
				r.CommandStates[input.FeedbackDecisionID] = "unknown"
			}
			return
		}
	}
	r.CommandStates[input.FeedbackDecisionID] = "failed"
	r.Snapshot.PendingTurn = nil
	r.Snapshot.Session.PendingCommandID, r.Snapshot.Session.PendingTurnID = "", ""
}

func (e *Engine) feedbackValue(id string, w *worker, p b.LifecyclePort, answer *runstate.Answer, businessAnswer bool) bool {
	if answer == nil {
		return false
	}
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	if answer.Epoch != e.epoch && answer.Epoch != controlScope(r, answer.QuestionID) {
		e.block(id, "fresh_feedback_required", errors.New("Saved feedback does not match the original execution and business source"), false)
		return false
	}
	if r.DevelopmentMessage() {
		if err := queueInput(r, workflow.StageInput{StepID: workflow.StepPRReady, FeedbackDecisionID: answer.CommandID}, answer.Text); err != nil {
			e.failControl(id, answer.CommandID, err)
			return false
		}
		r.PendingFeedback = nil
		r.CommandStates[answer.CommandID] = "running"
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.runs[id].Revision != r.Revision {
			return false
		}
		return e.commitLocked(r, nil, "message_frozen") == nil
	}
	if r.Request.Workflow.ID == "conversation" {
		return e.contentMessage(id, w, p, r, answer)
	}
	if state, _ := r.Workflow.CurrentState(); businessAnswer && state.Kind == "input" && state.Continue == "" {
		// Additional material is retained as user evidence, never an authorization
		// to repeat the completed audit or to continue an ended native request.
		r.AnswerHistory = append(r.AnswerHistory, *answer)
		r.PendingAnswer = nil
		r.CommandStates[answer.CommandID] = "completed"
		r.Status = "waiting_input"
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.runs[id].Revision != r.Revision {
			return false
		}
		_ = e.commitLocked(r, nil, "business_material_retained")
		return false
	}

	if err := e.qualifyContinuation(r, w, p, true, func(b.LifecycleRecoveryEvidence, error) error {
		return errors.New("The original session is not verifiably idle and available for feedback")
	}); err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}
	step, err := workflow.FeedbackStep(r.Workflow, "implement")
	if err != nil {
		e.failControl(id, answer.CommandID, err)
		return false
	}
	r.NextStage = nil // Explicit new business work supersedes only qualified unsent input.
	stage := workflow.StageInput{StepID: step, FeedbackDecisionID: answer.CommandID}

	if err := queueInput(r, stage, answer.Text); err != nil {
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
	if e.runs[id].Revision != r.Revision {
		e.mu.Unlock()
		return false
	}
	err = e.commitLocked(r, nil, "feedback_input_frozen")
	e.mu.Unlock()
	return err == nil
}

// resumeWorkflow chooses business continuation only after control qualification.
// The caller commits this together with its authorization and command receipt.
func resumeWorkflow(r *runstate.Record) error {
	if r.NextStage != nil {
		input := *r.NextStage
		r.NextStage = nil
		if err := queueInput(r, input, ""); err != nil {
			return err
		}
	} else if state, _ := r.Workflow.CurrentState(); state.Kind == "milestone" {
		r.Status = "pr_ready"
	} else if state, _ := r.Workflow.CurrentState(); state.Kind == "input" {
		r.Status = "waiting_input"
	} else if state, _ := r.Workflow.CurrentState(); state.Kind == "parse" {
		r.Status = "running"
		r.Phase = "parser_resumed"
	} else if r.Snapshot.PendingTurn == nil && r.Result != nil && r.PendingPRURL == "" {
		// Technical resume renews control eligibility, not completed business work.
		if r.Status != "pr_ready" {
			r.Status = "blocked"
		}
	} else if r.Snapshot.PendingTurn == nil && r.Snapshot.Session.CurrentCommandID != "" && r.PendingPRURL == "" {
		if turn := r.Snapshot.Session.CurrentTurnID; turn != "" && turn != r.Snapshot.Session.LastStoppedTurnID {
			// Accepted input is consumed, but its original completion may still be
			// arriving after restart. Renew observation; never queue its prompt.
			r.Status, r.Phase = "reconciling", "original_result_pending"
			r.Diagnostic = "Awaiting original execution evidence; no input was resent"
		} else {
			r.Status = "blocked"
			r.Diagnostic = "Consumed original input requires original evidence or explicit business feedback; resume cannot resend it"
		}
	} else if r.Status != "pr_ready" && r.Question == nil {
		r.Status = "running"
		r.Phase = "resumed"

	}
	return nil
}
