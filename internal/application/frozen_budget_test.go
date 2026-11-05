package application

import (
	"context"
	"database/sql"
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func assertFrozenStageBudgets(t *testing.T, r *runstate.Record) {
	t.Helper()
	if len(r.Workflow.StageInputs) == 0 {
		t.Fatal("no production-frozen stage")
	}
	for _, input := range r.Workflow.StageInputs {
		if input.Budget == nil {
			t.Fatal("production writer omitted budget", input.StepID)
		}
		if err := ensureStageBudget(r, &input); err != nil {
			t.Fatal("frozen production stage lost budget", input.StepID, err)
		}
	}
	if p := r.Snapshot.PendingTurn; p != nil {
		i := workflow.Input(r.Workflow, p.CommandID)
		if i == nil || p.Prompt != i.Prompt || !reflect.DeepEqual(p.Budget, i.Budget) {
			t.Fatal("native budget differs from frozen stage")
		}
	}
}

func TestFrozenBudgetLegalWorkflowEntries(t *testing.T) {
	for _, id := range []string{"development", workflow.AuditedDevelopment, workflow.SingleExecution, "conversation", "generate"} {
		adapters := []string{"codex-app-server", "orca-terminal"}
		if workflow.Content(id) {
			adapters = adapters[:1]
		}
		for _, adapter := range adapters {
			t.Run(id+"/"+adapter, func(t *testing.T) {
				var r *runstate.Record
				if workflow.Content(id) {
					material := "quoted \"材料\"\r\n😀"
					docs := []contracts.Document{{Kind: "inline", Path: "source.txt", Content: &material}}
					r = &runstate.Record{ID: "run", InputDigest: "config", Allowed: true, CommandStates: map[string]string{}, Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: id, Version: 1}, Execution: contracts.Execution{Adapter: adapter, Settings: json.RawMessage(`{"access_mode":"read_only"}`)}}}
					if id == "generate" {
						r.Request.Generate = &contracts.GenerateInput{Instruction: "整理材料", Materials: docs, OutputSchema: json.RawMessage(`{"type":"string"}`)}
					} else {
						r.Request.Conversation = &contracts.ConversationInput{Instruction: "讨论材料", Materials: docs, Directory: contracts.DirectoryAccess{Path: t.TempDir(), Access: "read_only"}}
					}
					var err error
					r, err = freezeContent(context.Background(), t.TempDir(), r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"}))
					if err != nil {
						t.Fatal(err)
					}
				} else {
					r = budgetRecord()
					version := 2
					if id == workflow.SingleExecution {
						version = 1
					}
					r.ID = "run"
					r.Request.Workflow = contracts.Workflow{ID: id, Version: version}
					r.Request.Execution.Adapter = adapter
					p, _ := workflow.Lookup(id, version)
					r.Workflow = workflow.Snapshot{Config: &p, StepID: p.Initial}
					if id == workflow.AuditedDevelopment {
						r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: "executor", ReasoningEffort: "medium"}
						r.Workflow.AuditModel = &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}
					}
					key := p.States[p.Initial].PromptKey
					r.PromptBodies = map[string]string{key: "执行 \"原文\"\r\n😀\n"}
					r.Request.Workflow.PromptOverrides = map[string]string{key: r.PromptBodies[key]}
					r.Request.ProgressFile = &contracts.ProgressFileInput{}
					r.Request.ResultFile = &contracts.ResultFileInput{}
					r.Snapshot.Session.AttemptID = "attempt"
				}
				if err := queueInput(r, workflow.StageInput{StepID: r.Workflow.Config.Initial}, ""); err != nil {
					t.Fatal(err)
				}
				assertFrozenStageBudgets(t, r)
				e, _, _ := operationEngine(t)
				if err := e.commitLocked(r, nil, "production_budget"); err != nil {
					t.Fatal(err)
				}
				rows, err := e.store.Runs()
				if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0].Workflow.StageInputs, r.Workflow.StageInputs) {
					t.Fatal("SQLite changed original stage budget", err)
				}
				assertFrozenStageBudgets(t, rows[0].Clone())
				if !workflow.Content(id) {
					original := rows[0].Clone()
					text, budget := original.Snapshot.PendingTurn.Prompt, original.Snapshot.PendingTurn.Budget.Clone()
					preservePendingStage(original)
					if original.NextStage == nil || original.NextStage.Budget == nil {
						t.Fatal("preserved original stage lost budget")
					}
					original.PromptBodies = map[string]string{}
					if err := resumeWorkflow(original); err != nil || original.Snapshot.PendingTurn.Prompt != text || !reflect.DeepEqual(original.Snapshot.PendingTurn.Budget, budget) {
						t.Fatal("restored input rerendered or reconstructed", err)
					}
				}
			})
		}
	}
}

