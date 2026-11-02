package operation

import (
	"errors"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// InputValue constructs new intents only. Saved operations retain their original
// input and are decoded by backend.CandidateForOperation, not rebuilt here.
func InputValue(action executionbackend.LifecycleAction, input executionbackend.LifecycleInput) (any, error) {
	if err := input.Validate(action); err != nil {
		return nil, err
	}
	switch action {
	case executionbackend.LifecyclePrepareWorkspace:
		return input.Workspace, nil
	case executionbackend.LifecycleCreateResource:
		return input.Resource, nil
	case executionbackend.LifecyclePrepareSession, executionbackend.LifecycleStartTurn, executionbackend.LifecycleSubmitInput:
		return input.Turn, nil
	case executionbackend.LifecycleDeliverFeedback:
		return input.Feedback, nil
	case executionbackend.LifecycleStopObservation, executionbackend.LifecycleStopExecution:
		return input.Stop, nil
	default:
		return nil, errors.New("unsupported lifecycle operation action")
	}
}
