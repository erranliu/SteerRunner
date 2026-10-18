package operation

import (
	"errors"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func InputValue(action executionbackend.LifecycleAction, input executionbackend.LifecycleInput) (any, error) {
	if err := input.Validate(action); err != nil {
		return nil, err
	}
	switch action {
	case executionbackend.LifecycleParseResult:
		return input.Parse, nil
	case executionbackend.LifecyclePrepareWorkspace:
		return input.Workspace, nil
	case executionbackend.LifecycleCreateResource:
		return input.Resource, nil
	case executionbackend.LifecyclePrepareSession, executionbackend.LifecycleStartTurn, executionbackend.LifecycleSubmitInput:
		return input.Turn, nil
	case executionbackend.LifecycleReportCompletion:
		return input.Completion, nil
	case executionbackend.LifecycleDeliverFeedback:
		return input.Feedback, nil
	case executionbackend.LifecycleAcknowledge:
		return input.Ack, nil
	case executionbackend.LifecycleStopObservation, executionbackend.LifecycleStopExecution:
		return input.Stop, nil
	case executionbackend.LifecycleCleanup:
		return input.Cleanup, nil
	case executionbackend.LifecycleHostAction:
		return input.Host, nil
	default:
		return nil, errors.New("unsupported lifecycle operation action")
	}
}
