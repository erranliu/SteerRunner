package host

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestLegacyOrcaStateRejectsStartupWithoutLosingOfflineEvidence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, status := range []string{"running", "stopped"} {
		t.Run(status, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "service"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "service", "writer.lock"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			run := saveStartupTestRun(t, dir, "orca-terminal", b.ProtocolOrcaTerminalV3, status)
			if err := writeShutdown(dir, contracts.ServiceShutdown{StateDir: dir, InstanceID: "old-service", ExecutableSHA256: "old-build"}); err != nil {
				t.Fatal(err)
			}
			paths := []string{filepath.Join(dir, "runner.db"), shutdownPath(dir)}
			before := make(map[string][]byte)
			for _, path := range paths {
				var err error
				before[path], err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, got := Ensure(ctx, dir, "unused"); got == nil || got.Detail.Code != "legacy_orca_state" {
				t.Fatalf("Ensure returned %v", got)
			}
			if got := fault(Serve(ctx, dir, ""), nil); got == nil || got.Detail.Code != "legacy_orca_state" {
				t.Fatalf("Serve returned %v", got)
			}
			if info, got := Status(ctx, dir); got != nil || info.Status != "not_running" || info.LastShutdown == nil || info.LastShutdown.InstanceID != "old-service" {
				t.Fatalf("shutdown evidence unavailable: %+v, %v", info, got)
			}
			if got := InspectOffline(ctx, dir, Options{Command: "inspect", RunID: run.ID}); !got.OK {
				t.Fatalf("original Run unavailable: %+v", got)
			}
			for _, path := range paths {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before[path], after) {
					t.Fatalf("startup changed %s: %v", path, err)
				}
			}
		})
	}
}

func TestStartupCompatibilityAllowsStructuredOrcaAndDirectCodex(t *testing.T) {
	dir := t.TempDir()
	if err := checkOrcaStartupState(dir); err != nil {
		t.Fatal(err)
	}
	saveStartupTestRun(t, dir, "orca-terminal", b.ProtocolOrcaAgentSessionV1, "stopped")
	saveStartupTestRun(t, dir, "codex-app-server", "", "stopped")
	if err := checkOrcaStartupState(dir); err != nil {
		t.Fatal(err)
	}
}

func TestStartupCompatibilityDistinguishesOrcaCheckoutOperations(t *testing.T) {
	for _, tt := range []struct {
		name       string
		protocol   b.ExecutionProtocol
		backend    b.BackendKind
		probe      *contracts.EnvironmentCheckInput
		fast       bool
		invocation *runstate.EnvironmentInvocation
		allowed    bool
	}{
		{name: "checkout probe", protocol: b.ProtocolLocalProcess, backend: b.BackendOrca, probe: &contracts.EnvironmentCheckInput{}, invocation: &runstate.EnvironmentInvocation{}, allowed: true},
		{name: "checkout fast-forward", protocol: b.ProtocolLocalProcess, backend: b.BackendOrca, fast: true, invocation: &runstate.EnvironmentInvocation{}, allowed: true},
		{name: "missing invocation", protocol: b.ProtocolLocalProcess, backend: b.BackendOrca, fast: true},
		{name: "ordinary local process", protocol: b.ProtocolLocalProcess, backend: b.BackendOrca, invocation: &runstate.EnvironmentInvocation{}},
		{name: "foreign backend", protocol: b.ProtocolLocalProcess, backend: b.BackendLocal, fast: true, invocation: &runstate.EnvironmentInvocation{}},
		{name: "legacy protocol", protocol: b.ProtocolOrcaTerminalV3, backend: b.BackendOrca, fast: true, invocation: &runstate.EnvironmentInvocation{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store, err := storage.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			run := &runstate.Record{
				ID: "checkout", Revision: 1, Status: "completed", StopStatus: "stopped",
				Request: contracts.RunRequest{
					Execution: contracts.Execution{Adapter: "orca-terminal"}, EnvironmentCheck: tt.probe, FastForward: tt.fast,
				},
				Snapshot:              b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: tt.backend, Protocol: tt.protocol}},
				EnvironmentInvocation: tt.invocation,
			}
			writeErr := store.Commit(run, 0, nil)
			if closeErr := store.Close(); writeErr != nil || closeErr != nil {
				t.Fatalf("save Run: %v; close: %v", writeErr, closeErr)
			}
			if err := checkOrcaStartupState(dir); (err == nil) != tt.allowed {
				t.Fatalf("allow=%t, got %v", tt.allowed, err)
			}
		})
	}
}

func saveStartupTestRun(t *testing.T, dir, adapter string, protocol b.ExecutionProtocol, status string) *runstate.Record {
	t.Helper()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run := &runstate.Record{
		ID:       adapter,
		Revision: 1,
		Status:   status,
		Request:  contracts.RunRequest{Execution: contracts.Execution{Adapter: adapter}},
		Snapshot: b.LifecycleSnapshot{Ref: b.ExecutionRef{Protocol: protocol}},
	}
	if err := store.Commit(run, 0, nil); err != nil {
		t.Fatal(err)
	}
	return run
}
