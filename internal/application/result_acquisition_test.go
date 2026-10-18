package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRAAtomicCompletion(t *testing.T) {
	e, w := auditEngine(t)
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	failOperationCommit(t, db)
	before := e.runs["run"].Revision
	finishStage(t, e, "实施及必要验证已完成。")
	if e.runs["run"].Revision != before || len(e.runs["run"].Workflow.Results) != 0 || len(e.runs["run"].Workflow.StageInputs) != 1 {
		t.Fatal("failed commit published successor")
	}
	input := e.runs["run"].Snapshot.PendingTurn
	captured := w.checkpoints[input.CommandID].Head
	if captured == "" {
		t.Fatal("original terminal checkpoint missing")
	}
	if _, err := db.Exec("DROP TRIGGER fail_operation_event"); err != nil {
		t.Fatal(err)
	}
	gitTest(t, e.runs["run"].RepositoryPath, "commit", "--allow-empty", "-m", "later")
	finishStage(t, e, "实施及必要验证已完成。")
	r := e.runs["run"]
	source := workflow.Result(r.Workflow, input.CommandID)
	if source == nil || source.Workspace.Head != captured || len(r.Workflow.Results) != 1 || len(r.Workflow.StageInputs) != 2 {
		t.Fatal("retry reconstructed source checkpoint")
	}
	audit := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
	if e.checkDispatchSource(r, *audit) == nil {
		t.Fatal("stale audit dispatched after commit retry")
	}
}

func TestRARecoveryMissingCheckpoint(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"]
	input := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
	f := e.originalCheckpoint(r, w, *input)
	if f.Qualified || f.Head != "" {
		t.Fatal("old final/current clean HEAD fabricated checkpoint")
	}
}

func TestRAStateAuthority(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Workflow.StepID = workflow.StepAuditParse
	if validateWorkflow(r) == nil {
		t.Fatal("cursor/pending native prompt conflict accepted")
	}
	r = e.runs["run"].Clone()
	r.NextStage = &workflow.StageInput{StepID: workflow.StepReport}
	if validateWorkflow(r) == nil {
		t.Fatal("two dispatchable prompts accepted")
	}
}

func TestRABudgetReportOnce(t *testing.T) {
	e, _ := auditEngine(t)
	finishStage(t, e, "unclassified implementation")
	r := e.runs["run"]
	report := *workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
	finishStage(t, e, "invalid report")
	r = e.runs["run"].Clone()
	report.Prompt = ""
	if r.Workflow.StepID != workflow.StepImplementInput || queueInput(r, report, "") == nil {
		t.Fatal("report retry budget reset")
	}
	before := len(r.Workflow.StageInputs)
	if err := resumeWorkflow(r); err != nil || len(r.Workflow.StageInputs) != before || r.Snapshot.PendingTurn != nil {
		t.Fatal("resume restarted implementation", err)
	}
}

