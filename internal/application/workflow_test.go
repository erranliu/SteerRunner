package application

import (
	"context"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type auditPort struct{ reportPolicyPort }

func (auditPort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, ResourceAvailable: true}, nil
}

type auditRestorePort struct{ auditPort }

func (auditRestorePort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, SessionRebindable: true, ProcessAbsent: true, WritersAbsent: true}, nil
}

func (auditRestorePort) BindRecovery(_ context.Context, _, next b.LifecycleSnapshot) (b.ImplementationDataEnvelope, error) {
	return b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: 1, Purpose: b.ImplementationResourcePurpose, ImplementationKey: "test", ProjectID: "project", ExecutionID: "execution", AttemptID: next.Session.AttemptID, Generation: next.ResourceGeneration, ConfigDigest: "config"}, []byte(`{}`))
}

func auditEngine(t *testing.T) (*Engine, *worker) {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	w := &worker{port: auditPort{reportPolicyPort{policy: "frozen_native_permissions; business_read_only_is_prompt_policy"}}, wake: make(chan struct{}, 1)}
	e := &Engine{dir: dir, ctx: context.Background(), epoch: "epoch", store: s, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}}
	p, _ := workflow.Lookup(workflow.AuditedDevelopment, 2)
	r := &runstate.Record{ID: "run", Allowed: true, CommandStates: map[string]string{}, Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: p.ID, Version: p.Version}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}}}, Materials: []runstate.Material{{MaterialView: contracts.MaterialView{Path: "spec", SnapshotPath: "spec"}}, {MaterialView: contracts.MaterialView{Path: "plan", SnapshotPath: "plan"}}}}
	for i := range r.Materials {
		m := &r.Materials[i]
		m.Content = "original fixture material"
		m.Digest = ra.Digest([]byte(m.Content))
		m.SnapshotPath = filepath.Join(dir, m.Path)
		if err := os.WriteFile(m.SnapshotPath, []byte(m.Content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	repo := t.TempDir()
	gitTest(t, repo, "init", "-b", "main")
	gitTest(t, repo, "config", "user.email", "fixture@example.test")
	gitTest(t, repo, "config", "user.name", "Fixture")
	if err := os.WriteFile(filepath.Join(repo, "code.txt"), []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "base")
	head := gitTest(t, repo, "rev-parse", "HEAD")
	r.RepositoryPath = repo
	r.RepositoryCommon = filepath.Join(repo, ".git")
	r.Snapshot.WorkspacePlan = b.WorkspaceIntent{Path: repo, Branch: "main", BaseCommit: head}
	r.Workflow.Config = &p
	r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: "executor", ReasoningEffort: "medium"}
	r.Workflow.AuditModel = &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}
	r.Snapshot.Session = b.SessionContext{SessionID: "session", AttemptID: "attempt"}
	bindTestWorkflow(r)
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	bindTestWorkflow(r)
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	return e, w
}

