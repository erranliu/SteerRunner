package application

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func resultFixture(t *testing.T, progress bool) (*Engine, *worker) {
	t.Helper()
	e, w := evidenceEngine(t)
	e.logger = log.New(io.Discard, "", 0)
	r := e.runs["run"].Clone()
	r.Request.ResultFile = &contracts.ResultFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte("seed"))}
	if progress {
		r.Request.ProgressFile = &contracts.ProgressFileInput{InitialContentBase64: "cHJvZ3Jlc3M="}
	}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if !e.prepareRuntimeFiles(e.runs["run"].Clone()) {
		t.Fatal("initialization failed", e.runs["run"].ResultFile)
	}
	return e, w
}
func resultStopped(t *testing.T, e *Engine) {
	t.Helper()
	r := e.runs["run"].Clone()
	r.Status, r.StopStatus = "stopped", "stopped"
	r.PendingStop = ""
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
}
func resultView(e *Engine) contracts.ResultFileView {
	return e.HandleResultFile("run").Data.(contracts.ResultFileView)
}
func snapshotBytes(t *testing.T, v contracts.ResultFileView) []byte {
	t.Helper()
	if v.ContentBase64 == nil {
		t.Fatal("missing snapshot", v)
	}
	data, err := base64.StdEncoding.DecodeString(*v.ContentBase64)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestResultFilePendingUntilWholeExecutionStops(t *testing.T) {
	e, w := resultFixture(t, false)
	path := resultPath(e.runs["run"])
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"running", "waiting_input", "pr_ready", "completed"} {
		r := e.runs["run"].Clone()
		r.Status = status
		if err := e.commitLocked(r, nil); err != nil {
			t.Fatal(err)
		}
		before := e.runs["run"].Clone()
		e.sealResult(context.Background(), "run")
		v := resultView(e)
		if v.CollectionStatus != "pending" || v.Version != 0 || v.Diagnostic != "" || w.resultCollectionStarted || !reflect.DeepEqual(before, e.runs["run"]) {
			t.Fatal("running/turn end triggered collection", status, v)
		}
	}
	// Passive service teardown may stop a local process, but is not Run end.
	r := e.runs["run"].Clone()
	r.Status, r.StopStatus = "pr_ready", "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.sealResult(context.Background(), "run")
	if resultView(e).CollectionStatus != "pending" {
		t.Fatal("service detach sealed result")
	}
	if e.HandleResultFile("unknown").OK {
		t.Fatal("unknown Run accepted")
	}
}

func TestResultFileFinalBytesAtomicReplacementAndSealOnce(t *testing.T) {
	for _, data := range [][]byte{{}, {0, 255, 'x'}, bytes.Repeat([]byte{254}, contracts.MaxResultBytes)} {
		t.Run(fmt.Sprintf("%d_bytes", len(data)), func(t *testing.T) {
			e, w := resultFixture(t, true)
			path := resultPath(e.runs["run"])
			if path == progressPath(e.runs["run"]) {
				t.Fatal("shared path")
			}
			if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path+".tmp", path); err != nil {
				t.Fatal(err)
			}
			e.collectProgress(context.Background(), "run", "")
			resultStopped(t, e)
			before := e.runs["run"].Clone()
			e.sealResult(context.Background(), "run")
			sealed := e.runs["run"].Clone().ResultFile
			v := resultView(e)
			if !sealed.Sealed || v.CollectionStatus != "closed" || v.Version != 1 || !bytes.Equal(snapshotBytes(t, v), data) || v.SizeBytes != len(data) || !reflect.DeepEqual(before.Progress, e.runs["run"].Progress) || before.Revision != e.runs["run"].Revision {
				t.Fatal(v)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			e.sealResult(context.Background(), "run")
			// Simulate service reload from the actual durable record.
			records, err := e.store.Runs()
			if err != nil {
				t.Fatal(err)
			}
			e.runs["run"] = records[0]
			w.resultCollectionStarted = false
			e.sealResult(context.Background(), "run")
			// A qualified resume never clears the durable seal.
			r := e.runs["run"].Clone()
			r.StopStatus, r.Status = "", "running"
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			resultStopped(t, e)
			w.resultCollectionStarted = false
			e.sealResult(context.Background(), "run")
			if !reflect.DeepEqual(sealed, e.runs["run"].ResultFile) {
				t.Fatal("seal refreshed")
			}
		})
	}
}

