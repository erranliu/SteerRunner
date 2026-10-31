package application

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/textanalysis"
)

const parsedAudit = `{"outcome":"done","summary":"audited and validated zero repairs","question":null,"blocker":null,"fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`

func analysisEngine(t *testing.T) (*Engine, *worker) {
	t.Helper()
	e, w := auditEngine(t)
	config := ra.DefaultConfig()
	config.CodexParser.Enabled = true
	if err := os.MkdirAll(filepath.Join(e.dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, "service", "result-acquisition.json"), canonical(config), 0600); err != nil {
		t.Fatal(err)
	}
	finishStage(t, e, "实施及必要验证已完成。")
	finishStage(t, e, "Audit and validation completed. Zero repairs, no remaining P0-P2 issues.")
	if e.runs["run"].Workflow.StepID != workflow.StepAuditParse {
		t.Fatal("analysis wait not selected", e.runs["run"].Workflow.StepID)
	}
	return e, w
}

func TestTextAcquisitionCandidates(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		err              error
	}{
		{"done", parsedAudit, workflow.StepPublish, nil},
		{"missing", `{"outcome":"needs_input","summary":"validation missing","question":"provide evidence","blocker":null,"fixed_count":null,"fixed_issues":null,"remaining_issues":null}`, workflow.StepAuditInput, nil},
		{"invalid", "not JSON", workflow.StepAuditReport, nil},
		{"failed", "", workflow.StepAuditReport, errors.New("CLI failed")},
		{"conflict", `{"outcome":"done","summary":"conflicting count","question":null,"blocker":null,"fixed_count":1,"fixed_issues":[],"remaining_issues":[]}`, workflow.StepAuditParse, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, w := analysisEngine(t)
			before := e.runs["run"].Clone()
			var calls atomic.Int32
			e.analyzeText = func(context.Context, textanalysis.Request, ra.ParserConfig) (string, error) {
				calls.Add(1)
				return tc.body, tc.err
			}
			e.advanceAcquisition("run", w, nil)
			task := w.acquisition
			<-task.done
			if !e.advanceAcquisition("run", w, nil) {
				t.Fatal("result not committed")
			}
			after := e.runs["run"]
			if after.Workflow.StepID != tc.want || calls.Load() != 1 || len(after.Intents) != len(before.Intents) || len(after.Workflow.StageResults) != len(before.Workflow.StageResults) || after.LastFinal.Text != before.LastFinal.Text {
				t.Fatal("analysis changed native execution or wrong transition", after.Workflow.StepID, after.Diagnostic)
			}
			if after.Workflow.StepID == workflow.StepPublish && (len(after.Workflow.Audits) != 1 || after.Workflow.AuditResultID == "") {
				t.Fatal("audit facts lost")
			}
			if tc.name == "conflict" {
				if after.Status != "blocked" {
					t.Fatal("conflict did not block")
				}
				for i := 0; i < 3; i++ {
					if e.advanceAcquisition("run", w, nil) || w.acquisition != nil {
						t.Fatal("blocked conflict restarted analysis")
					}
				}
				if calls.Load() != 1 {
					t.Fatal("conflict repeated inference", calls.Load())
				}
			}
		})
	}
}

func TestTextAcquisitionRevocationAndNewTask(t *testing.T) {
	for _, change := range []string{"pause", "source", "blocked"} {
		t.Run(change, func(t *testing.T) {
			e, w := analysisEngine(t)
			original := e.runs["run"].Clone()
			started := []chan struct{}{make(chan struct{}), make(chan struct{})}
			release := []chan struct{}{make(chan struct{}), make(chan struct{})}
			defer func() {
				for _, ch := range release {
					select {
					case <-ch:
					default:
						close(ch)
					}
				}
			}()
			var calls atomic.Int32
			e.analyzeText = func(context.Context, textanalysis.Request, ra.ParserConfig) (string, error) {
				i := int(calls.Add(1)) - 1
				close(started[i])
				<-release[i] // Deliberately ignore cancellation to test late delivery.
				return parsedAudit, nil
			}
			e.advanceAcquisition("run", w, nil)
			old := w.acquisition
			<-started[0]
			e.mu.Lock()
			next := e.runs["run"].Clone()
			switch change {
			case "pause":
				next.Allowed, next.ResumeRequired = false, true
			case "source":
				next.Workflow.StageResults[len(next.Workflow.StageResults)-1].Final += " new source"
			case "blocked":
				next.Status = "blocked"
			}
			err := e.commitLocked(next, nil, "business_revoked")
			e.mu.Unlock()
			if err != nil || w.acquisition != nil {
				t.Fatal("revocation did not invalidate task", err)
			}
			// Existing resume/source selection renews business eligibility. No parser
			// generation, durable task ID or inherited budget is needed.
			e.mu.Lock()
			next = original.Clone()
			if err = resumeWorkflow(next); err == nil {
				err = e.commitLocked(next, nil, "business_restored")
			}
			e.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			e.advanceAcquisition("run", w, nil)
			current := w.acquisition
			<-started[1]
			close(release[0])
			<-old.done
			if e.advanceAcquisition("run", w, nil) || w.acquisition != current || e.runs["run"].Workflow.StepID != workflow.StepAuditParse {
				t.Fatal("old completion displaced the current task")
			}
			close(release[1])
			<-current.done
			if !e.advanceAcquisition("run", w, nil) || e.runs["run"].Workflow.StepID != workflow.StepPublish || calls.Load() != 2 {
				t.Fatal("renewed task was not accepted exactly once")
			}
		})
	}
}

