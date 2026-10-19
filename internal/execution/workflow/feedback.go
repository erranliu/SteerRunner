package workflow

import "errors"

// Only a deliberate new implementation instruction can leave business input.
// Native questions are answered through their original request, outside here.
func FeedbackStep(w Snapshot, option string) (string, error) {
	state, ok := w.CurrentState()
	if !ok {
		return "", errors.New("unknown frozen state")
	}
	if state.Kind == "milestone" {
		return "", errors.New("milestone has no implementation feedback edge")
	}
	if option == "implement" {
		if state.ManualImplementation != "" {
			return state.ManualImplementation, nil
		}
		if state.Kind == "input" && state.Continue != "" {
			return state.Continue, nil
		}
		if state.Contract == "audit" {
			return w.Config.Initial, nil
		}
		if state.Contract == "implementation" {
			return w.Config.Initial, nil
		}
	}
	return "", errors.New("business continuation requires an explicit implementation instruction")
}