func TestFrozenBudgetSuccessorsAndDraftResume(t *testing.T) {
	for _, reported := range []bool{false, true} {
		t.Run(map[bool]string{false: "report", true: "audit-draft"}[reported], func(t *testing.T) {
			e, _ := auditEngine(t)
			if reported {
				r := e.runs["run"].Clone()
				r.Allowed, r.ResumeRequired = false, true
				e.runs["run"] = r
				finishStage(t, e, "实施及必要验证已完成")
				r = e.runs["run"].Clone()
				if r.NextStage == nil || r.NextStage.Prompt != "" || r.NextStage.Budget != nil {
					t.Fatal("successor is not an unrendered draft")
				}
				if err := resumeWorkflow(r); err != nil {
					t.Fatal(err)
				}
				assertFrozenStageBudgets(t, r)
			} else {
				finishStage(t, e, "Unstructured native final")
				if e.runs["run"].Workflow.StepID != workflow.StepReport {
					t.Fatal("production report route not exercised")
				}
				assertFrozenStageBudgets(t, e.runs["run"])
			}
		})
	}
}

func TestFrozenBudgetReadsSavedLegacySources(t *testing.T) {
	for _, source := range []string{"legacy.wrapper", "legacy.content.label", "legacy.content.frame", "legacy.content.wrapper", "legacy.parser"} {
		text, budget, err := m.DescribeInput(m.InputPart{Source: source, Owner: m.RunnerInput, Text: "旧来源😀"})
		if err != nil {
			t.Fatal(err)
		}
		r := budgetRecord()
		i := workflow.StageInput{Prompt: text, Budget: budget}
		if err := ensureStageBudget(r, &i); err != nil || !reflect.DeepEqual(i.Budget, budget) {
			t.Fatal("saved legacy provenance changed", source, err)
		}
	}
}

// Use the production renderer and durable store before damaging one saved field.
func frozenBudgetEngine(t *testing.T) (*Engine, *worker, *sql.DB) {
	t.Helper()
	e, w, db := operationEngine(t)
	r := e.runs["run"].Clone()
	r.PromptBodies = map[string]string{workflow.StepImplement: workflow.ImplementPrompt}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	if err := e.commitLocked(r, nil, "original_input"); err != nil {
		t.Fatal(err)
	}
	return e, w, db
}

