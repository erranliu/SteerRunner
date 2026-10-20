package application

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func parserEngine(t *testing.T) (*Engine, *worker, int) {
	t.Helper()
	e, w := auditEngine(t)
	r := e.runs["run"].Clone()
	r.AcquisitionConfig = ra.FrozenConfig{Config: ra.DefaultConfig()}
	r.AcquisitionConfig.Config.CodexParser.Enabled = true
	r.Snapshot.Ref.RunID = "run"
	r.Snapshot.Session.WorkflowRunID = "run"
	r.Snapshot.Ref.AttemptID = r.Snapshot.Session.AttemptID
	e.runs["run"] = r
	finishStage(t, e, "实施及必要验证已完成。")
	finishStage(t, e, "Audited and validated. Fixed zero. No P0-P2 issues remain.")
	r = e.runs["run"]
	index := parserIndex(r)
	if index < 0 || r.Workflow.StepID != workflow.StepAuditParse || r.NextStage != nil || r.Snapshot.PendingTurn != nil || len(r.Workflow.StageInputs) != 2 {
		t.Fatal("parser fabricated a prompt or cursor", r.Workflow.StepID)
	}
	var frozen parser.Package
	if json.Unmarshal([]byte(r.Intents[index].Request.Input.Parse.Payload), &frozen) != nil || frozen.ReportToolPolicy == "" {
		t.Fatal("original adapter report policy was not frozen")
	}
	return e, w, index
}
func parserReceipt(r *runstate.Record, index int, status string) b.LifecycleReceipt {
	body := `{"outcome":"done","summary":"audited validated zero repairs","question":null,"blocker":null,"fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`
	return b.LifecycleReceipt{Operation: r.Intents[index].Request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WritersAbsent: true, WorkerAbsent: true}, ParseEvidence: &b.ParseEvidence{Status: status, Content: body, ContentDigest: ra.Digest([]byte(body)), SessionID: "independent-parser"}}
}
func acceptParser(t *testing.T, e *Engine, w *worker, index int, receipt b.LifecycleReceipt) bool {
	t.Helper()
	w.gate.Lock()
	defer w.gate.Unlock()
	return e.acceptParserReceipt("run", w, index, receipt)
}

func TestRAParserAcquisition(t *testing.T) {
	for _, variant := range []string{"completed", "needs_input", "conflict", "failed", "policy_violation", "side_effect_unverified", "stop", "source-drift", "new-source"} {
		t.Run(variant, func(t *testing.T) {
			e, w, index := parserEngine(t)
			before := e.runs["run"].Clone()
			receipt := parserReceipt(before, index, variant)
			if variant == "needs_input" {
				receipt.ParseEvidence.Status = "completed"
				receipt.ParseEvidence.Content = `{"outcome":"needs_input","summary":"validation not completed","question":"provide evidence","blocker":null,"fixed_count":null,"fixed_issues":null,"remaining_issues":null}`
			}
			if variant == "conflict" {
				receipt.ParseEvidence.Content = `{"outcome":"done","summary":"count contradiction","question":null,"blocker":null,"fixed_count":1,"fixed_issues":[],"remaining_issues":[]}`
			}
			receipt.ParseEvidence.ContentDigest = ra.Digest([]byte(receipt.ParseEvidence.Content))
			originalPort := w.port
			if variant == "failed" {
				// Recovery applies the original receipt before rebuilding the
				// execution port; fallback must use the frozen original policy.
				w.port = nil
			}
			switch variant {
			case "stop":
				r := before.Clone()
				r.Allowed = false
				r.PendingStop = "stop"
				r.StopStatus = "pending"
				e.runs["run"] = r
			case "source-drift":
				gitTest(t, before.RepositoryPath, "commit", "--allow-empty", "-m", "later")
			case "new-source":
				r := before.Clone()
				r.Workflow.ImplementationResultID = "new"
				e.runs["run"] = r
			}
			if !acceptParser(t, e, w, index, receipt) {
				t.Fatal("receipt was not committed")
			}
			w.port = originalPort
			after := e.runs["run"]
			if after.View().Final == nil || after.View().Final.Text != before.View().Final.Text || len(after.Workflow.StageResults) != 2 {
				t.Fatal("parser replaced native final or fabricated turn")
			}
			switch variant {
			case "completed":
				if after.Workflow.StepID != workflow.StepPublish || after.Workflow.AuditResultID == "" || len(after.Workflow.Audits) != 1 || after.Workflow.Audits[0].TurnID != before.Result.TurnID {
					t.Fatal(after.Workflow.StepID)
				}
			case "needs_input":
				if after.Workflow.StepID != workflow.StepAuditInput || len(after.Workflow.StageInputs) != 2 || after.Workflow.AuditResultID != "" {
					t.Fatal("parser success became business success")
				}
			case "conflict":
				if parserIndex(after) >= 0 || after.Status != "blocked" || len(after.Workflow.StageInputs) != 2 {
					t.Fatal("conflict re-entered acquisition")
				}
			case "side_effect_unverified":
				if after.Status != "blocked" || after.Allowed || len(after.Workflow.StageInputs) != 2 || after.NextStage != nil || parserIndex(after) >= 0 {
					t.Fatal("unverified external side effect authorized fallback")
				}
				if validateWorkflow(after) == nil {
					t.Fatal("technical resume could bypass unverified side effects")
				}
			case "failed", "policy_violation":
				if after.Workflow.StepID != workflow.StepAuditReport || len(after.Workflow.StageInputs) != 3 {
					t.Fatal("single report not selected", after.Workflow.StepID)
				}
				finishStage(t, e, "invalid report")
				if e.runs["run"].Workflow.StepID != workflow.StepAuditInput || len(e.runs["run"].Workflow.StageInputs) != 3 {
					t.Fatal("repeated report")
				}
			default:
				if len(after.Workflow.StageInputs) != 2 || after.NextStage != nil {
					t.Fatal("late/stale parser advanced")
				}
			}
		})
	}
}

