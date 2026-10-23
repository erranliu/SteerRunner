package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestSlowReadCancellationAndIndependentSource(t *testing.T) {
	// A private named pipe provides a real pending synchronous ReadFile without
	// changing filesystem permissions or adding production I/O fault switches.
	name := fmt.Sprintf(`\\.\pipe\steerrunner-viewer-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	p, _ := syscall.UTF16PtrFromString(name)
	h, _, err := kernel.NewProc("CreateNamedPipeW").Call(uintptr(unsafe.Pointer(p)), 2, 0, 1, 4096, 4096, 0, 0)
	if h == ^uintptr(0) {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	connected := make(chan error, 1)
	go func() {
		ok, _, e := kernel.NewProc("ConnectNamedPipe").Call(h, 0)
		if ok == 0 && e != syscall.Errno(535) { // ERROR_PIPE_CONNECTED
			connected <- e
			return
		}
		connected <- nil
	}()
	ctx, cancelRead := context.WithCancel(context.Background())
	defer cancelRead()
	done := make(chan error, 1)
	go func() { _, err := readSnapshot(ctx, name); done <- err }()
	select {
	case err := <-connected:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("private pending read never connected")
	}
	path := filepath.Join(t.TempDir(), "other-source.json")
	if err := os.WriteFile(path, []byte("independent"), 0600); err != nil {
		t.Fatal(err)
	}
	other, cancelOther := context.WithTimeout(context.Background(), time.Second)
	defer cancelOther()
	if body, err := readSnapshot(other, path); err != nil || string(body) != "independent" {
		t.Fatal("slow source blocked the other source", string(body), err)
	}
	cancelRead()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("pending read did not return cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("original read did not settle after cancellation")
	}
}