func TestRAConcurrencyPublication(t *testing.T) {
	for _, variant := range []string{"stop", "new-source", "head-drift", "wrong-pr-head"} {
		t.Run(variant, func(t *testing.T) {
			e, _ := auditEngine(t)
			finishStage(t, e, "实施及必要验证已完成。")
			finishStage(t, e, `{"outcome":"done","summary":"audited and validated","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`)
			entered, release := make(chan struct{}), make(chan struct{})
			var url string
			post := checkpoint(e.runs["run"], e.runs["run"].Workflow.AuditResultID).Head
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				close(entered)
				<-release
				head := post
				if variant == "wrong-pr-head" {
					head = "0000000000000000000000000000000000000000"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"html_url": url, "number": 1, "state": "open", "merged": false, "base": map[string]any{"ref": "main", "repo": map[string]string{"full_name": "owner/repo"}}, "head": map[string]any{"ref": "main", "sha": head, "repo": map[string]string{"full_name": "owner/repo"}}})
			}))
			defer server.Close()
			url = server.URL + "/owner/repo/pulls/1"
			r := e.runs["run"].Clone()
			r.Request.Repository.URL = server.URL + "/owner/repo.git"
			r.Request.Repository.BaseBranch = "main"
			r.Request.Publication = contracts.Publication{Provider: "gitea", Repository: "owner/repo", BaseBranch: "main", Settings: canonical(map[string]string{"base_url": server.URL})}
			r.Snapshot.Ref.WorkspaceBranch = "main"
			e.runs["run"] = r
			finishStage(t, e, "Published "+url+"\n本次发布已完成。")
			if e.runs["run"].PendingPRURL != url {
				t.Fatal("publication source not accepted", e.runs["run"].Diagnostic)
			}
			done := make(chan bool, 1)
			go func() { done <- e.verifyPublication("run") }()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				close(release)
				t.Fatal("query never started")
			}
			e.mu.Lock()
			next := e.runs["run"].Clone()
			if variant == "stop" {
				next.Allowed = false
				next.PendingStop = "stop"
				next.StopStatus = "pending"
			}
			if variant == "new-source" {
				next.Workflow.ImplementationResultID = "new-source"
			}
			if variant == "stop" || variant == "new-source" {
				if err := e.commitLocked(next, nil, "concurrent_control"); err != nil {
					e.mu.Unlock()
					close(release)
					t.Fatal(err)
				}
			}
			e.mu.Unlock()
			if variant == "head-drift" {
				gitTest(t, next.RepositoryPath, "commit", "--allow-empty", "-m", "later")
			}
			close(release)
			if <-done {
				t.Fatal("stale query advanced workflow")
			}
			if e.runs["run"].Status == "pr_ready" || e.runs["run"].Workflow.StepID == workflow.StepPRReady {
				t.Fatal("source or PR mismatch promoted")
			}
			if variant == "stop" && e.runs["run"].StopStatus != "pending" {
				t.Fatal("query replaced stop")
			}
		})
	}
}

func TestRAConfigRejectBeforeEffects(t *testing.T) {
	// Admission reads local config before any Git/native side effects.
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "service"), 0700)
	_ = os.WriteFile(filepath.Join(dir, "service", "result-acquisition.json"), []byte(`{"codex_parser":{"enabled":true,"unknown":true}}`), 0600)
	_, err := freeze(context.Background(), dir, contracts.Request{Input: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("invalid local config accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "r")); !os.IsNotExist(err) {
		t.Fatal("invalid config reached repository preparation")
	}
}

func TestRAAuditObservedReportWork(t *testing.T) {
	e, _ := auditEngine(t)
	finishStage(t, e, "实施及必要验证已完成。")
	finishStage(t, e, "natural audit output")
	r := e.runs["run"].Clone()
	input := r.Snapshot.PendingTurn
	if input == nil || input.StepID != workflow.StepAuditReport {
		t.Fatal("audit report not selected")
	}
	r.Snapshot.Session.CurrentCommandID = input.CommandID
	r.Snapshot.Session.CurrentTurnID = "turn-" + input.CommandID
	e.runs["run"] = r
	if err := e.acceptEvent("run", b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventWorkObserved, SessionID: "session", StartedTurnID: r.Snapshot.Session.CurrentTurnID, Source: "native", SourceEventID: "actual-file-change", BusinessWork: "fileChange"}}); err != nil {
		t.Fatal(err)
	}
	finishStage(t, e, `{"outcome":"done","summary":"audited","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`)
	r = e.runs["run"]
	if r.Snapshot.PendingTurn != nil || r.Status != "blocked" || r.Workflow.AuditResultID != "" {
		t.Fatal("observed report work accepted despite unchanged HEAD")
	}
}

func TestRATransitionUnknownNativeGate(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Intents = []operation.Intent{{Request: b.LifecycleRequest{Operation: b.Operation{ID: "original-unknown", Status: b.OperationUnknown}}}}
	e.runs["run"] = r
	finishStage(t, e, "实施及必要验证已完成。")
	r = e.runs["run"]
	if r.Result == nil || len(r.Workflow.StageResults) != 0 || len(r.Workflow.Results) != 0 || r.NextStage != nil {
		t.Fatal("unknown source unlocked result acquisition")
	}
	r = r.Clone()
	r.Intents[0].Request.Operation.Status = b.OperationConfirmed
	e.runs["run"] = r
	finishStage(t, e, "实施及必要验证已完成。")
	r = e.runs["run"]
	if r.Workflow.StepID != workflow.StepImplementInput || r.Workflow.ImplementationResultID != "" {
		t.Fatal("later clean HEAD reconstructed terminal checkpoint")
	}
}

