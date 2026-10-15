package application

import (
	"context"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"testing"
)

type reportPolicyPort struct {
	b.LifecyclePort
	policy string
}

func (p reportPolicyPort) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{ReportToolPolicy: p.policy}
}
func TestSRReportFreezesOriginatingNativePolicy(t *testing.T) {
	for _, policy := range []string{"frozen_native_permissions; business_read_only_is_prompt_policy", "host_managed_permissions; business_read_only_is_prompt_policy"} {
		t.Run(policy, func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			w := &worker{port: reportPolicyPort{policy: policy}, wake: make(chan struct{}, 1)}
			e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}}
			r := &runstate.Record{ID: "run", Allowed: true, Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, PromptBodies: map[string]string{workflow.StepReport: "Format the original result as JSON."}, Materials: []runstate.Material{{MaterialView: contracts.MaterialView{Path: "spec.md", SnapshotPath: "spec.md"}}, {MaterialView: contracts.MaterialView{Path: "plan.md", SnapshotPath: "plan.md"}}}}
			r.Snapshot.Session = b.SessionContext{SessionID: "original-session", AttemptID: "attempt", CurrentCommandID: "command", CurrentTurnID: "turn"}
			r.Workflow = workflow.Snapshot{StepID: workflow.StepImplement, StageInputs: []workflow.StageInput{{CommandID: "command", AttemptID: "attempt", StepID: workflow.StepImplement, Version: 1}}}
			if err := e.commitLocked(r, nil, "accepted"); err != nil {
				t.Fatal(err)
			}
			evidence := b.TurnEvidence{CommandID: "command", SessionID: "original-session", TurnID: "turn", Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "final-item", LastMessage: "original verified result"}
			if !e.finish("run", evidence) {
				t.Fatal("verified original result did not queue report")
			}
			next := e.runs["run"].Snapshot.PendingTurn
			if next == nil || next.StepID != workflow.StepReport || next.ReportToolPolicy != policy {
				t.Fatal("report lost originating native policy")
			}
			if err := reliable.QualifyInput(next, w.port.PolicyCapabilities()); err != nil {
				t.Fatal("next native preparation rejected frozen report", err)
			}
			changed := w.port.PolicyCapabilities()
			changed.ReportToolPolicy = "different-policy"
			if reliable.QualifyInput(next, changed) == nil {
				t.Fatal("a changed native policy was accepted")
			}
		})
	}
}
