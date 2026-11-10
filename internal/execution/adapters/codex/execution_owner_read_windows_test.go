//go:build windows

package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"golang.org/x/sys/windows"
)

func TestExecutionOwnerTransientReadSharingKeepsOriginalDispatch(t *testing.T) {
	for _, file := range []string{ExecutionOwnerConfigFile, "owner-ready.json"} {
		t.Run(file, func(t *testing.T) {
			l, request, calls := sharingOwnerFixture(t, false)
			unlock := lockOwnerFile(t, filepath.Join(l.ownerDirectory(), file))
			result := make(chan error, 1)
			go func() { _, err := l.ownerCall(context.Background(), request, false); result <- err }()
			select {
			case err := <-result:
				t.Fatalf("transient read collision ended original dispatch: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			if len(calls) != 0 {
				t.Fatal("owner received input while readiness was unreadable")
			}
			unlock()
			select {
			case err := <-result:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("read did not recover within its budget")
			}
			select {
			case got := <-calls:
				if got.Method != "native" || got.Request.Operation.ID != request.Operation.ID || ownerRequestDigest(got.Request) != ownerRequestDigest(request) {
					t.Fatal("original dispatch identity changed")
				}
			default:
				t.Fatal("original owner received no request")
			}
			if len(calls) != 0 {
				t.Fatal("original request was resent")
			}
		})
	}
}

func TestExecutionOwnerReadSharingCannotBypassCancellationBudgetOrIdentity(t *testing.T) {
	for _, mode := range []string{"cancelled", "cancel-during-read", "persistent", "foreign-ready"} {
		t.Run(mode, func(t *testing.T) {
			l, request, calls := sharingOwnerFixture(t, mode == "foreign-ready")
			unlock := lockOwnerFile(t, filepath.Join(l.ownerDirectory(), "owner-ready.json"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			if mode == "cancel-during-read" {
				time.AfterFunc(50*time.Millisecond, cancel)
			}
			if mode == "foreign-ready" {
				time.AfterFunc(50*time.Millisecond, unlock)
			}
			start := time.Now()
			_, err := l.ownerCall(ctx, request, false)
			if err == nil {
				t.Fatal("unavailable or foreign readiness accepted")
			}
			if (mode == "cancelled" || mode == "cancel-during-read") && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if mode == "foreign-ready" && !strings.Contains(err.Error(), "identity") {
				t.Fatalf("identity check bypassed: %v", err)
			}
			if mode == "persistent" && (time.Since(start) < 450*time.Millisecond || time.Since(start) > 2*time.Second) {
				t.Fatalf("read budget not bounded: %s", time.Since(start))
			}
			if len(calls) != 0 {
				t.Fatal("failed readiness sent an owner request")
			}
		})
	}
}

func sharingOwnerFixture(t *testing.T, foreign bool) (ExecutionLifecycle, b.LifecycleRequest, <-chan ownerQuery) {
	t.Helper()
	l := ExecutionLifecycle{Backend: Backend{Config: Config{ServerDir: t.TempDir()}, Process: ProcessPort{Inspect: func(*b.ProcessIdentity) string { return "matching process still exists" }}}}
	dir := l.ownerDirectory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	sha := "original-sha"
	if foreign {
		sha = "foreign-sha"
	}
	for name, value := range map[string]any{ExecutionOwnerConfigFile: ownerConfig{Token: "original", RuntimeSHA256: "original-sha"}, "owner-ready.json": ownerReady{Identity: ExecutionOwnerIdentity{Protocol: ExecutionOwnerProtocol, Directory: dir, RuntimeSHA256: sha}, Endpoint: listener.Addr().String()}} {
		if err := os.WriteFile(filepath.Join(dir, name), encode(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := make(chan ownerQuery, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(3 * time.Second))
				var q ownerQuery
				if json.NewDecoder(conn).Decode(&q) != nil {
					return
				}
				calls <- q
				json.NewEncoder(conn).Encode(ownerResponse{Receipt: b.LifecycleReceipt{Operation: q.Request.Operation, Disposition: b.OperationConfirmed}})
			}()
		}
	}()
	return l, ownerTestRequest(t), calls
}

func lockOwnerFile(t *testing.T, path string) func() {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Prove this is the same Windows sharing error seen in the preserved Run.
	if _, err = os.ReadFile(path); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		windows.CloseHandle(h)
		t.Fatalf("fixture did not cause sharing violation: %v", err)
	}
	// Cleanup and timed release may race; close the original handle exactly once.
	var once sync.Once
	release := func() { once.Do(func() { windows.CloseHandle(h) }) }
	t.Cleanup(release)
	return release
}