func TestRAStateAuthorityQuestionSource(t *testing.T) {
	e, _ := auditEngine(t)
	finishStage(t, e, `{"outcome":"needs_input","summary":"unfinished","question":"Which interface?"}`)
	r := e.runs["run"]
	if r.Question == nil || r.Question.Source != "program" || r.Workflow.StepID != workflow.StepImplementInput || r.Snapshot.QuestionRoute != nil {
		t.Fatal("actual question source confused with native request or policy")
	}
}

func TestRARecoveryConsumedInput(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Session.CurrentCommandID = r.Snapshot.PendingTurn.CommandID
	preservePendingStage(r)
	if r.NextStage != nil {
		t.Fatal("consumed prompt became an unsent input")
	}
	if err := e.restoreOriginalAttempt(r, w, auditRestorePort{}); err != nil {
		t.Fatal(err)
	}
	if err := resumeWorkflow(r); err != nil || r.Snapshot.PendingTurn != nil || r.NextStage != nil || r.Status != "blocked" {
		t.Fatal("resume resent consumed implementation", err)
	}
}
func TestRAStateAuthorityMilestone(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"].Clone()
	r.Workflow.StepID = workflow.StepPRReady
	r.Snapshot.PendingTurn = nil
	r.Status = "stopped"
	if err := resumeWorkflow(r); err != nil || r.Status != "pr_ready" || r.Snapshot.PendingTurn != nil {
		t.Fatal("milestone projection changed business work", err)
	}
}
func TestRAFeedbackSupersedesUnsent(t *testing.T) {
	e, w := auditEngine(t)
	finishStage(t, e, "unclassified implementation")
	w.port = auditRestorePort{}
	if !e.feedbackValue("run", w, w.port, &runstate.Answer{Epoch: e.epoch, CommandID: "feedback", Text: "Implement the requested adjustment"}, false) {
		t.Fatal(e.runs["run"].Diagnostic)
	}
	r := e.runs["run"]
	if r.NextStage != nil || r.Snapshot.PendingTurn == nil || r.Workflow.StepID != workflow.StepImplement || validateWorkflow(r) != nil {
		t.Fatal("explicit feedback left two input authorities")
	}
}

type consumedObservationPort struct {
	auditPort
	evidence     b.TurnEvidence
	reads, sends int
}

func (p *consumedObservationPort) ObserveLifecycle(context.Context, b.LifecycleSnapshot) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reads++
	return b.LifecycleReceipt{State: b.LifecycleStateFacts{SessionID: p.evidence.SessionID}, TurnEvidence: &p.evidence}, nil, nil
}
func (p *consumedObservationPort) Execute(context.Context, b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.sends++
	return b.LifecycleReceipt{}, nil, nil
}
func TestRAResumeConsumedActiveInputWaitsForOriginalEvidence(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"].Clone()
	original := *r.Snapshot.PendingTurn
	r.Snapshot.PendingTurn = nil
	r.Snapshot.Session.CurrentCommandID = original.CommandID
	r.Snapshot.Session.CurrentTurnID = "original-turn"
	r.Snapshot.Session.PendingCommandID = ""
	r.Snapshot.Session.PendingTurnID = ""
	r.Snapshot.Session.LastStoppedTurnID = ""
	r.Allowed, r.ResumeRequired = false, true
	r.PendingResume = "resume"
	p := &consumedObservationPort{evidence: b.TurnEvidence{CommandID: original.CommandID, SessionID: "session", TurnID: "original-turn", Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "original-final", LastMessage: "original completed work"}}
	w.port = p
	if err := e.commitLocked(r, nil, "restarted"); err != nil {
		t.Fatal(err)
	}
	if !e.resumeRun("run", w, p) {
		t.Fatal(e.runs["run"].Diagnostic)
	}
	resumed := e.runs["run"]
	if resumed.Status != "reconciling" || resumed.Phase != "original_result_pending" || resumed.Snapshot.PendingTurn != nil || resumed.NextStage != nil || len(resumed.Workflow.StageInputs) != 1 || resumed.ResumeRequired || !resumed.Allowed {
		t.Fatal("resume blocked or recreated consumed input", resumed.Status, resumed.Phase)
	}
	e.advance("run", w)
	result := workflow.Result(e.runs["run"].Workflow, original.CommandID)
	if p.reads != 1 || p.sends != 0 || result == nil || result.TurnID != "original-turn" || result.Final != "original completed work" {
		t.Fatal("original completion was not read without a resend", p.reads, p.sends, result)
	}
}
