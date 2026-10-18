package contracts

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

// ExecutionModel reads public preferences without resolving any host resources.
func ExecutionModel(e Execution) (workflow.ModelSettings, error) {
	switch e.Adapter {
	case "codex-app-server":
		var s CodexSettings
		if err := DecodeStrict(e.Settings, &s); err != nil {
			return workflow.ModelSettings{}, err
		}
		return workflow.ModelSettings{Model: s.Model, ReasoningEffort: s.ReasoningEffort}, nil
	case "orca-terminal":
		var s OrcaSettings
		if err := DecodeStrict(e.Settings, &s); err != nil {
			return workflow.ModelSettings{}, err
		}
		if s.AgentName() != "codex" || s.AgentLaunch.RuntimeExecutable == "" {
			return workflow.ModelSettings{}, errors.New("auditing requires the managed execution-host runtime")
		}
		return workflow.ModelSettings{Model: s.AgentLaunch.Model, ReasoningEffort: s.AgentLaunch.ReasoningEffort}, nil
	}
	return workflow.ModelSettings{}, errors.New("unsupported model adapter")
}
