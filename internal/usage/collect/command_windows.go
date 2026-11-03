package collect

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
)

func hideConsole(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
