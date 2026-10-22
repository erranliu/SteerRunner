package application

import "github.com/erranliu/SteerRunner/internal/contracts"

// HandleUsageTarget only checks the in-memory target. It does not collect, mutate
// the Run, acquire worker dispatch gates, or initialize pricing.
func (e *Engine) HandleUsageTarget(run string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.runs[run] == nil {
		return false, contracts.Fail(5, "run_not_found", "Run was not found.")
	}
	return true, nil
}

// StopObserving begins the existing service shutdown without waiting for usage
// resource cleanup. It revokes this writer's observation, not Run authorization.
func (e *Engine) StopObserving() { e.cancel() }
