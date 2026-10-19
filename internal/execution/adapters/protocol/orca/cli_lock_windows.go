//go:build windows

package orca

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"runtime"
	"strings"
)

func cliMutexName(path string) string {
	key := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(path))))
	return fmt.Sprintf("Global\\SteerRunner.OrcaCLI.%x", key[:16])
}

func lockCLIFile(ctx context.Context, path string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Windows mutex ownership belongs to a thread, not a Go goroutine or an
	// inherited file handle. Keep this goroutine on its owning thread until release.
	runtime.LockOSThread()
	fail := func(err error) (func(), error) { runtime.UnlockOSThread(); return nil, err }
	name, err := windows.UTF16PtrFromString(cliMutexName(path))
	if err != nil {
		return fail(err)
	}
	mutex, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return fail(err)
	}
	canceled, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(mutex)
		return fail(err)
	}
	closeHandles := func() { _ = windows.CloseHandle(mutex); _ = windows.CloseHandle(canceled) }
	done := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() { _ = windows.SetEvent(canceled); close(done) })
	result, err := windows.WaitForMultipleObjects([]windows.Handle{mutex, canceled}, false, windows.INFINITE)
	if !stopCancel() {
		<-done
	}
	if err != nil {
		closeHandles()
		return fail(err)
	}
	// An owner killed during a CLI call abandons its mutex; Windows grants the
	// next waiter ownership. It must still revalidate original resource identity.
	if result != windows.WAIT_OBJECT_0 && result != windows.WAIT_ABANDONED {
		closeHandles()
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		return fail(fmt.Errorf("unexpected CLI mutex wait result: %d", result))
	}
	release := func() { _ = windows.ReleaseMutex(mutex); closeHandles(); runtime.UnlockOSThread() }
	if err = ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}
