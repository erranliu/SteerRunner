//go:build !windows

package textanalysis

import (
	"errors"
	"os/exec"
)

func start(*exec.Cmd) (func(), error) { return nil, errors.New("parser CLI supported only on Windows") }
