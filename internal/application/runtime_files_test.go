package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
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

func (*fileBusinessPort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, ResourceAvailable: true}, nil
}

const native101Arrangement = `{"tests":["N101-CONTENT","N101-SCOPE","demo.progress.roundtrip","demo.progress.retry","demo.progress.cleanup"]}`
const native101Progress = `{"test.N101-CONTENT":false,"test.N101-SCOPE":false,"test.demo.progress.cleanup":false,"test.demo.progress.retry":false,"test.demo.progress.roundtrip":false}`

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

func TestRequiredInputFileGatesBusiness(t *testing.T) {
	const arrangement = native101Arrangement
	for _, mode := range []string{"exact", "missing", "changed-after-initialization", "initial-read-unavailable"} {
		t.Run(mode, func(t *testing.T) {
			e, w, _ := operationEngine(t)
			r := e.runs["run"].Clone()
			r.Snapshot.Workspace.Path = t.TempDir()
			r.Snapshot.WorkspacePlan = r.Snapshot.Workspace
			r.Request.ResultFile = &contracts.ResultFileInput{RequiredInput: true, InitialContentBase64: base64.StdEncoding.EncodeToString([]byte(arrangement))}
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			e.advance("run", w)
			p := &fileBusinessPort{}
			w.port = p
			var writes atomic.Int32
			e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
				return &runtimeFileChannel{state: fileState(r, result), initialize: func(_ context.Context, raw []byte) error {
					writes.Add(1)
					if string(raw) != arrangement {
						return errors.New("changed seed")
					}
					return nil
				}, read: func(context.Context) ([]byte, error) {
					switch {
					case mode == "missing":
						return nil, os.ErrNotExist
					case mode == "initial-read-unavailable" && prepare:
						return nil, context.DeadlineExceeded
					case mode == "changed-after-initialization" && !prepare:
						return []byte(`{"tests":[]}`), nil
					default:
						return []byte(arrangement), nil
					}
				}, close: func() error { return nil }}, nil
			}
			e.advance("run", w)
			if writes.Load() != 1 {
				t.Fatalf("required input initialization attempts: %d", writes.Load())
			}
			if mode == "missing" || mode == "changed-after-initialization" {
				got := e.runs["run"]
				if len(p.requests) != 0 || got.Phase != "required_input_unverified" || !strings.Contains(got.Diagnostic, "required input file") {
					t.Fatalf("unverified file crossed dispatch: requests=%d, phase=%s, diagnostic=%s", len(p.requests), got.Phase, got.Diagnostic)
				}
				return
			}
			if len(p.requests) != 1 || p.requests[0].Input.Turn == nil {
				t.Fatalf("verified bytes did not permit one business turn: %v", p.requests)
			}
			body := p.requests[0].Input.Turn.Prompt
			if !strings.Contains(body, "frozen caller input") || strings.Contains(body, "coordinate writers") || strings.Contains(body, arrangement) {
				t.Fatal("wrong required input prompt", body)
			}
			if resultView(e).CollectionStatus != "disabled" {
				t.Fatal("required input was exposed as a result")
			}
		})
	}
}

func TestNative101RequiredInputAndProgressHaveSeparateBudgets(t *testing.T) {
	e, w, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Workspace.Path = t.TempDir()
	r.Snapshot.WorkspacePlan = r.Snapshot.Workspace
	r.Request.ResultFile = &contracts.ResultFileInput{RequiredInput: true, InitialContentBase64: base64.StdEncoding.EncodeToString([]byte(native101Arrangement))}
	r.Request.ProgressFile = &contracts.ProgressFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte(native101Progress))}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.advance("run", w)
	p := &fileBusinessPort{}
	w.port = p
	var arrangementReady, progressReady atomic.Bool
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, _ bool) (*runtimeFileChannel, error) {
		delay, expected, ready := 2*time.Second, native101Progress, &progressReady
		if result {
			delay, expected, ready = 2500*time.Millisecond, native101Arrangement, &arrangementReady
		}
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(ctx context.Context, raw []byte) error {
			if string(raw) != expected {
				return errors.New("wrong original bytes")
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				ready.Store(true)
				return nil
			}
		}, read: func(context.Context) ([]byte, error) {
			if !ready.Load() {
				return nil, os.ErrNotExist
			}
			return []byte(expected), nil
		}, close: func() error { return nil }}, nil
	}
	e.advance("run", w)
	got := e.runs["run"]
	if len(p.requests) != 1 || got.ResultFile == nil || !got.ResultFile.Initialized || got.Progress == nil || !got.Progress.Initialized {
		t.Fatalf("independently valid files were not both initialized before dispatch: requests=%d progress=%+v input=%+v", len(p.requests), got.Progress, got.ResultFile)
	}
}

