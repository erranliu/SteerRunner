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

type TerminalCreate struct {
	Worktree, Title string
	Command         *string
}

func TerminalCreateArgs(request TerminalCreate) []string {
	args := []string{"terminal", "create", "--worktree", request.Worktree, "--title", request.Title}
	if request.Command != nil {
		args = append(args, "--command", *request.Command)
	}
	return append(args, "--json")
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

type TerminalCreation struct{ Terminal Terminal }

func DecodeTerminalCreation(result Result) (TerminalCreation, error) {
	var value TerminalCreation
	err := json.Unmarshal(result.Payload, &value)
	return value, err
}
