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

// preservePendingStage retains business input before the original session's
// resource binding is restored. It does not authorize or dispatch the input.
func preservePendingStage(r *runstate.Record) {
	if r.NextStage == nil && r.Snapshot.PendingTurn != nil {
		if prior := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID); prior != nil {
			copy := *prior
			copy.CommandID = ""
			copy.Prompt = ""
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
	} else if r.Status != "pr_ready" && r.Question == nil && r.Snapshot.PendingTurn == nil && r.Result != nil {
		step, retry, err := workflow.FeedbackStep(r.Workflow, workflow.FeedbackIntent{Continue: true, Option: "implement"})
		if err != nil {
			return err
		}
		r.Workflow.ReportRetry = retry
		if err := queueInput(r, workflow.StageInput{StepID: step}, ""); err != nil {
			return err
		}
	} else if r.Status != "pr_ready" && r.Question == nil {
		r.Status = "running"
		r.Phase = "resumed"
	}
	return nil
}
