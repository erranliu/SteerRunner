package main

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"time"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var cancelSync = kernel.NewProc("CancelSynchronousIo")
var getThread = kernel.NewProc("GetCurrentThreadId")
var openThread = kernel.NewProc("OpenThread")
var getTick = kernel.NewProc("GetTickCount64")

func ticks() uint64 { r, _, _ := getTick.Call(); return uint64(r) } // includes sleep

func readSnapshot(ctx context.Context, path string) ([]byte, error) {
	type result struct {
		body []byte
		err  error
	}
	done := make(chan result, 1)
	ready := make(chan syscall.Handle, 1)
	release := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		id, _, _ := getThread.Call()
		h, _, e := openThread.Call(1, 0, id)
		ready <- syscall.Handle(h)
		var b []byte
		var err error
		if h == 0 {
			err = e
		} else {
			p, _ := syscall.UTF16PtrFromString(path)
			var f syscall.Handle
			f, err = syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
			if err == nil {
				file := os.NewFile(uintptr(f), path)
				if ctx.Err() == nil {
					b, err = readAll(file)
				} else {
					err = ctx.Err()
				}
				file.Close()
			}
		}
		done <- result{b, err}
		<-release
		if h != 0 {
			syscall.CloseHandle(syscall.Handle(h))
		}
	}()
	h := <-ready
	defer close(release)
	select {
	case r := <-done:
		return r.body, r.err
	case <-ctx.Done():
		t := time.NewTicker(10 * time.Millisecond)
		defer t.Stop()
		for {
			if h != 0 {
				cancelSync.Call(uintptr(h))
			}
			select {
			case <-done:
				return nil, ctx.Err()
			case <-t.C:
			}
		}
	}
}
