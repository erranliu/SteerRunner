package application

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"time"
)

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
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	facts, err := p.VerifyManualRecovery(ctx, r.Snapshot.Clone())
	cancel()
	if err != nil || !reliable.CanContinueOriginal(facts) {
		e.failControl(id, answer.CommandID, errors.New("The original session is not verifiably idle and available for feedback"))
		return false
	}
	if facts.ProcessAbsent {
		if err := reliable.ValidateRestoration(facts, r.Snapshot.Session.SessionID, unknown(r)); err != nil {
			e.failControl(id, answer.CommandID, err)
			return false
		}
		if err := e.restoreOriginalAttempt(r, w, p); err != nil {
			e.failControl(id, answer.CommandID, err)
			return false
		}
	}
	option := "implement"
	if state, _ := r.Workflow.CurrentState(); businessAnswer && state.Kind == "input" && state.Continue == "" {
		option = ""
	}
	step, err := workflow.FeedbackStep(r.Workflow, workflow.FeedbackIntent{Option: option, Continue: true})
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

// preservePendingStage retains business input before the original session's
// resource binding is restored. It does not authorize or dispatch the input.
func preservePendingStage(r *runstate.Record) {
	if r.NextStage == nil && r.Snapshot.PendingTurn != nil && r.Snapshot.PendingTurn.CommandID != r.Snapshot.Session.CurrentCommandID {
		if prior := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID); prior != nil {
			copy := *prior
			copy.CommandID = ""
			r.NextStage = &copy
		}
	}
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
