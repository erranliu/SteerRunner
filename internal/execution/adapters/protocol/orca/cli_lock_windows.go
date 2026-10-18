//go:build windows

package orca

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
)

func lockCLIFile(ctx context.Context, path string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	file, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(file)
		return nil, err
	}
	closeHandles := func() { _ = windows.CloseHandle(file); _ = windows.CloseHandle(event) }
	var overlap windows.Overlapped
	overlap.HEvent = event
	// Queue in the kernel instead of polling FAIL_IMMEDIATELY; busy readers
	// must not repeatedly win ahead of calls that are already waiting.
	// https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-lockfileex
	err = windows.LockFileEx(file, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlap)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		canceled := make(chan struct{})
		stopCancel := context.AfterFunc(ctx, func() {
			_ = windows.CancelIoEx(file, &overlap)
			close(canceled)
		})
		var transferred uint32
		// Drain completion, including canceled I/O, before freeing OVERLAPPED
		// or its event. CancelIoEx alone is not completion proof.
		err = windows.GetOverlappedResult(file, &overlap, &transferred, true)
		if !stopCancel() {
			<-canceled
		}
	}
	if err != nil {
		closeHandles()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	release := func() { _ = windows.UnlockFileEx(file, 0, 1, 0, &overlap); closeHandles() }
	if err = ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}