func TestRALateParserReceiptAndDuplicate(t *testing.T) {
	e, w, index := parserEngine(t)
	receipt := parserReceipt(e.runs["run"], index, "completed")
	if !acceptParser(t, e, w, index, receipt) {
		t.Fatal("first receipt")
	}
	revision := e.runs["run"].Revision
	if !acceptParser(t, e, w, index, receipt) || e.runs["run"].Revision != revision {
		t.Fatal("duplicate receipt re-entered state")
	}
	conflicting := receipt.Clone()
	conflicting.ParseEvidence.SessionID = "different-parser"
	if !acceptParser(t, e, w, index, conflicting) || e.runs["run"].Status != "blocked" {
		t.Fatal("conflicting original receipt accepted")
	}
}

func TestRAStateAuthorityParserHistory(t *testing.T) {
	e, w, index := parserEngine(t)
	r := e.runs["run"].Clone()
	past := r.Intents[index]
	past.Request = past.Request.Clone()
	a := *workflow.Acquired(r.Workflow, past.Request.Input.Parse.ResultID)
	input := *workflow.Input(r.Workflow, a.Source.CommandID)
	raw := *workflow.Result(r.Workflow, a.Source.CommandID)
	input.CommandID = "past-audit"
	input.ImplementationResultID = "past-implementation"
	raw.CommandID = input.CommandID
	raw.TurnID = "past-turn"
	a.ID = "past-result"
	a.Source = workflow.Source(raw)
	a.Attempts = []ra.Attempt{{Method: "llm_parser", Source: a.Source, OperationID: "past-operation", Status: "pending"}}
	var pack parser.Package
	_ = json.Unmarshal([]byte(past.Request.Input.Parse.Payload), &pack)
	pack.Source = a.Source
	pack.ImplementationResultID = input.ImplementationResultID
	pack = parser.FreezePackage(pack)
	payload, _ := json.Marshal(pack)
	past.Request.Input.Parse.Payload = string(payload)
	past.Request.Input.Parse.ResultID = a.ID
	past.Request.Input.Parse.Generation = "past-generation"
	past.Request.Operation.ID = "past-operation"
	wire, _ := json.Marshal(past.Request.Input.Parse)
	past.Request.Operation.Request = wire
	past.Request.Operation.PayloadDigest = ra.Digest(wire)
	r.Workflow.StageInputs = append(r.Workflow.StageInputs, input)
	r.Workflow.StageResults = append(r.Workflow.StageResults, raw)
	r.Workflow.Results = append(r.Workflow.Results, a)
	r.Intents = append(r.Intents, past)
	r.Snapshot.Operations = append(r.Snapshot.Operations, past.Request.Operation)
	if err := validateWorkflow(r); err != nil || parserIndex(r) != index {
		t.Fatal("historical parser became current authority", err)
	}
	e.runs["run"] = r
	receipt := parserReceipt(r, len(r.Intents)-1, "completed")
	if !acceptParser(t, e, w, len(r.Intents)-1, receipt) {
		t.Fatal("late evidence not retained")
	}
	after := e.runs["run"]
	if after.Workflow.StepID != r.Workflow.StepID || after.Status != r.Status || after.Phase != r.Phase || after.Workflow.ImplementationResultID != r.Workflow.ImplementationResultID || workflow.Acquired(after.Workflow, after.Workflow.ImplementationResultID).Status != "satisfied" || len(after.Workflow.StageInputs) != len(r.Workflow.StageInputs) {
		t.Fatal("late old parser polluted the current source")
	}
}

func TestRAAtomicParserRecovery(t *testing.T) {
	e, w, index := parserEngine(t)
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	receipt := parserReceipt(e.runs["run"], index, "completed")
	before := e.runs["run"].Revision
	failOperationCommit(t, db)
	if acceptParser(t, e, w, index, receipt) || e.runs["run"].Revision != before || e.runs["run"].Workflow.AuditResultID != "" {
		t.Fatal("failed commit published parser result")
	}
	_, _ = db.Exec("DROP TRIGGER fail_operation_event")
	r := e.runs["run"].Clone()
	r.Allowed = false
	r.ResumeRequired = true
	e.runs["run"] = r
	if !acceptParser(t, e, w, index, receipt) || len(e.runs["run"].Workflow.StageInputs) != 2 {
		t.Fatal("recovery dispatched without resume")
	}
	r = e.runs["run"].Clone()
	r.Allowed = true
	r.ResumeRequired = false
	if err = resumeWorkflow(r); err != nil || r.Workflow.StepID != workflow.StepAuditParse {
		t.Fatal("resume changed business state", err)
	}
	e.runs["run"] = r
	if !acceptParser(t, e, w, index, receipt) || e.runs["run"].Workflow.StepID != workflow.StepPublish {
		t.Fatal("complete original evidence not applied")
	}
}

