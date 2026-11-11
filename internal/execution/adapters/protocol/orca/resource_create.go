package orca

import "encoding/json"

// Creation arguments are pure operations. Callers own dispatch authorization,
// immutable plans and unknown-outcome reconciliation.
type WorktreeCreate struct {
	Repository, Name, BaseBranch, Setup string
	Agent                               *string
	NoParent                            bool
}

func WorktreeCreateArgs(request WorktreeCreate) []string {
	args := []string{"worktree", "create", "--repo", request.Repository, "--name", request.Name, "--base-branch", request.BaseBranch}
	if request.NoParent {
		args = append(args, "--no-parent")
	}
	if request.Setup != "" {
		args = append(args, "--setup", request.Setup)
	}
	args = append(args, "--json")
	if request.Agent != nil {
		args = append(args, "--agent", *request.Agent)
	}
	return args
}

type WorktreeCreation struct {
	Worktree            Worktree `json:"worktree"`
	AgentTerminalHandle string   `json:"agentTerminalHandle,omitempty"`
	StartupTerminal     Terminal `json:"startupTerminal"`
}

func DecodeWorktreeCreation(result Result) (WorktreeCreation, error) {
	var value WorktreeCreation
	err := json.Unmarshal(result.Payload, &value)
	return value, err
}
