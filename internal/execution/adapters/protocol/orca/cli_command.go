package orca

// CLICommand is a compiled Orca command, independent of any process executor.
// Callers retain their working directory, deadline and dispatch authorization.
type CLICommand struct {
	Executable         string
	Args               []string
	BlockedEnvironment []string
}

// CompileCLICommand appends the explicit destination after operation arguments.
// The returned slices are owned by this command and never alias caller input.
func CompileCLICommand(executable string, prefix []string, environment string, args ...string) CLICommand {
	all := append(append([]string(nil), prefix...), args...)
	if environment != "" {
		all = append(all, "--environment", environment)
	}
	return CLICommand{Executable: executable, Args: all, BlockedEnvironment: []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}}
}
