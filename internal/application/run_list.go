package application

import "github.com/erranliu/SteerRunner/internal/runview"

// DisplayRuns copies only immutable display values under the existing gate.
// It does not query providers, serialize prompts, or expose a new CLI command.
func (e *Engine) DisplayRuns() ([]runview.Run, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	runs := make([]runview.Run, 0, len(e.runs))
	for _, r := range e.runs {
		v := runview.Run{ID: r.ID, Title: r.Request.Title, Workflow: runview.Workflow{ID: r.Request.Workflow.ID, Version: r.Request.Workflow.Version}, Step: r.Workflow.StepID, Status: r.Status}
		if r.PR != nil {
			v.PR = &runview.PR{URL: r.PR.URL, Number: r.PR.Number}
		}
		runs = append(runs, v)
	}
	return runs, nil
}
