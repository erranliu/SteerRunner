//go:build !windows

package process

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os/exec"
)

func StartManagedBound(cmd *exec.Cmd, ready func(b.ProcessIdentity) error) (b.StopHandle, error) {
	return nil, errors.New("independent parser profile is qualified only on Windows")
}
