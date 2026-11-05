//go:build !windows

package textanalysis

import (
	"github.com/erranliu/SteerRunner/internal/metadataprocess"
	"os/exec"
)

func start(cmd *exec.Cmd) (func(), error) { return metadataprocess.Start(cmd) }