func TestNative101RequiredInputFailurePreservesImplementationOnResume(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Ref.RunID, r.Snapshot.Ref.AttemptID = r.ID, r.Snapshot.Session.AttemptID
	r.Snapshot.Session.ProjectID = r.RepositoryCommon
	r.Snapshot.Session.WorkflowRunID, r.Snapshot.Session.ExecutionID = r.ID, r.ID
	r.Snapshot.ConfigurationDigest, r.Snapshot.ResourceGeneration = "config", "generation"
	r.Request.ResultFile = &contracts.ResultFileInput{RequiredInput: true, InitialContentBase64: base64.StdEncoding.EncodeToString([]byte(native101Arrangement))}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	finishStage(t, e, "实施及必要验证已完成。")
	r = e.runs["run"]
	audit := *r.Snapshot.PendingTurn
	implementationID := r.Workflow.ImplementationResultID
	accepted := workflow.Acquired(r.Workflow, implementationID)
	if r.Workflow.StepID != workflow.StepAuditCode || accepted == nil || accepted.Status != "satisfied" {
		t.Fatal("fixture has no accepted implementation and queued audit")
	}
	if err := e.saveResultLocked(r.ID, &runstate.ResultFile{Progress: runstate.Progress{InitializationAttempted: true, Initialized: true}}); err != nil {
		t.Fatal(err)
	}
	p := &fileBusinessPort{}
	w.port = p
	var reads, preparations atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		if !result || prepare {
			preparations.Add(1)
			return nil, errors.New("resume must only read the original file")
		}
		return &runtimeFileChannel{state: fileState(r, true), read: func(context.Context) ([]byte, error) {
			switch reads.Add(1) {
			case 1:
				return nil, context.DeadlineExceeded
			case 2:
				return []byte(`{"tests":[]}`), nil
			default:
				return []byte(native101Arrangement), nil
			}
		}, close: func() error { return nil }}, nil
	}
	w.gate.Lock()
	defer w.gate.Unlock()
	for attempt := 0; attempt < 3; attempt++ {
		if attempt != 0 {
			r = e.runs["run"].Clone()
			r.PendingResume = fmt.Sprintf("resume-%d", attempt)
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			if !e.resumeRun(r.ID, w, p) {
				t.Fatal("resume did not reauthorize the original audit", e.runs["run"].Diagnostic)
			}
		}
		e.prepareRun("run", w, p, e.runs["run"].Clone())
		got := e.runs["run"]
		preserved := workflow.Acquired(got.Workflow, implementationID)
		if got.Workflow.ImplementationResultID != implementationID || preserved == nil || preserved.Status != "satisfied" || len(preserved.Conflicts) != len(accepted.Conflicts) {
			t.Fatal("file failure changed the accepted implementation")
		}
		if attempt < 2 && (len(p.requests) != 0 || got.Phase != "required_input_unverified" || !got.ResumeRequired) {
			t.Fatalf("unverified file crossed audit dispatch: requests=%d phase=%s diagnostic=%s", len(p.requests), got.Phase, got.Diagnostic)
		}
	}
	if len(p.requests) != 1 || p.requests[0].Input.Turn == nil || p.requests[0].Input.Turn.CommandID != audit.CommandID || p.requests[0].Input.Turn.Prompt != audit.Prompt || reads.Load() != 3 || preparations.Load() != 0 {
		got := e.runs["run"]
		t.Fatalf("expected one dispatch of the original audit after three read-only checks: requests=%d reads=%d preparations=%d phase=%s diagnostic=%s", len(p.requests), reads.Load(), preparations.Load(), got.Phase, got.Diagnostic)
	}
}

func TestRuntimeFilesSharedDeadlineAndNoRetry(t *testing.T) {
	e, w, _ := operationEngine(t)
	enableTestFiles(t, e, 3)
	p := &fileBusinessPort{}
	w.port = p
	release := make(chan struct{})
	defer close(release)
	var writes atomic.Int32
	e.openRuntimeFile = func(_ context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
		return &runtimeFileChannel{state: fileState(r, result), initialize: func(context.Context, []byte) error { writes.Add(1); <-release; return nil }, read: func(context.Context) ([]byte, error) { return []byte("late"), nil }, close: func() error { return nil }}, nil
	}
	start := time.Now()
	e.advance("run", w)
	elapsed := time.Since(start)
	if elapsed < 3900*time.Millisecond || elapsed > 4500*time.Millisecond {
		t.Fatalf("shared window=%v", elapsed)
	}
	start = time.Now()
	e.advance("run", w)
	if time.Since(start) > 500*time.Millisecond || writes.Load() != 1 {
		t.Fatal("repeat wait or second initialization after shared deadline")
	}
	if len(p.requests) == 0 {
		t.Fatal("timeout blocked business")
	}
	t.Logf("initial window=%v; later advance=%v; writes=%d", elapsed, time.Since(start), writes.Load())
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
	response := e.Handle("stop", "run", "", "", "", 0, false, input)
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
