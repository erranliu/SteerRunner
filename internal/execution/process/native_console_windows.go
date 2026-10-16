//go:build windows

package process

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"golang.org/x/sys/windows"
	"strings"
	"syscall"
	"unsafe"
)

// OpenNativeConsole gives the viewer its own console and standard handles.
// No worker Job or stop handle owns this interactive viewer window.
func OpenNativeConsole(ctx context.Context, command b.LaunchCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	executable, err := windows.UTF16PtrFromString(command.Executable)
	if err != nil {
		return err
	}
	directory, err := windows.UTF16PtrFromString(command.WorkingDirectory)
	if err != nil {
		return err
	}
	args := []string{syscall.EscapeArg(command.Executable)}
	for _, arg := range command.Args {
		args = append(args, syscall.EscapeArg(arg))
	}
	line, err := windows.UTF16PtrFromString(strings.Join(args, " "))
	if err != nil {
		return err
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var child windows.ProcessInformation
	if err := windows.CreateProcess(executable, line, nil, nil, false, windows.CREATE_NEW_CONSOLE, nil, directory, &startup, &child); err != nil {
		return err
	}
	windows.CloseHandle(child.Thread)
	windows.CloseHandle(child.Process)
	return nil
}
