package application

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestProgressClosedObservationIsNotReopened(t *testing.T) {
	e, w := auditEngine(t)
	e.ctx, e.cancel = context.WithCancel(context.Background())
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	r.Status, r.StopStatus = "stopped", "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.saveProgressLocked(e.runs[r.ID], &runstate.Progress{View: contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: r.ID, CollectionStatus: "closed"}}); err != nil {
		t.Fatal(err)
	}
	w.progressEnabled = true
	e.mu.Lock()
	e.startProgressLocked(r.ID, w)
	started := w.progressWatching
	e.mu.Unlock()
	e.cancel()
	e.wait.Wait()
	if started {
		t.Fatal("service restart reopened an already finalized observation")
	}
}

func TestProgressTerminalShutdownFinishesClosed(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	r.Status, r.StopStatus = "stopped", "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.finishProgress(r.ID, "detached")
	if e.runs[r.ID].Progress.View.CollectionStatus != "closed" {
		t.Fatal("passive shutdown relabeled a terminal Run as detached")
	}
}

func TestProgressActiveAndResumedObserversDetachOnShutdown(t *testing.T) {
	for _, status := range []string{"observing", "closed"} {
		t.Run(status, func(t *testing.T) {
			e, w := auditEngine(t)
			e.ctx, e.cancel = context.WithCancel(context.Background())
			r := e.runs["run"].Clone()
			r.Request.ProgressFile = &contracts.ProgressFileInput{}
			r.Status, r.StopStatus = "running", ""
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			if err := e.saveProgressLocked(e.runs[r.ID], &runstate.Progress{View: contracts.ProgressFileView{RunID: r.ID, CollectionStatus: status}}); err != nil {
				t.Fatal(err)
			}
			w.progressEnabled = true
			e.mu.Lock()
			e.startProgressLocked(r.ID, w)
			started := w.progressWatching
			e.mu.Unlock()
			e.cancel()
			e.wait.Wait()
			if !started || e.runs[r.ID].Progress.View.CollectionStatus != "detached" {
				t.Fatal("active/resumed observation was closed or suppressed")
			}
		})
	}
}

func TestProgressTerminalPendingFinalizationStillStarts(t *testing.T) {
	e, w := auditEngine(t)
	e.ctx, e.cancel = context.WithCancel(context.Background())
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	r.Status, r.StopStatus = "stopped", "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	w.progressEnabled = true
	e.mu.Lock()
	e.startProgressLocked(r.ID, w)
	started := w.progressWatching
	e.mu.Unlock()
	if !started {
		t.Fatal("lost finalization was suppressed")
	}
	for until := time.Now().Add(3 * time.Second); time.Now().Before(until); time.Sleep(10 * time.Millisecond) {
		e.mu.Lock()
		closed := e.runs[r.ID].Progress != nil && e.runs[r.ID].Progress.View.CollectionStatus == "closed"
		e.mu.Unlock()
		if closed {
			e.cancel()
			e.wait.Wait()
			return
		}
	}
	e.cancel()
	e.wait.Wait()
	t.Fatal("terminal observation did not finalize")
}

