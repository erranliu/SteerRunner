package application

import (
	"context"
	"errors"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/textanalysis"
)

// textTask is service-local work, never a durable operation or native resource.
type textTask struct {
	source         ra.Source
	implementation string
	cancel         context.CancelFunc
	done           chan struct{}
	body           string
	err            error
}

func (e *Engine) selectParser(r *runstate.Record, input workflow.StageInput, state workflow.State, transition *workflow.Transition) {
	config, err := ra.LoadConfig(e.dir)
	if err != nil {
		transition.Reason = "Independent text analysis configuration is invalid; using the original report route"
		if transition.Acquired != nil {
			transition.Acquired.Attempts = append(transition.Acquired.Attempts, ra.Attempt{Method: "llm_parser", Source: transition.Acquired.Source, Status: "failed", Diagnostic: "Invalid parser configuration"})
		}
		return
	}
	if !config.Config.CodexParser.Enabled {
		return
	}
	source := workflow.Result(r.Workflow, input.CommandID)
	parse, ok := r.Workflow.State(state.Parse)
	if !ok || parse.Kind != "parse" || source == nil || len(source.Final) > 65536 {
		return
	}
	// Report metadata belongs to the existing business source, not parser permissions.
	if original := workflow.Input(r.Workflow, input.CommandID); original != nil {
		original.ReportToolPolicy = transition.Next.ReportToolPolicy
	}
	transition.Next = nil
	transition.Cursor = state.Parse
	transition.Status = "running"
}

func analysisEligible(r *runstate.Record) bool {
	if r == nil || r.Status != "running" || !r.Allowed || r.ResumeRequired || r.PendingStop != "" || r.PendingResume != "" || r.StopStatus != "" || r.Closed {
		return false
	}
	state, ok := r.Workflow.CurrentState()
	return ok && state.Kind == "parse"
}

func analysisSource(r *runstate.Record) (workflow.StageInput, *workflow.StageResult, error) {
	for i := len(r.Workflow.Results) - 1; i >= 0; i-- {
		a := r.Workflow.Results[i]
		input := workflow.Input(r.Workflow, a.Source.CommandID)
		if input == nil {
			continue
		}
		state, ok := r.Workflow.State(input.StepID)
		if !ok || state.Parse != r.Workflow.StepID {
			continue
		}
		source := workflow.Result(r.Workflow, input.CommandID)
		if source == nil || workflow.Source(*source) != a.Source || input.ImplementationResultID != r.Workflow.ImplementationResultID {
			return workflow.StageInput{}, nil, errors.New("analysis business source changed")
		}
		if err := workflow.VerifyPrerequisites(r.Workflow, *input, state); err != nil {
			return workflow.StageInput{}, nil, err
		}
		return *input, source, nil
	}
	return workflow.StageInput{}, nil, errors.New("analysis business source missing")
}

func validateAnalysisSource(r *runstate.Record) error {
	_, _, err := analysisSource(r)
	if err == nil && (r.NextStage != nil || r.Snapshot.PendingTurn != nil) {
		return errors.New("analysis wait conflicts with native input")
	}
	return err
}

// Called only after an ordinary business transition commits, under Engine.mu.
func (e *Engine) invalidateAnalysisLocked(r *runstate.Record) {
	w := e.workers[r.ID]
	if w == nil || w.acquisition == nil {
		return
	}
	_, source, err := analysisSource(r)
	if !analysisEligible(r) || err != nil || source == nil || workflow.Source(*source) != w.acquisition.source || r.Workflow.ImplementationResultID != w.acquisition.implementation {
		w.acquisition.cancel() // signal only; never wait for a process or model
		w.acquisition = nil
	}
}

// Only the Workflow text-result wait uses this path. No native operation is made.
func (e *Engine) advanceAcquisition(id string, w *worker, _ *runstate.Record) bool {
	e.mu.Lock()
	r := e.runs[id]
	if !analysisEligible(r) || e.ctx.Err() != nil {
		e.mu.Unlock()
		return false
	}
	input, source, err := analysisSource(r)
	if err != nil {
		e.mu.Unlock()
		e.block(id, "analysis_source", err, false)
		return false
	}
	task := w.acquisition
	if task == nil {
		ctx, cancel := context.WithCancel(e.ctx)
		task = &textTask{source: workflow.Source(*source), implementation: input.ImplementationResultID, cancel: cancel, done: make(chan struct{})}
		w.acquisition = task
		parse, _ := r.Workflow.CurrentState()
		request := textanalysis.Request{Text: source.Final, Instruction: parse.Prompt, Schema: ra.ParserSchema()}
		e.mu.Unlock()
		go func() {
			defer cancel()
			config, err := ra.LoadConfig(e.dir)
			if err != nil {
				task.err = err
			} else if !config.Config.CodexParser.Enabled {
				task.err = errors.New("parser disabled")
			} else if e.analyzeText != nil {
				task.body, task.err = e.analyzeText(ctx, request, config.Config.CodexParser)
			} else {
				task.body, task.err = e.textAnalyzer.Analyze(ctx, request, config.Config.CodexParser)
			}
			close(task.done)
			e.signal(w)
		}()
		return false
	}
	select {
	case <-task.done:
	default:
		e.mu.Unlock()
		return false
	}
	if task.source != workflow.Source(*source) || task.implementation != input.ImplementationResultID {
		e.mu.Unlock()
		return false
	}
	next := r.Clone()
	state, _ := next.Workflow.State(input.StepID)
	body := "invalid independent parser result"
	if task.err == nil {
		if normalized, err := ra.NormalizeParserReport(task.body); err == nil || errors.Is(err, ra.ErrConflict) {
			body = normalized
		}
	}
	transition := workflow.RouteParsed(next.Workflow, input, body)
	if task.err != nil && transition.Acquired != nil {
		for i := range transition.Acquired.Attempts {
			if transition.Acquired.Attempts[i].Method == "llm_parser" {
				transition.Acquired.Attempts[i].Status = "failed"
				transition.Acquired.Attempts[i].Diagnostic = "CLI text analysis failed"
			}
		}
	}
	e.applyWorkflowTransition(next, input, state, transition)
	if task.err != nil {
		next.Diagnostic = "Text analysis failed; using the original report/wait route"
	}
	err = e.commitLocked(next, nil, "analysis_result_committed")
	if err == nil && w.acquisition == task {
		w.acquisition = nil
		task.cancel()
	}
	e.mu.Unlock()
	if err == nil {
		e.signal(w)
	}
	return err == nil
}
