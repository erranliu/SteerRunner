package orcatransport

import (
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func cliLaunchCommand(profile Profile, executable, directory string, timeout time.Duration, args ...string) b.LaunchCommand {
	command := native.CompileCLICommand(executable, profile.Args, profile.Environment, args...)
	return b.LaunchCommand{Executable: command.Executable, Args: command.Args, WorkingDirectory: directory, Timeout: timeout, BlockedEnvironment: command.BlockedEnvironment}
}
