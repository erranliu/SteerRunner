package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// Advance all the way to a business input, rather than merely asserting that
// optional preparation returned. Keep normal intent/receipt transactions.
type fileBusinessPort struct{ operationCommitPort }

func (p *fileBusinessPort) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismNotApplicable, Resource: b.MechanismNotApplicable, Session: b.MechanismNotApplicable}}
}
func (p *fileBusinessPort) PreparationFacts(context.Context, b.LifecycleSnapshot) (b.PreparationFacts, error) {
	return b.PreparationFacts{Workspace: b.GateNotApplicable, Resource: b.GateNotApplicable, Session: b.GateNotApplicable}, nil
}

func enableTestFiles(t *testing.T, e *Engine, mask int) {
	t.Helper()
	r := e.runs["run"].Clone()
	r.Snapshot.Workspace.Path = t.TempDir()
	r.Snapshot.WorkspacePlan = r.Snapshot.Workspace
	if mask&1 != 0 {
		r.Request.ProgressFile = &contracts.ProgressFileInput{}
	}
	if mask&2 != 0 {
		r.Request.ResultFile = &contracts.ResultFileInput{}
	}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.advance("run", e.workers["run"]) // Freeze the first pending business input.
}

func TestRuntimeFilesFailureDoesNotGateBusiness(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		for _, failure := range []string{"open", "initialize", "lost-reply", "save"} {
			t.Run(fmt.Sprintf("enabled-%d/%s", mask, failure), func(t *testing.T) {
				e, w, db := operationEngine(t)
				enableTestFiles(t, e, mask)
				p := &fileBusinessPort{}
				w.port = p
				var opens, writes atomic.Int32
				e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
					opens.Add(1)
					if failure == "open" {
						return nil, errors.New("directory_unavailable")
					}
					return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error { writes.Add(1); return errors.New("reply lost") }, read: func(context.Context) ([]byte, error) {
						if failure == "lost-reply" {
							return []byte("agent changed seed"), nil
						}
						return nil, errors.New("file_missing")
					}, close: func() error { return nil }}, nil
				}
				if failure == "save" {
					if _, err := db.Exec(`CREATE TRIGGER fail_file_save BEFORE UPDATE ON runs WHEN NEW.revision=OLD.revision BEGIN SELECT RAISE(ABORT,'file metadata unavailable'); END;`); err != nil {
						t.Fatal(err)
					}
				}
				for n := 0; n < 3; n++ {
					e.advance("run", w)
				}
				business := 0
				for _, request := range p.requests {
					if request.Operation.Action == string(b.LifecycleStartTurn) || request.Operation.Action == string(b.LifecycleSubmitInput) {
						business++
					}
				}
				if business == 0 || e.runs["run"].ResumeRequired || e.runs["run"].Status == "blocked" {
					t.Fatalf("file failure gated business: requests=%+v run=%+v", p.requests, e.runs["run"])
				}
				want := int32(0)
				if mask&1 != 0 {
					want++
				}
				if mask&2 != 0 {
					want++
				}
				if failure == "save" || failure == "open" {
					if writes.Load() != 0 {
						t.Fatal("write crossed failed admission")
					}
				} else if writes.Load() != want {
					t.Fatalf("writes=%d want=%d", writes.Load(), want)
				}
				if failure == "save" && opens.Load() != 0 {
					t.Fatal("metadata failure issued file RPC")
				}
				runs, err := e.store.Runs()
				if err != nil {
					t.Fatal(err)
				}
				// A fresh service must not replay initialization, including when its
				// marker save failed but the business intent was persisted.
				restarted := &Engine{ctx: context.Background(), store: e.store, runs: map[string]*runstate.Record{"run": runs[0]}, workers: map[string]*worker{"run": {wake: make(chan struct{}, 1)}}, openRuntimeFile: e.openRuntimeFile, logger: e.logger}
				before := opens.Load()
				restarted.prepareRuntimeFiles(runs[0])
				restarted.prepareRuntimeFiles(runs[0])
				if opens.Load() != before {
					t.Fatal("restart repeated preparation")
				}
			})
		}
	}
}