func TestResultFileFailureSealsAndPreservesProgress(t *testing.T) {
	for _, failure := range []string{"file_missing", "size_exceeded", "binding_invalid", "transport_unavailable", "file_changing", "read_failed"} {
		t.Run(failure, func(t *testing.T) {
			e, w := resultFixture(t, true)
			e.collectProgress(context.Background(), "run", "")
			path := resultPath(e.runs["run"])
			switch failure {
			case "file_missing":
				_ = os.Remove(path)
			case "size_exceeded":
				_ = os.WriteFile(path, make([]byte, contracts.MaxResultBytes+1), 0600)
			case "binding_invalid":
				e.runs["run"].ResultFile.Binding = "foreign"
			}
			resultStopped(t, e)
			before := e.runs["run"].Clone()
			started := time.Now()
			if failure == "transport_unavailable" || failure == "file_changing" || failure == "read_failed" {
				// Adapter wire failures are separately tested against the original file RPC.
				if err := e.publishResultLocked(before, nil, errors.New(failure)); err != nil {
					t.Fatal(err)
				}
			} else {
				e.sealResult(context.Background(), "run")
			}
			v := resultView(e)
			if time.Since(started) > 2200*time.Millisecond || v.CollectionStatus != "closed" || v.Version != 0 || v.ContentBase64 != nil || v.Diagnostic != failure || !e.runs["run"].ResultFile.Sealed || !reflect.DeepEqual(before.Progress, e.runs["run"].Progress) || e.runs["run"].StopStatus != "stopped" || e.runs["run"].Revision != before.Revision {
				t.Fatal(v)
			}
			_ = os.WriteFile(path, []byte("repair must not refresh"), 0600)
			w.resultCollectionStarted = false
			e.sealResult(context.Background(), "run")
			if !reflect.DeepEqual(v, resultView(e)) {
				t.Fatal("failed seal retried")
			}
			_ = os.WriteFile(progressPath(e.runs["run"]), []byte("progress still independent"), 0600)
			e.collectProgress(context.Background(), "run", "")
			if !reflect.DeepEqual(v, resultView(e)) || e.runs["run"].Progress.View.Version != 2 {
				t.Fatal("progress/result contamination")
			}
		})
	}
}