func finishStage(t *testing.T, e *Engine, body string) {
	t.Helper()
	r := e.runs["run"].Clone()
	input := r.Snapshot.PendingTurn
	if input == nil {
		t.Fatal("no pending turn")
	}
	r.Snapshot.Session.CurrentCommandID = input.CommandID
	r.Snapshot.Session.CurrentTurnID = "turn-" + input.CommandID
	e.runs["run"] = r
	w := e.workers["run"]
	if w.ownedInputs == nil {
		w.ownedInputs = map[string]bool{}
	}
	w.ownedInputs[input.CommandID] = true
	e.finish("run", b.TurnEvidence{CommandID: input.CommandID, SessionID: "session", TurnID: r.Snapshot.Session.CurrentTurnID, Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "item", LastMessage: body})
}

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func TestRAAuditLifecycle(t *testing.T) {
	e, _ := auditEngine(t)
	finishStage(t, e, "实施及必要验证已完成。")
	r := e.runs["run"]
	if r.Workflow.StepID != workflow.StepAuditCode || r.Snapshot.PendingTurn.Model != "reviewer" || len(r.Workflow.StageInputs) != 2 {
		t.Fatalf("audit not selected without report: %+v", r.Workflow)
	}
	repo := r.RepositoryPath
	if err := os.WriteFile(filepath.Join(repo, "code.txt"), []byte("repaired"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "repair")
	finishStage(t, e, `{"outcome":"done","summary":"audited repaired validated","fixed_count":1,"fixed_issues":[{"severity":"P1","issue":"bug","repair":"fix","evidence":"code.txt"}],"remaining_issues":[]}`)
	r = e.runs["run"]
	if r.Workflow.StepID != workflow.StepPublish || r.Snapshot.PendingTurn.Model != "executor" || !workflow.AuditsPassed(r.Workflow) || len(r.Workflow.Audits) != 1 {
		t.Fatalf("audit did not authorize exact publication: %s %s", r.Status, r.Diagnostic)
	}
	a := r.Workflow.Audits[0]
	if a.SourceResultID != r.Workflow.ImplementationResultID || a.PreHead == a.PostHead || a.FixedCount == nil || *a.FixedCount != 1 {
		t.Fatal(a)
	}
	if r.View().Final.CommandID == r.Snapshot.PendingTurn.CommandID {
		t.Fatal("queued publication replaced native final")
	}
	rows, err := e.store.Runs()
	if err != nil || len(rows) != 1 || rows[0].Workflow.AuditResultID != r.Workflow.AuditResultID {
		t.Fatal("source commit not durable", err)
	}
	// An independent commit invalidates the selected publication without re-audit.
	gitTest(t, repo, "commit", "--allow-empty", "-m", "independent change")
	pub := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
	if e.checkDispatchSource(r, *pub) == nil {
		t.Fatal("source drift still publishable")
	}
}
func TestRARecoveryBusinessInput(t *testing.T) {
	e, w := auditEngine(t)
	finishStage(t, e, "实施及必要验证已完成。")
	finishStage(t, e, `{"outcome":"needs_input","summary":"unfinished","question":"clarify"}`)
	r := e.runs["run"].Clone()
	before := len(r.Workflow.StageInputs)
	if r.Workflow.StepID != workflow.StepAuditInput || r.Question == nil || r.Snapshot.QuestionRoute != nil {
		t.Fatal("ended business input became native question")
	}
	if err := resumeWorkflow(r); err != nil || len(r.Workflow.StageInputs) != before || r.Snapshot.PendingTurn != nil {
		t.Fatal("resume re-audited", err)
	}
	if e.feedbackValue("run", w, w.port, &runstate.Answer{CommandID: "answer", Epoch: e.epoch, Text: "more material"}, true) {
		t.Fatal("audit answer authorized a new audit")
	}
	if !e.feedbackValue("run", w, w.port, &runstate.Answer{CommandID: "feedback", Epoch: e.epoch, Text: "Implement the requested change"}, false) {
		t.Fatal("explicit implementation not authorized", e.runs["run"].Diagnostic)
	}
	r = e.runs["run"]
	if r.Workflow.StepID != workflow.StepImplement || r.Workflow.ImplementationResultID != "" || r.Workflow.AuditResultID != "" {
		t.Fatal("new source inherited old eligibility")
	}
}
func TestRAVersionRejectLegacy(t *testing.T) {
	r := &runstate.Record{Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "development", Version: 1}}}
	if validateWorkflow(r) == nil {
		t.Fatal("legacy workflow silently accepted")
	}
}

// Tests construct the same frozen definition that submit now persists.
func bindTestWorkflow(r *runstate.Record) {
	if r.Request.Workflow.Version != 2 {
		return
	}
	if r.Snapshot.Ref.Version == 0 {
		r.Snapshot.Ref.Version = 1
	}
	if r.Workflow.Config == nil {
		p, ok := workflow.Lookup(r.Request.Workflow.ID, r.Request.Workflow.Version)
		if ok {
			r.Workflow.Config = &p
		}
	}
	if r.Workflow.StepID == "" && r.Workflow.Config != nil {
		r.Workflow.StepID = r.Workflow.Config.Initial
	}
}

func TestRAUnreportedOutcomeBoundary(t *testing.T) {
	for _, outcome := range []string{"unreported", "unknown", "failed", "cancelled", "business-blocked"} {
		t.Run(outcome, func(t *testing.T) {
			e, w := auditEngine(t)
			r := e.runs["run"].Clone()
			input := r.Snapshot.PendingTurn
			r.Snapshot.Session.CurrentCommandID, r.Snapshot.Session.CurrentTurnID = input.CommandID, "original-turn"
			e.runs["run"] = r
			w.ownedInputs = map[string]bool{input.CommandID: true}
			body, nativeOutcome := "实施及必要验证已完成。", outcome
			if outcome == "business-blocked" {
				nativeOutcome = "unreported"
				body = `{"outcome":"blocked","summary":"uncertain publication","blocker":{"category":"unknown_external","action":"publish","error":"lost response"}}`
			}
			e.finish("run", b.TurnEvidence{CommandID: input.CommandID, SessionID: "session", TurnID: "original-turn", NativeOutcome: nativeOutcome, Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "original-final", LastMessage: body})
			r = e.runs["run"]
			if original := workflow.Result(r.Workflow, input.CommandID); original == nil || original.Outcome != nativeOutcome || r.View().Final == nil || r.View().Final.NativeOutcome != nativeOutcome {
				t.Fatal("public native outcome was promoted", r.View())
			}
			if outcome == "unreported" {
				if r.Workflow.StepID != workflow.StepAuditCode || len(r.Workflow.StageInputs) != 2 {
					t.Fatal("verified result did not enter existing business routing", r.View())
				}
			} else if len(r.Workflow.StageInputs) != 1 || outcome != "business-blocked" && (!r.ResumeRequired || r.Phase != "original_session_recovery") || outcome == "business-blocked" && r.Status != "reconciling" {
				t.Fatal("blocked result authorized a successor", r.View())
			}
		})
	}
}
