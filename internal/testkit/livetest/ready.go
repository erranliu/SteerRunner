package livetest

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
)

// ObservePublicationReady reads the persisted Run first. A completed feedback
// workflow can still contain the previous provider snapshot; explicitly refresh
// only that already-bound PR. This never publishes, resumes or sends model input.
func ObservePublicationReady(oldHead string, inspect func(bool) (contracts.RunView, error)) (contracts.RunView, error) {
	view, err := inspect(false)
	if err != nil || oldHead == "" || view.Status != "pr_ready" || view.PR == nil || view.PR.HeadRevision != oldHead {
		return view, err
	}
	fresh, err := inspect(true)
	if err != nil {
		return view, err
	}
	a, z := view.PR, fresh.PR
	if z == nil || fresh.RunID != view.RunID || z.URL != a.URL || z.Provider != a.Provider || z.Repository != a.Repository || z.Number != a.Number || z.BaseBranch != a.BaseBranch || z.HeadRepository != a.HeadRepository || z.HeadBranch != a.HeadBranch || fresh.Execution.SessionID != view.Execution.SessionID || fresh.Execution.AttemptID != view.Execution.AttemptID || fresh.Execution.Workspace != view.Execution.Workspace || fresh.Execution.Branch != view.Execution.Branch {
		return view, errors.New("readonly feedback refresh changed the original PR or execution identity")
	}
	return fresh, nil
}
