package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func fileTransferFixture(t *testing.T, adapter string) (*Engine, *worker) {
	t.Helper()
	e, w, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Request.Execution.Adapter = adapter
	r.Request.ResultFile = &contracts.ResultFileInput{}
	r.Snapshot.Workspace.Path = t.TempDir()
	r.ManagedWorkspace = &runstate.ManagedWorkspace{Identity: "fixture-incarnation", OriginRunID: r.ID}
	p := fileState(r, true)
	p.InitializationAttempted, p.InitializationWriteStatus = true, "not_started"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.saveResultLocked(r.ID, &runstate.ResultFile{Progress: *p}); err != nil {
		t.Fatal(err)
	}
	return e, w
}

func stopFileTransferFixture(t *testing.T, e *Engine) {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs["run"].Clone()
	r.Allowed, r.Closed, r.Status, r.StopStatus = false, true, "stopped", "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	p := *e.runs[r.ID].ResultFile
	p.Sealed = true
	if err := e.saveResultLocked(r.ID, &p); err != nil {
		t.Fatal(err)
	}
}

func requireTransferRejected(t *testing.T, e *Engine) {
	t.Helper()
	if _, unlock, err := e.reusableSource("run"); err == nil {
		unlock()
		t.Fatal("unsettled file writer allowed workspace transfer")
	}
}

func awaitFilePreparationExit(t *testing.T, e *Engine, w *worker) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		active := w.filePreparationsActive
		e.mu.Unlock()
		if active == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("physical preparation did not exit")
}

func TestWorktreeReuseWaitsForLateFileWriterAndClose(t *testing.T) {
	e, w := fileTransferFixture(t, "local-codex")
	entered, release, closing, closed := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		select {
		case <-closed:
		default:
			close(closed)
		}
	})
	path := filepath.Join(e.runs["run"].Snapshot.Workspace.Path, "retained")
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error {
			close(entered)
			<-release
			return os.WriteFile(path, []byte("old seed"), 0600)
		}, close: func() error { close(closing); <-closed; return nil }}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan runtimeFileSample, 1)
	go func() { result <- e.sampleRuntimeFile(ctx, e.runs["run"].Clone(), true, true) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("initializer not entered")
	}
	cancel()
	sample := <-result
	if sample.err == nil {
		t.Fatal("blocked physical writer exceeded no deadline")
	}
	stopFileTransferFixture(t, e)
	requireTransferRejected(t, e)
	// Restart cannot turn a durable pending obligation into writer absence.
	records, err := e.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Engine{ctx: context.Background(), runs: map[string]*runstate.Record{"run": records[0]}, workers: map[string]*worker{"run": {wake: make(chan struct{}, 1)}}}
	requireTransferRejected(t, restarted)
	close(release)
	<-closing
	requireTransferRejected(t, e)
	close(closed)
	awaitFilePreparationExit(t, e, w)
	source, unlock, err := e.reusableSource("run")
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if source.ResultFile.InitializationWriteStatus != "settled" || source.ResultFile.Initialized || !source.ResultFile.Sealed {
		t.Fatal("late write published bytes or lost settlement", source.ResultFile)
	}
	e.mu.Lock()
	stale := *source.ResultFile
	stale.InitializationWriteStatus = "not_started"
	err = e.saveResultLocked("run", &stale)
	e.mu.Unlock()
	if err != nil || e.runs["run"].ResultFile.InitializationWriteStatus != "settled" {
		t.Fatal("stale sampling erased settlement", err)
	}
	if err := os.WriteFile(path, []byte("new writer"), 0600); err != nil {
		t.Fatal(err)
	}
	// Transfer can only occur after all old physical writes and close finished.
	next := source.Clone()
	next.ID, next.ManagedWorkspace.SourceRunID = "successor", source.ID
	e.mu.Lock()
	err = e.commitLocked(next, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new writer" {
		t.Fatal("old write affected successor", string(data), err)
	}
}

func TestWorktreeReuseUnknownRemoteFileWriteStaysUnsettledAfterRead(t *testing.T) {
	e, w := fileTransferFixture(t, "orca-terminal")
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error { return errors.New("initialization_unverified") }, read: func(context.Context) ([]byte, error) { return []byte("actual bytes"), nil }, close: func() error { return nil }}, nil
	}
	r := e.runs["run"].Clone()
	sample := e.sampleRuntimeFile(context.Background(), r, true, true)
	awaitFilePreparationExit(t, e, w)
	e.mu.Lock()
	e.publishFilePreparationLocked(r, sample, true)
	e.mu.Unlock()
	if sample.err != nil || !e.runs["run"].ResultFile.Initialized || e.runs["run"].ResultFile.InitializationWriteStatus != "pending" {
		t.Fatal("read must remain independent of write settlement", sample.err)
	}
	stopFileTransferFixture(t, e)
	requireTransferRejected(t, e)
	// Legacy attempted writes without settlement evidence also fail closed.
	e.runs["run"].ResultFile.InitializationWriteStatus = ""
	requireTransferRejected(t, e)
}

func TestResultSealSurvivesConcurrentFileWriteSettlement(t *testing.T) {
	e, _ := fileTransferFixture(t, "local-codex")
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs["run"].Clone()
	r.Allowed, r.Status, r.StopStatus = false, "stopped", "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	original := e.runs["run"].Clone()
	p := *original.ResultFile
	p.InitializationWriteStatus = "settled"
	if err := e.saveResultLocked(r.ID, &p); err != nil {
		t.Fatal(err)
	}
	if err := e.publishResultLocked(original, []byte("final bytes"), nil); err != nil {
		t.Fatal(err)
	}
	got := e.runs[r.ID].ResultFile
	if !got.Sealed || got.InitializationWriteStatus != "settled" || got.View.Version != 1 {
		t.Fatal("settlement consumed the only seal attempt", got)
	}
}

type cleanupTransferPort struct {
	operationCommitPort
	entered, release chan struct{}
	calls            int
}

func (p *cleanupTransferPort) ShutdownLifecycle(context.Context, b.LifecycleSnapshot, b.StopHandle) (b.LifecycleStateFacts, error) {
	p.calls++
	close(p.entered)
	<-p.release
	return b.LifecycleStateFacts{Stopped: true}, nil
}

func TestWorktreeReuseSerializesOldCleanupAndTransfer(t *testing.T) {
	e, w, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Allowed, r.Closed, r.Status, r.StopStatus = false, true, "stopped", "stopped"
	r.ManagedWorkspace = &runstate.ManagedWorkspace{Identity: "cleanup-incarnation", OriginRunID: r.ID}
	r.Snapshot.Workspace.Path = t.TempDir()
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	p := &cleanupTransferPort{entered: make(chan struct{}), release: make(chan struct{})}
	w.port = p
	done := make(chan struct{})
	go func() { e.closeWorker("run", w); close(done) }()
	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not start")
	}
	if w.gate.TryLock() {
		w.gate.Unlock()
		close(p.release)
		t.Fatal("cleanup did not retain transfer gate")
	}
	close(p.release)
	<-done
	source, unlock, err := e.reusableSource("run")
	if err != nil {
		t.Fatal(err)
	}
	next := source.Clone()
	next.ID, next.ManagedWorkspace.SourceRunID = "successor", source.ID
	e.mu.Lock()
	err = e.commitLocked(next, nil)
	e.mu.Unlock()
	unlock()
	if err != nil {
		t.Fatal(err)
	}
	e.closeWorker("run", w)
	if p.calls != 1 {
		t.Fatal("old cleanup reached port after transfer", p.calls)
	}
}
