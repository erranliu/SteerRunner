package application

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"io"
	"log"
	"reflect"
	"testing"
)

// Records the actual recovery boundary without implementing input rebuilding.
type frozenResumePort struct {
	auditPort
	mode                                 string
	queries, executes, reconciles, stops int
	verified                             b.LifecycleSnapshot
}

func (p *frozenResumePort) VerifyManualRecovery(_ context.Context, snapshot b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	p.queries++
	p.verified = snapshot.Clone()
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: p.mode != "busy", ResourceAvailable: p.mode != "lost", ProcessAbsent: p.mode == "lost"}, nil
}
func (p *frozenResumePort) Execute(context.Context, b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.executes++
	return b.LifecycleReceipt{}, nil, nil
}
func (p *frozenResumePort) Reconcile(context.Context, b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	return b.LifecycleReceipt{}, nil, nil
}
func (p *frozenResumePort) StopLifecycle(context.Context, b.LifecycleRequest, b.StopHandle) (b.LifecycleReceipt, error) {
	p.stops++
	return b.LifecycleReceipt{}, nil
}

func TestSRResumeWorkflowRetainsOriginalReportSource(t *testing.T) {
	for _, mode := range []string{"available", "lost", "busy"} {
		t.Run(mode, func(t *testing.T) {
			e, _ := auditEngine(t)
			finishStage(t, e, "unclassified original implementation")
			r := e.runs["run"].Clone()
			input := workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
			if input == nil || input.StepID != workflow.StepReport || input.SourceCommandID == "" || input.SourceTurnID == "" || input.SourceVersion == 0 || input.SourceResultID == "" || input.ReportToolPolicy == "" || input.Budget == nil || input.Model == "" || len(r.Intents) != 0 {
				t.Fatal("fixture did not freeze a legal unsent report", input)
			}
			r.Allowed, r.ResumeRequired = false, true
			r.Status, r.Phase = "blocked", "paused_before_report"
			if err := e.commitLocked(r, nil, "paused_before_report"); err != nil {
				t.Fatal(err)
			}
			before := e.runs["run"].Clone()
			reloaded, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
			if err != nil {
				t.Fatal(err)
			}
			defer reloaded.cancel()
			w := reloaded.workers["run"]
			if string(canonical(before)) != string(canonical(reloaded.runs["run"])) || w.port != nil {
				t.Fatal("service reload rewrote the original report or constructed an executor")
			}
			p := &frozenResumePort{mode: mode}
			w.port = p
			_, command := manualResume(t, reloaded, "resume-report")
			accepted := reloaded.runs["run"].Clone()
			if accepted.PendingResume != command || accepted.CommandStates[command] != "pending" || p.queries != 0 || p.executes != 0 {
				t.Fatal("resume acceptance performed recovery or dispatched input")
			}
			w.gate.Lock()
			resumed := reloaded.resumeRun("run", w, p)
			w.gate.Unlock()
			after := reloaded.runs["run"]
			if resumed != (mode == "available") || p.queries != 1 || p.executes != 0 || p.reconciles != 0 || p.stops != 0 || !reflect.DeepEqual(p.verified, accepted.Snapshot.Clone()) {
				t.Fatal("resume did not use only the original recovery query", resumed, p)
			}
			if !reflect.DeepEqual(before.Snapshot, after.Snapshot) || !reflect.DeepEqual(before.Workflow, after.Workflow) || !reflect.DeepEqual(before.Intents, after.Intents) || !reflect.DeepEqual(before.Result, after.Result) || after.NextStage != nil {
				t.Fatal("resume changed frozen report identity, source, budget, model or policy")
			}
			expectedRejectionPhase := "control_rejected"
			if mode == "lost" {
				expectedRejectionPhase = manualIntervention
			}
			if (after.ManualPause != nil) != (mode == "lost") {
				t.Fatal("lost-resource recovery did not retain its manual boundary")
			}
			if mode == "available" {
				if after.CommandStates[command] != "completed" || !after.Allowed || after.ResumeRequired || after.PendingResume != "" {
					t.Fatal("qualified resume did not commit its authorization and command result")
				}
			} else if after.CommandStates[command] != "failed" || after.Allowed || !after.ResumeRequired || after.PendingResume != "" || after.Phase != expectedRejectionPhase || after.Diagnostic == "" {
				t.Fatal("unavailable original resource was not rejected by recovery qualification", after.CommandStates[command], after.Allowed, after.ResumeRequired, after.PendingResume, after.Phase, after.Diagnostic)
			}
			rows, err := reloaded.store.Runs()
			if err != nil || len(rows) != 1 || string(canonical(rows[0])) != string(canonical(after)) {
				t.Fatal("resume published a result different from the durable record", err)
			}
			t.Logf("mode=%s reload=New resume=resumeRun recovery_queries=%d executes=%d reconciles=%d stops=%d command_state=%s", mode, p.queries, p.executes, p.reconciles, p.stops, after.CommandStates[command])
		})
	}
}