func TestFrozenBudgetCopyRequiresOriginalUnsentBinding(t *testing.T) {
	for _, mode := range []string{"copy", "stage-missing", "source-budget", "both-budgets", "different-copy", "command", "step", "version", "attempt", "prompt", "model", "effort", "policy", "sha", "provenance", "accounting", "committed-input", "save-failure"} {
		t.Run(mode, func(t *testing.T) {
			e, w, db := frozenBudgetEngine(t)
			r := e.runs["run"].Clone()
			original := r.Snapshot.PendingTurn.Budget.Clone()
			r.Snapshot.PendingTurn.Budget = nil
			i := &r.Workflow.StageInputs[0]
			switch mode {
			case "stage-missing":
				r.Workflow.StageInputs = nil
			case "source-budget", "both-budgets":
				i.Budget = nil
			case "different-copy":
				r.Snapshot.PendingTurn.Budget = original.Clone()
				r.Snapshot.PendingTurn.Budget.CallerBytes++
			case "command":
				i.CommandID = "different"
			case "step":
				i.StepID = "different"
			case "version":
				i.Version++
			case "attempt":
				i.AttemptID = "different"
			case "prompt":
				i.Prompt += "different"
			case "model":
				i.Model = "different"
			case "effort":
				i.ReasoningEffort = "different"
			case "policy":
				i.ReportToolPolicy = "different"
			case "sha":
				i.Budget.SHA256 = "different"
			case "provenance":
				i.Budget.ProvenanceSHA256 = "different"
			case "accounting":
				i.Budget.CallerBytes++
			case "committed-input":
				snapshot := r.Snapshot.Clone()
				snapshot.PendingTurn.Budget = original.Clone()
				intent, err := operation.New("run", snapshot, b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: snapshot.PendingTurn}}, "original-operation", "original-key", time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
				intent.Request.Operation.Status = b.OperationConfirmed
				r.Intents = append(r.Intents, intent)
			}
			if mode == "source-budget" {
				r.Snapshot.PendingTurn.Budget = original.Clone()
			}
			if mode == "source-budget" {
				before := r.Clone()
				if err := queueInput(r, *i, ""); err == nil || !reflect.DeepEqual(before, r) {
					t.Fatal("missing source budget queued or changed original")
				}
			}
			if err := e.commitLocked(r, nil, "damaged_copy_fixture"); err != nil {
				t.Fatal(err)
			}
			before := e.runs["run"].Clone()
			if mode == "save-failure" {
				// Trigger the real Store transaction failure, with no production seam.
				if _, err := db.Exec(`CREATE TRIGGER fail_budget BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'budget save failed'); END;`); err != nil {
					t.Fatal(err)
				}
			}
			got := e.prepareRun("run", w, w.port, r.Clone())
			if got != (mode == "copy") {
				t.Fatal("unexpected preparation", got)
			}
			p := w.port.(*operationCommitPort)
			if p.executes != 0 || p.reconciles != 0 || p.stops != 0 {
				t.Fatal("budget validation dispatched native effect", p)
			}
			after := e.runs["run"]
			if mode == "copy" {
				if !reflect.DeepEqual(after.Snapshot.PendingTurn.Budget, original) || after.Snapshot.PendingTurn.Prompt != before.Snapshot.PendingTurn.Prompt {
					t.Fatal("saved budget copy changed original")
				}
			} else if !reflect.DeepEqual(after.Snapshot.PendingTurn.Budget, before.Snapshot.PendingTurn.Budget) {
				t.Fatal("bad source restored budget")
			}
			if !reflect.DeepEqual(before.Intents, after.Intents) || !reflect.DeepEqual(before.Workflow.StageInputs, after.Workflow.StageInputs) {
				t.Fatal("original stage or operation rewritten")
			}
			if mode == "save-failure" && !reflect.DeepEqual(before, after) {
				t.Fatal("failed persistence published state")
			}
			rows, err := e.store.Runs()
			if err != nil || !reflect.DeepEqual(rows[0].Snapshot.PendingTurn.Budget, after.Snapshot.PendingTurn.Budget) {
				t.Fatal("durable budget differs", err)
			}
		})
	}
}

type frozenBudgetQueryPort struct {
	partialReceiptPort
	resourceReady bool
}

func (p *frozenBudgetQueryPort) Reconcile(ctx context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if p.resourceReady && request.Operation.Action == string(b.LifecycleCreateResource) {
		p.reconciles++
		p.requests = append(p.requests, request.Clone())
		original := b.ProcessIdentity{Kind: "orca_terminal", Terminal: "original-terminal"}
		return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, Process: &original}, nil, nil
	}
	return p.partialReceiptPort.Reconcile(ctx, request)
}