func TestRAParserDispatchCommitFailureDoesNotStartResource(t *testing.T) {
	e, w, index := parserEngine(t)
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	failOperationCommit(t, db)
	before := e.runs["run"].Clone()
	w.gate.Lock()
	handled, progress := e.advanceParser("run", w, before)
	w.gate.Unlock()
	if !handled || progress || w.parserTask != nil || e.runs["run"].Intents[index].Request.Operation.Status != b.OperationPending || e.runs["run"].Revision != before.Revision {
		t.Fatal("uncommitted parser reached a native resource")
	}
	if _, err = os.Stat(parser.Directory(e.dir, before.Intents[index].Request)); !os.IsNotExist(err) {
		t.Fatal("uncommitted parser created its resource directory", err)
	}
}

func TestRAConcurrencyParserStopResources(t *testing.T) {
	e, w, index := parserEngine(t)
	r := e.runs["run"].Clone()
	other := r.Clone()
	other.ID = "other"
	other.Intents[index].Request.Operation.Status = b.OperationUnknown
	e.runs["other"] = other
	w.gate.Lock()
	handled, progress := e.advanceParser("run", w, r)
	w.gate.Unlock()
	if !handled || progress || w.parserTask != nil || e.runs["run"].Intents[index].Request.Operation.Status != b.OperationPending {
		t.Fatal("unknown old parser did not retain the single slot")
	}
	delete(e.runs, "other")
	r.Intents[index].Request.Operation.Status = b.OperationUnknown
	if parserResourcesSettled(r) {
		t.Fatal("missing parser root released capacity")
	}
	r.StopStatus = "stopped"
	if stoppedRecord(r, func(*b.ProcessIdentity) bool { return true }) {
		t.Fatal("stop flag hid unknown parser")
	}
	r.Intents[index].Request.Operation.Status = b.OperationPending
	r.Intents[index].Request.Input.Parse.Deadline = time.Now().Add(-time.Second)
	// Arrange an originally expired frozen request; production never edits a
	// committed deadline. Keep the fixture's immutable payload binding valid.
	wire, _ := json.Marshal(r.Intents[index].Request.Input.Parse)
	r.Intents[index].Request.Operation.Request = wire
	r.Intents[index].Request.Operation.PayloadDigest = ra.Digest(wire)
	e.runs["run"] = r
	w.gate.Lock()
	handled, progress = e.advanceParser("run", w, r)
	w.gate.Unlock()
	if !handled || !progress || w.parserTask != nil {
		t.Fatal("expired capacity wait launched process")
	}
	// A definition with no parser does not inspect this other Run's slot.
	plain := &runstate.Record{ID: "plain"}
	w.gate.Lock()
	handled, _ = e.advanceParser("plain", w, plain)
	w.gate.Unlock()
	if handled {
		t.Fatal("unrelated parser blocked a program-only definition")
	}
}

func TestRAStateAuthorityParserReferences(t *testing.T) {
	e, _, index := parserEngine(t)
	r := e.runs["run"].Clone()
	if err := validateWorkflow(r); err != nil {
		t.Fatal("valid parser cursor rejected", err)
	}
	r.Intents[index].Request.Input.Parse.Payload += " "
	if validateWorkflow(r) == nil {
		t.Fatal("mutable parser operation input accepted")
	}
}

func TestRARecoveredParserStartedReceiptStaysRunning(t *testing.T) {
	e, w, index := parserEngine(t)
	r := e.runs["run"].Clone()
	if err := r.Intents[index].MarkParserDispatch(r.Snapshot.Operations); err != nil {
		t.Fatal(err)
	}
	receipt := parserReceipt(r, index, "running")
	receipt.Disposition = b.OperationAccepted
	receipt.State = b.LifecycleStateFacts{ResourcePending: true, ResourceID: r.Intents[index].Request.Input.Parse.Generation}
	dir := parser.Directory(e.dir, r.Intents[index].Request)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "started.json"), canonical(receipt), 0600); err != nil {
		t.Fatal(err)
	}
	e.runs["run"] = r
	w.gate.Lock()
	handled, progress := e.advanceParser("run", w, r)
	w.gate.Unlock()
	current := e.runs["run"]
	if !handled || !progress || current.Phase != "parser_running" || current.Intents[index].Request.Operation.Status != b.OperationAccepted || current.Workflow.StepID != workflow.StepAuditParse || len(current.Workflow.StageInputs) != 2 {
		t.Fatal("started journal was consumed as a final acquisition", current.Phase)
	}
}
