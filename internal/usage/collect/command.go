package collect

import (
	"context"
	"os/exec"
)

// outputCLI keeps background metadata queries off the user's desktop.
// The caller retains CLI serialization and its original deadline.
func outputCLI(ctx context.Context, executable string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, executable, args...)
	hideConsole(command)
	return command.Output()
}