func TestResultPreparationReadsOnlyAfterUnconfirmedWrite(t *testing.T) {
	for _, tc := range []struct {
		name              string
		writeErr, readErr error
		wantReads         int
		wantInitialized   bool
		wantDiagnostic    string
	}{
		{name: "write confirmed", wantInitialized: true},
		{name: "reply lost, original readable", writeErr: errors.New("initialization_unverified"), wantReads: 1, wantInitialized: true},
		{name: "reply lost, original missing", writeErr: errors.New("initialization_unverified"), readErr: errors.New("file_missing"), wantReads: 1, wantDiagnostic: "initialization_unverified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, w, _ := operationEngine(t)
			enableTestFiles(t, e, 3)
			port := &fileBusinessPort{}
			w.port = port
			var resultWrites, resultReads, progressReads int
			e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
				if !prepare {
					return nil, errors.New("unexpected non-preparation sample")
				}
				return &runtimeFileChannel{
					state: fileState(r, result),
					initialize: func(context.Context, []byte) error {
						if result {
							resultWrites++
							return tc.writeErr
						}
						return nil
					},
					read: func(context.Context) ([]byte, error) {
						if result {
							resultReads++
							return []byte("original result"), tc.readErr
						}
						progressReads++
						return []byte("original progress"), nil
					},
					close: func() error { return nil },
				}, nil
			}
			e.advance("run", w)
			e.prepareRuntimeFiles(e.runs["run"].Clone())
			if resultWrites != 1 || resultReads != tc.wantReads || progressReads != 1 {
				t.Fatalf("result writes=%d reads=%d, progress reads=%d", resultWrites, resultReads, progressReads)
			}
			got := e.runs["run"].ResultFile
			if got == nil || !got.InitializationAttempted || got.Initialized != tc.wantInitialized ||
				got.View.Diagnostic != tc.wantDiagnostic || got.View.Version != 0 || got.View.ContentBase64 != nil {
				t.Fatalf("unexpected result preparation state: %+v", got)
			}
			if len(port.requests) != 1 || !nativeInputAction(b.LifecycleAction(port.requests[0].Operation.Action)) {
				t.Fatal("result preparation changed business dispatch")
			}
		})
	}
}

func TestRuntimeFilesIndependentDeadlineAndNoRetry(t *testing.T) {
	e, w, _ := operationEngine(t)
	enableTestFiles(t, e, 3)
	p := &fileBusinessPort{}
	w.port = p
	release := make(chan struct{})
	defer close(release)
	var progressWrites, resultWrites atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error {
			if result {
				resultWrites.Add(1)
				return nil
			}
			progressWrites.Add(1)
			<-release
			return nil
		}, read: func(context.Context) ([]byte, error) { return []byte("fixture"), nil }, close: func() error { return nil }}, nil
	}
	start := time.Now()
	e.advance("run", w)
	elapsed := time.Since(start)
	if elapsed < 3900*time.Millisecond || elapsed > 8*time.Second {
		t.Fatalf("first input wait=%v", elapsed)
	}
	start = time.Now()
	e.advance("run", w)
	if time.Since(start) > time.Second || progressWrites.Load() != 1 || resultWrites.Load() != 1 {
		t.Fatal("slow progress prevented the result attempt or caused a retry")
	}
	if len(p.requests) == 0 {
		t.Fatal("timeout blocked business")
	}
	if r := e.runs["run"]; r.Progress.Initialized || !r.ResultFile.Initialized {
		t.Fatal("the result channel did not initialize independently")
	}
}

func TestRuntimeFilesStopDuringIO(t *testing.T) {
	e, w, _ := operationEngine(t)
	enableTestFiles(t, e, 3)
	p := &fileBusinessPort{}
	w.port = p
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var writes atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error {
			if writes.Add(1) == 1 {
				close(entered)
				<-release
			}
			return nil
		}, read: func(context.Context) ([]byte, error) { return []byte("late"), nil }, close: func() error { return nil }}, nil
	}
	done := make(chan bool, 1)
	go func() { done <- e.advance("run", w) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("no file barrier")
	}
	if !w.gate.TryLock() {
		t.Fatal("file I/O retained dispatch gate")
	}
	w.gate.Unlock()
	e.mu.Lock()
	revision := e.runs["run"].Revision
	e.mu.Unlock()
	input := &contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-file-io", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, revision))}
	start := time.Now()
	response := e.Handle("stop", "run", "", "", "", 0, input)
	if !response.OK || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("stop blocked by file I/O: %+v %v", response, time.Since(start))
	}
	t.Logf("stop accepted during file I/O in %v", time.Since(start))
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("file operation did not finish")
	}
	if writes.Load() != 1 || p.executes != 0 || e.runs["run"].Allowed || e.runs["run"].Progress.Initialized || e.runs["run"].ResultFile.Initialized {
		t.Fatal("late file result restored authority or published after stop")
	}
}