func TestTextAcquisitionOtherRunProgressAndStops(t *testing.T) {
	e, w := analysisEngine(t)
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	e.analyzeText = func(context.Context, textanalysis.Request, ra.ParserConfig) (string, error) {
		close(started)
		<-release
		return parsedAudit, nil
	}
	e.advanceAcquisition("run", w, nil)
	task := w.acquisition
	<-started

	other, ow := auditEngine(t)
	r := other.runs["run"].Clone()
	r.ID = "other"
	e.workers[r.ID] = ow
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		input := r.Snapshot.PendingTurn
		e.mu.Lock()
		next := e.runs[r.ID].Clone()
		next.Snapshot.Session.CurrentCommandID, next.Snapshot.Session.CurrentTurnID = input.CommandID, "other-turn"
		e.runs[r.ID] = next
		ow.ownedInputs = map[string]bool{input.CommandID: true}
		e.mu.Unlock()
		e.finish(r.ID, b.TurnEvidence{CommandID: input.CommandID, SessionID: "session", TurnID: "other-turn", Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "item", LastMessage: "实施及必要验证已完成。"})
	}()
	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		t.Fatal("other Run progress waited for parser")
	}
	if e.runs[r.ID].Workflow.StepID != workflow.StepAuditCode {
		t.Fatal("other Run did not advance")
	}
	for _, id := range []string{"other", "run"} {
		body := canonical(contracts.RevisionInput{ExpectedRevision: e.runs[id].Revision})
		request := contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-" + id, Input: body}
		if response := e.Handle("stop", id, "", "", "", 0, false, &request); !response.OK {
			t.Fatal(response)
		}
		stopped := make(chan struct{})
		go func(id string) { defer close(stopped); e.advance(id, e.workers[id]) }(id)
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("stop waited for parser", id)
		}
		if e.runs[id].StopStatus != "stopped" {
			t.Fatal("stop did not complete", id)
		}
	}
	if w.acquisition != nil {
		t.Fatal("own stop retained task")
	}
	select {
	case <-task.done:
		t.Fatal("probe released parser too soon")
	default:
	}
}

func TestTextAcquisitionCommitRetryAndRevision(t *testing.T) {
	e, w := analysisEngine(t)
	var calls atomic.Int32
	e.analyzeText = func(context.Context, textanalysis.Request, ra.ParserConfig) (string, error) {
		calls.Add(1)
		return parsedAudit, nil
	}
	e.advanceAcquisition("run", w, nil)
	task := w.acquisition
	<-task.done
	// An unrelated observation does not discard completed analysis.
	e.mu.Lock()
	next := e.runs["run"].Clone()
	if err := e.commitLocked(next, nil, "observation"); err != nil {
		t.Fatal(err)
	}
	e.mu.Unlock()
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	failOperationCommit(t, db)
	if e.advanceAcquisition("run", w, nil) || w.acquisition != task {
		t.Fatal("failed commit consumed candidate")
	}
	if _, err = db.Exec("DROP TRIGGER fail_operation_event"); err != nil {
		t.Fatal(err)
	}
	if !e.advanceAcquisition("run", w, nil) || calls.Load() != 1 || e.runs["run"].Workflow.StepID != workflow.StepPublish {
		t.Fatal("commit retry repeated inference or lost result")
	}
}

func TestTextAcquisitionCancellationDoesNotWait(t *testing.T) {
	e, w := analysisEngine(t)
	started, release := make(chan struct{}), make(chan struct{})
	e.analyzeText = func(ctx context.Context, _ textanalysis.Request, _ ra.ParserConfig) (string, error) {
		close(started)
		<-release
		return parsedAudit, nil
	}
	e.advanceAcquisition("run", w, nil)
	task := w.acquisition
	<-started
	for i := 0; i < 3; i++ {
		e.advanceAcquisition("run", w, nil)
		if w.acquisition != task {
			t.Fatal("wake replaced running task")
		}
	}
	e.mu.Lock()
	next := e.runs["run"].Clone()
	next.Allowed = false
	next.PendingStop = "stop"
	next.StopStatus = "pending"
	if err := e.commitLocked(next, nil, "stop_requested"); err != nil {
		t.Fatal(err)
	}
	if w.acquisition != nil {
		t.Fatal("stop did not invalidate memory task")
	}
	e.mu.Unlock()
	close(release)
	<-task.done
	if e.advanceAcquisition("run", w, nil) || e.runs["run"].Workflow.StepID != workflow.StepAuditParse {
		t.Fatal("late result advanced stopped run")
	}
}

func TestTextAcquisitionRestartRecomputesUncommitted(t *testing.T) {
	e, w := analysisEngine(t)
	// No parser operation was saved. A fresh service can recompute from business facts.
	other, err := New(context.Background(), e.dir, e.store, nil)
	if err != nil {
		t.Fatal(err)
	}
	other.analyzeText = func(context.Context, textanalysis.Request, ra.ParserConfig) (string, error) { return parsedAudit, nil }
	ow := other.workers["run"]
	other.advanceAcquisition("run", ow, nil)
	<-ow.acquisition.done
	if !other.advanceAcquisition("run", ow, nil) || other.runs["run"].Workflow.StepID != workflow.StepPublish {
		t.Fatal("restart failed to recompute")
	}
	if w.acquisition != nil {
		t.Fatal("original service acquired a durable parser")
	}
}