func TestResultFileStaleAggregateAndLatePublication(t *testing.T) {
	e, _ := resultFixture(t, true)
	stale := e.runs["run"].Clone()
	e.collectProgress(context.Background(), "run", "")
	resultStopped(t, e)
	before := e.runs["run"].Clone()
	e.sealResult(context.Background(), "run")
	sealed := e.runs["run"].Clone().ResultFile
	// An old business clone still carries a legitimate business Result update.
	stale.Result = &b.TurnEvidence{}
	stale.Status, stale.StopStatus = "stopped", "stopped"
	if err := e.commitLocked(stale, nil); err != nil {
		t.Fatal(err)
	}
	progress := e.runs["run"].Clone().Progress
	progress.View.Diagnostic = "file_missing"
	if err := e.saveProgressLocked(before, progress); err != nil {
		t.Fatal(err)
	}
	for _, readErr := range []error{nil, errors.New("read_failed")} {
		if err := e.publishResultLocked(before, []byte("late"), readErr); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(sealed, e.runs["run"].ResultFile) || e.runs["run"].Result == nil || e.runs["run"].Progress.View.Diagnostic != "file_missing" {
		t.Fatal("old whole-record save lost fields")
	}
	records, err := e.store.Runs()
	if err != nil || !reflect.DeepEqual(records[0].ResultFile, sealed) || records[0].Result == nil || !reflect.DeepEqual(records[0].Progress, e.runs["run"].Progress) {
		t.Fatal("durable merge mismatch", err)
	}
	if err := e.saveResultPreparationLocked(before, before.ResultFile); err == nil {
		t.Fatal("late initialization overwrote seal")
	}
}

func TestResultFileBindingAndResumeFence(t *testing.T) {
	e, _ := resultFixture(t, false)
	resultStopped(t, e)
	original := e.runs["run"].Clone()
	for _, change := range []func(*runstate.Record){
		func(r *runstate.Record) { r.Snapshot.Session.ExecutionID = "other-run" },
		func(r *runstate.Record) { r.Snapshot.Ref.External.PID++ },
		func(r *runstate.Record) { r.PendingResume = "resume" },
	} {
		e.runs["run"] = original.Clone()
		change(e.runs["run"])
		if err := e.publishResultLocked(original, []byte("late"), nil); err != nil {
			t.Fatal(err)
		}
		if e.runs["run"].ResultFile.Sealed {
			t.Fatal("wrong execution or resume accepted stale result")
		}
	}
	e.runs["run"] = original.Clone()
	// A current progress or business save does not invalidate a valid seal.
	r := original.Clone()
	r.Summary = "new business fact"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.publishResultLocked(original, []byte("final"), nil); err != nil {
		t.Fatal(err)
	}
	if !e.runs["run"].ResultFile.Sealed || e.runs["run"].Summary != "new business fact" {
		t.Fatal("seal lost current business facts")
	}
}

func TestResultFileInitializationRecoveryAndNoStopCreation(t *testing.T) {
	e, _ := resultFixture(t, false)
	path := resultPath(e.runs["run"])
	p := e.runs["run"].Clone().ResultFile
	p.Initialized = false
	if err := e.saveResultLocked("run", p); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(path, []byte("partial unknown write"), 0600)
	if err := e.prepareResult(e.runs["run"].Clone()); err == nil || err.Error() != "initialization_unverified" {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "partial unknown write" {
		t.Fatal("initialization replayed")
	}
	_ = os.Remove(path)
	if err := e.prepareResult(e.runs["run"].Clone()); err == nil {
		t.Fatal("missing unknown write recreated")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	resultStopped(t, e)
	e.sealResult(context.Background(), "run")
	if resultView(e).Diagnostic != "initialization_unverified" {
		t.Fatal(resultView(e))
	}
	// No initialization is attempted for a Run already stopped before preparation.
	e2, _ := evidenceEngine(t)
	e2.logger = log.New(io.Discard, "", 0)
	r := e2.runs["run"].Clone()
	r.Request.ResultFile = &contracts.ResultFileInput{}
	r.Status, r.StopStatus = "stopped", "stopped"
	if err := e2.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if err := e2.prepareResult(e2.runs["run"].Clone()); err == nil {
		t.Fatal("stop allowed initialization")
	}
	e2.sealResult(context.Background(), "run")
	if _, err := os.Stat(filepath.Join(r.Snapshot.Workspace.Path, ".steerrunner", "result")); !os.IsNotExist(err) {
		t.Fatal("created result during stop", err)
	}
}

func TestResultFilePreparationFailureStillPreparesPeer(t *testing.T) {
	for _, failed := range []string{"progress", "result"} {
		t.Run(failed, func(t *testing.T) {
			e, _ := resultFixture(t, true)
			p := e.runs["run"].Clone().ResultFile
			p.Initialized = false
			if err := e.saveResultLocked("run", p); err != nil {
				t.Fatal(err)
			}
			progress := e.runs["run"].Clone().Progress
			progress.Initialized = false
			if err := e.saveProgressLocked(e.runs["run"], progress); err != nil {
				t.Fatal(err)
			}
			missing := resultPath(e.runs["run"])
			if failed == "progress" {
				missing = progressPath(e.runs["run"])
			}
			if err := os.Remove(missing); err != nil {
				t.Fatal(err)
			}
			if e.prepareRuntimeFiles(e.runs["run"].Clone()) || e.runs["run"].Progress.Initialized != (failed != "progress") || e.runs["run"].ResultFile.Initialized != (failed != "result") {
				t.Fatal("peer preparation skipped")
			}
		})
	}
}

func TestResultFileNormalCompletionRequiresStoppedProducer(t *testing.T) {
	e, _ := resultFixture(t, false)
	r := e.runs["run"].Clone()
	r.Status = "completed"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.sealResult(context.Background(), "run")
	if resultView(e).CollectionStatus != "pending" {
		t.Fatal("completion alone assumed writers stopped")
	}
	r = e.runs["run"].Clone()
	r.StopStatus = "stopped"
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	e.sealResult(context.Background(), "run")
	if resultView(e).Version != 1 {
		t.Fatal(resultView(e))
	}
}

func TestResultFileBoundedReadAndFailedPersistence(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := boundedResultRead(ctx, func() ([]byte, error) { <-release; return []byte("late"), nil })
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatal("parent deadline exceeded", err)
	}
	e, w := resultFixture(t, false)
	resultStopped(t, e)
	before := e.runs["run"].Clone()
	_ = e.store.Close()
	e.sealResult(context.Background(), "run")
	if !reflect.DeepEqual(before, e.runs["run"]) || !w.resultCollectionStarted {
		t.Fatal("failed persistence published a seal")
	}
	// Do not retry reads in this live lifecycle after a failed durable write.
	_ = os.Remove(resultPath(e.runs["run"]))
	e.sealResult(context.Background(), "run")
	if !reflect.DeepEqual(before, e.runs["run"]) {
		t.Fatal("failed persistence changed state")
	}
}

func TestResultFileShutdownDeadlineAndOriginalRecovery(t *testing.T) {
	e, w := resultFixture(t, false)
	resultStopped(t, e)
	// Loaded original end facts can seal without resolving or recreating a port.
	records, err := e.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	e.runs["run"] = records[0]
	w.port = nil
	w.resultCollectionStarted = false
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	e.shutdownContext = expired
	e.closeWorker("run", w)
	v := resultView(e)
	if !e.runs["run"].ResultFile.Sealed || v.Version != 0 || v.Diagnostic != "read_failed" {
		t.Fatal("shutdown deadline not respected", v)
	}
	// A lost original file seals failure rather than creating a new workspace.
	e2, w2 := resultFixture(t, false)
	resultStopped(t, e2)
	originalWorkspace := e2.runs["run"].Snapshot.Workspace.Path
	_ = os.Remove(resultPath(e2.runs["run"]))
	records, err = e2.store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	e2.runs["run"] = records[0]
	w2.port = nil
	w2.resultCollectionStarted = false
	e2.sealResult(context.Background(), "run")
	if resultView(e2).Diagnostic != "file_missing" || e2.runs["run"].Snapshot.Workspace.Path != originalWorkspace || w2.port != nil {
		t.Fatal("original resource loss created a replacement")
	}
}