func TestRuntimeFilesUnconfirmedInitializationReadsActualBytes(t *testing.T) {
	e, _ := evidenceEngine(t)
	r := e.runs["run"].Clone()
	r.Request.ProgressFile = &contracts.ProgressFileInput{InitialContentBase64: "c2VlZA=="}
	r.Request.ResultFile = &contracts.ResultFileInput{InitialContentBase64: "c2VlZA=="}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	// Fail before path/host metadata can be established, then let the producer
	// create its actual files. Collection must only read, never seed/recreate.
	e.openRuntimeFile = func(context.Context, *runstate.Record, bool, bool) (*runtimeFileChannel, error) {
		return nil, errors.New("transport_unavailable")
	}
	e.prepareRuntimeFiles(e.runs["run"].Clone())
	e.openRuntimeFile = nil
	r = e.runs["run"]
	for _, result := range []bool{false, true} {
		directories, initialize := workspace.ProgressDirectories, workspace.InitializeProgress
		if result {
			directories, initialize = workspace.ResultDirectories, workspace.InitializeResult
		}
		dirs, err := directories(r.Snapshot.Workspace.Path, r.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := initialize(r.Snapshot.Workspace.Path, r.ID, dirs, []byte("producer bytes")); err != nil {
			t.Fatal(err)
		}
	}
	revision := r.Revision
	e.collectProgress(context.Background(), r.ID, "")
	if p := e.runs[r.ID].Progress; !p.Initialized || p.View.Version != 1 || p.View.ContentBase64 == nil || *p.View.ContentBase64 != "cHJvZHVjZXIgYnl0ZXM=" {
		t.Fatal("unconfirmed progress rejected actual bytes", p)
	}
	if e.runs[r.ID].Revision != revision {
		t.Fatal("sampling changed business revision")
	}
	resultStopped(t, e)
	e.sealResult(context.Background(), r.ID)
	if v := resultView(e); v.Version != 1 || string(snapshotBytes(t, v)) != "producer bytes" {
		t.Fatal("unconfirmed result failed final seal", v)
	}
	path := filepath.Join(r.Snapshot.Workspace.Path, ".steerrunner", "result", r.ID, "result.bin")
	if err := os.WriteFile(path, []byte("late overwrite"), 0600); err != nil {
		t.Fatal(err)
	}
	e.sealResult(context.Background(), r.ID)
	if string(snapshotBytes(t, resultView(e))) != "producer bytes" {
		t.Fatal("sealed result refreshed")
	}
}

func TestRuntimeFilesBoundPhysicalProgressReads(t *testing.T) {
	e, w, _ := operationEngine(t)
	enableTestFiles(t, e, 1)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	var opens atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		opens.Add(1)
		return &runtimeFileChannel{state: fileState(r, result), read: func(context.Context) ([]byte, error) {
			close(entered)
			<-release
			close(finished)
			return []byte("late"), nil
		}, close: func() error { return nil }}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	sample := e.sampleRuntimeFile(ctx, e.runs["run"].Clone(), false, false)
	if sample.err == nil {
		t.Fatal("uncooperative read exceeded deadline")
	}
	<-entered
	for n := 0; n < 5; n++ {
		if e.sampleRuntimeFile(context.Background(), e.runs["run"].Clone(), false, false).err == nil {
			t.Fatal("parallel read escaped guard")
		}
	}
	if opens.Load() != 1 || !w.progressReadActive {
		t.Fatal("polling accumulated physical reads")
	}
}

func TestRuntimeFilesLostMarkerRestartOnlyReads(t *testing.T) {
	e, w, db := operationEngine(t)
	enableTestFiles(t, e, 1)
	w.port = &fileBusinessPort{}
	if _, err := db.Exec(`CREATE TRIGGER fail_file_save BEFORE UPDATE ON runs WHEN NEW.revision=OLD.revision BEGIN SELECT RAISE(ABORT,'file metadata unavailable'); END;`); err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error { writes.Add(1); return nil }, read: func(context.Context) ([]byte, error) { return []byte("after restart"), nil }, close: func() error { return nil }}, nil
	}
	e.advance("run", w)
	if _, err := db.Exec(`DROP TRIGGER fail_file_save`); err != nil {
		t.Fatal(err)
	}
	runs, err := e.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	e.runs["run"] = runs[0]
	e.workers["run"] = &worker{wake: make(chan struct{}, 1)}
	e.collectProgress(context.Background(), "run", "")
	if writes.Load() != 0 || e.runs["run"].Progress == nil || e.runs["run"].Progress.View.Version != 1 {
		t.Fatal("lost marker restart replayed a write or refused original bytes", e.runs["run"].Progress)
	}
}