func TestProgressIndependentVersionSurvivesBusinessCloneAndRestart(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	dirs, err := workspace.ProgressDirectories(r.Snapshot.Workspace.Path, r.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	p := &runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), Directories: dirs, HostID: "local", Initialized: true, InitializationAttempted: true, View: contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: r.ID, Encoding: "base64"}}
	if err := workspace.InitializeProgress(p.Workspace, r.ID, dirs, []byte{0, 255}); err != nil {
		t.Fatal(err)
	}
	if err := e.saveProgressLocked(e.runs[r.ID], p); err != nil {
		t.Fatal(err)
	}
	stale := e.runs[r.ID].Clone()
	revision := stale.Revision
	e.collectProgress(context.Background(), r.ID, "")
	e.collectProgress(context.Background(), r.ID, "")
	v := e.runs[r.ID].Progress.View
	if v.Version != 1 || e.runs[r.ID].Revision != revision || len(w.wake) != 0 {
		t.Fatal(v)
	}
	if err := e.commitLocked(stale, nil); err != nil {
		t.Fatal(err)
	}
	if e.runs[r.ID].Progress.View.Version != 1 {
		t.Fatal("business clone erased progress")
	}
	path := filepath.Join(p.Workspace, filepath.FromSlash(workspace.ProgressRelative(r.ID)))
	_ = os.Remove(path)
	e.collectProgress(context.Background(), r.ID, "")
	v = e.runs[r.ID].Progress.View
	if v.Version != 1 || v.CollectionStatus != "unavailable" || v.ContentBase64 == nil {
		t.Fatal(v)
	}
	_ = os.WriteFile(path, []byte("latest"), 0600)
	e.collectProgress(context.Background(), r.ID, "closed")
	persisted, err := e.store.Runs()
	if err != nil || len(persisted) != 1 {
		t.Fatal(err)
	}
	v = persisted[0].Progress.View
	raw, _ := base64.StdEncoding.DecodeString(*v.ContentBase64)
	if v.Version != 2 || string(raw) != "latest" || v.CollectionStatus != "closed" {
		t.Fatal(v)
	}
	wrong := persisted[0].Clone()
	wrong.Snapshot.Session.ExecutionID = "foreign"
	if _, err := e.readProgress(context.Background(), wrong); err == nil {
		t.Fatal("wrong binding read")
	}
}

func TestProgressStopBeforeInitializationHasNoInventedSnapshot(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.collectProgress(context.Background(), r.ID, "closed")
	v := e.HandleProgress(r.ID).Data.(contracts.ProgressFileView)
	if v.CollectionStatus != "closed" || v.Version != 0 || v.ContentBase64 != nil {
		t.Fatal(v)
	}
	if _, err := os.Stat(filepath.Join(r.Snapshot.WorkspacePlan.Path, ".steerrunner")); !os.IsNotExist(err) {
		t.Fatal("created file during stop", err)
	}
}

func TestProgressInitializationRecoveryRetainsUnverifiedSeed(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte("original-seed"))}
	r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	dirs, err := workspace.ProgressDirectories(r.Snapshot.Workspace.Path, r.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	p := &runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), Directories: dirs, HostID: "local", InitializationAttempted: true, View: contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: r.ID, Encoding: "base64"}}
	if err := workspace.InitializeProgress(p.Workspace, r.ID, dirs, []byte("partial")); err != nil {
		t.Fatal(err)
	}
	if err := e.saveProgressLocked(e.runs[r.ID], p); err != nil {
		t.Fatal(err)
	}
	if err := e.prepareProgress(e.runs[r.ID].Clone()); err == nil || err.Error() != "initialization_unverified" {
		t.Fatal(err)
	}
	path := filepath.Join(p.Workspace, filepath.FromSlash(workspace.ProgressRelative(r.ID)))
	raw, _ := os.ReadFile(path)
	if string(raw) != "partial" || e.runs[r.ID].Progress.Initialized {
		t.Fatal("unverified initialization was replaced/accepted")
	}
	_ = os.Remove(path)
	if err := e.prepareProgress(e.runs[r.ID].Clone()); err == nil {
		t.Fatal("missing seed was recreated")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestProgressOrcaRoutingDoesNotRequireBusinessWorker(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	r.Request.Execution.Adapter = "orca-terminal"
	r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	// This fixture deliberately has no Orca profile. Routing must return a
	// bounded diagnostic rather than dereference a business worker or dispatch.
	if err := e.prepareProgress(e.runs[r.ID].Clone()); err == nil || err.Error() != "binding_invalid" {
		t.Fatal(err)
	}
	if e.runs[r.ID].Progress.View.Diagnostic != "binding_invalid" {
		t.Fatal(e.runs[r.ID].Progress)
	}
}
