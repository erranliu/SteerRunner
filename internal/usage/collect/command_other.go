//go:build !windows

package collect

import "os/exec"

func hideConsole(*exec.Cmd) {}