func TestFrozenBudgetUnknownReconcileAndStopDoNotRepair(t *testing.T) {
	for _, status := range []b.OperationStatus{b.OperationUnknown, b.OperationAccepted} {
		t.Run(string(status), func(t *testing.T) {
			e, w, _ := frozenBudgetEngine(t)
			r := e.runs["run"].Clone()
			candidate := b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: r.Snapshot.PendingTurn}}
			if status == b.OperationAccepted {
				// ResourcePending is legal only on an accepted resource creation.
				// Accepted business input is consumed and observed, covered separately.
				candidate = b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Objective: "original"}}}
			}
			intent, err := operation.New("run", r.Snapshot, candidate, "original-operation", "original-key", time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			intent.Request.Operation.Status = status
			if status == b.OperationAccepted {
				intent.Receipt = &b.LifecycleReceipt{Operation: intent.Request.Operation, Disposition: b.OperationAccepted, State: b.LifecycleStateFacts{ResourcePending: true, ResourceID: "original-resource"}}
				if err := intent.Receipt.ValidateFor(intent.Request); err != nil {
					t.Fatal("invalid accepted resource fixture", err)
				}
			}
			r.Intents = append(r.Intents, intent)
			r.Snapshot.PendingTurn.Budget, r.Workflow.StageInputs[0].Budget = nil, nil
			if err := e.commitLocked(r, nil, "missing_budget_fixture"); err != nil {
				t.Fatal(err)
			}
			p := &frozenBudgetQueryPort{}
			w.port = p
			e.advance("run", w)
			if p.executes != 0 || p.reconciles != 1 || len(p.requests) != 1 {
				t.Fatal("missing budget blocked original query or resent", p)
			}
			if string(p.requests[0].Operation.Request) != string(intent.Request.Operation.Request) || p.requests[0].Operation.PayloadDigest != intent.Request.Operation.PayloadDigest {
				t.Fatal("query rewrote original payload")
			}
			r = e.runs["run"].Clone()
			if r.Snapshot.PendingTurn.Budget != nil || r.Workflow.StageInputs[0].Budget != nil {
				t.Fatal("query repaired budget")
			}
			r.PendingStop = "stop-original"
			if err := e.commitLocked(r, nil, "stop_fixture"); err != nil {
				t.Fatal(err)
			}
			p.resourceReady = true
			if status == b.OperationAccepted {
				// Stop first commits the original creation's late identity, then
				// addresses it. Neither query can resend the resource creation.
				e.advance("run", w)
				if p.stops != 0 || p.reconciles != 2 {
					t.Fatal("stop skipped original creation reconciliation", p)
				}
			}
			e.advance("run", w)
			if p.stops != 1 || p.executes != 0 {
				t.Fatal("stop blocked by missing budget", p)
			}
			if r := e.runs["run"]; r.Workflow.StageInputs[0].Budget != nil || !strings.Contains(r.StopStatus, "stopped") {
				t.Fatal("stop changed budget or failed", r.StopStatus)
			}
		})
	}
}

func TestFrozenBudgetFirstRenderCommitFailureDoesNotPublish(t *testing.T) {
	e, w, db := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Snapshot.Workspace.Path = t.TempDir()
	r.PromptBodies = map[string]string{workflow.StepImplement: workflow.ImplementPrompt}
	if err := e.commitLocked(r, nil, "workspace_fixture"); err != nil {
		t.Fatal(err)
	}
	before := e.runs["run"].Clone()
	failOperationCommit(t, db)
	if e.prepareRun("run", w, w.port, r.Clone()) || !reflect.DeepEqual(before, e.runs["run"]) {
		t.Fatal("failed first freeze published state")
	}
	if p := w.port.(*operationCommitPort); p.executes != 0 {
		t.Fatal("failed first freeze dispatched", p)
	}
	rows, err := e.store.Runs()
	if err != nil || len(rows[0].Workflow.StageInputs) != 0 || rows[0].Snapshot.PendingTurn != nil {
		t.Fatal("failed first freeze saved input", err)
	}
}
