//go:build !windows

package process

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func OpenNativeConsole(context.Context, b.LaunchCommand) error {
	return errors.New("native viewer requires Windows")
}
