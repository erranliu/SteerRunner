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
