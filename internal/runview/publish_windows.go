package runview

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var cancelSync = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")
var replaceFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

// Publish waits for the actual operation to finish even after cancellation.
// MoveFileEx cannot be cancelled by Windows. In that short section cancellation
// means "do not start another operation", never "the rename did not happen".
// https://learn.microsoft.com/windows/win32/fileio/canceling-pending-i-o-operations
func Publish(ctx context.Context, path string, body []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	threadReady := make(chan windows.Handle, 1)
	release := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		thread, err := windows.OpenThread(windows.THREAD_TERMINATE, false, windows.GetCurrentThreadId())
		threadReady <- thread
		if err == nil {
			err = publishFile(ctx, path, body)
		}
		done <- err
		<-release // thread cannot be reused while a cancellation targets it
		if thread != 0 {
			windows.CloseHandle(thread)
		}
	}()
	thread := <-threadReady
	defer close(release)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Retry cancellation across the open/write boundary, without returning
		// a still-active writer to the host. The writer lease remains held.
		timer := time.NewTicker(10 * time.Millisecond)
		defer timer.Stop()
		for {
			if thread != 0 {
				cancelSync.Call(uintptr(thread))
			}
			select {
			case <-done:
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
}
func publishFile(ctx context.Context, path string, body []byte) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "display-*.tmp")
	if err != nil {
		return err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	from, err := syscall.UTF16PtrFromString(temporary)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		ok, _, callErr := replaceFile.Call(uintptr(unsafe.Pointer(to)), uintptr(unsafe.Pointer(from)), 0, 0, 0, 0)
		if ok == 0 {
			return callErr
		}
		return nil
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING)
}
