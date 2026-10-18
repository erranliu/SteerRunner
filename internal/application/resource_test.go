package application

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

type pendingResourcePort struct {
	b.LifecyclePort
	confirm       bool
	executes      int
	observed      []string
	originalInput string
}

type stagePreparationPort struct {
	b.LifecyclePort
	actions []string
}

func (p *stagePreparationPort) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{Preparation: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: b.MechanismNotApplicable, Session: b.MechanismApplicable}, ReportToolPolicy: "host_managed_permissions; business_read_only_is_prompt_policy"}
}
func (p *stagePreparationPort) PreparationFacts(context.Context, b.LifecycleSnapshot) (b.PreparationFacts, error) {
	return b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateNotApplicable, Session: b.GateRequired}, nil
}
func (p *stagePreparationPort) Execute(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.actions = append(p.actions, request.Operation.Action)
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed}, nil, nil
}

func TestSRLongTurnCanPrepareNextStage(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	port := &stagePreparationPort{}
	w := &worker{port: port, wake: make(chan struct{}, 1)}
	e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}}
	r := &runstate.Record{ID: "run", Allowed: true, Status: "running", PreparationDeadline: time.Now().Add(-time.Minute), CommandStates: map[string]string{}, Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, PromptBodies: map[string]string{workflow.StepReport: "Format the original result as JSON."}, Materials: []runstate.Material{{MaterialView: contracts.MaterialView{Path: "spec.md"}}, {MaterialView: contracts.MaterialView{Path: "plan.md"}}}}
	r.Snapshot.Ref = b.ExecutionRef{RunID: "run", AttemptID: "attempt"}
	r.Request.Workflow = contracts.Workflow{ID: "development", Version: 2}
	r.Snapshot.Session = b.SessionContext{WorkflowRunID: "run", SessionID: "original-session", AttemptID: "attempt", CurrentCommandID: "command", CurrentTurnID: "turn"}
	r.Snapshot.Workspace.Path = "original-workspace"
	r.Workflow = workflow.Snapshot{StepID: workflow.StepImplement, StageInputs: []workflow.StageInput{{CommandID: "command", AttemptID: "attempt", StepID: workflow.StepImplement, Version: 1}}}
	bindTestWorkflow(r)
	if err := e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	evidence := b.TurnEvidence{CommandID: "command", SessionID: "original-session", TurnID: "turn", Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "final-item", LastMessage: "original verified result"}
	if !e.finish("run", evidence) || !e.advance("run", w) {
		t.Fatal("long business turn prevented report preparation", e.runs["run"].Phase)
	}
	next := e.runs["run"]
	if len(port.actions) != 1 || port.actions[0] != string(b.LifecyclePrepareSession) || next.Workflow.StepID != workflow.StepReport || next.Snapshot.Session.SessionID != "original-session" || !next.PreparationDeadline.After(time.Now()) {
		t.Fatal("report did not prepare on the original session with a fresh bounded budget")
	}
}

func (p *pendingResourcePort) Execute(_ context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.executes++
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationRejected}, nil, nil
}
func (p *pendingResourcePort) Reconcile(_ context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if r.Snapshot.Session.SessionID != "original-session" || r.Operation.LifecycleIdentity.InputBindingDigest != p.originalInput {
		return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}, nil, fmt.Errorf("accepted resource facts or original operation binding lost")
	}
	p.observed = append(p.observed, r.Operation.ID)
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationAccepted, State: b.LifecycleStateFacts{ResourceID: "original-terminal", ResourcePending: true}}
	if p.confirm {
		receipt.Disposition = b.OperationConfirmed
		receipt.State.ResourcePending = false
	}
	return receipt, nil, nil
}

func TestSRAcceptedResourceUsesOriginalOperation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "deadline"}[expired], func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			e := &Engine{ctx: context.Background(), store: store, runs: map[string]*runstate.Record{}, logger: log.New(io.Discard, "", 0)}
			r := &runstate.Record{ID: "run", Allowed: true, Status: "running", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, CommandStates: map[string]string{}}
			r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt"}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "run", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
			r.Request.Workflow = contracts.Workflow{ID: "development", Version: 2}
			bindTestWorkflow(r)
			if err := e.commitLocked(r, nil, "accepted"); err != nil {
				t.Fatal(err)
			}
			i, err := e.saveIntent(r.ID, r, b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Objective: "fixed original launch"}}})
			if err != nil {
				t.Fatal(err)
			}
			r = e.runs[r.ID].Clone()
			r.Intents[i].Request.Operation.Status = b.OperationAccepted
			r.Intents[i].Receipt = &b.LifecycleReceipt{Operation: r.Intents[i].Request.Operation, Disposition: b.OperationAccepted, State: b.LifecycleStateFacts{ResourceID: "original-terminal", ResourcePending: true}}
			r.Snapshot.Operations[i].Status = b.OperationAccepted
			r.Snapshot.Session.SessionID = "original-session"
			if expired {
				r.PreparationDeadline = time.Now().Add(-time.Second)
			}
			bindTestWorkflow(r)
			if err := e.commitLocked(r, nil, "resource_accepted"); err != nil {
				t.Fatal(err)
			}
			original := r.Intents[i].Request.Operation.ID
			port := &pendingResourcePort{originalInput: r.Intents[i].Request.Operation.LifecycleIdentity.InputBindingDigest}
			worker := &worker{port: port}
			for j := 0; j < 3; j++ {
				if e.advance(r.ID, worker) {
					t.Fatal("pending resource caused an immediate new preparation loop")
				}
			}
			if len(e.runs[r.ID].Intents) != 1 || port.executes != 0 || len(port.observed) != 3 {
				t.Fatal("accepted resource was recreated", len(e.runs[r.ID].Intents), port.executes, port.observed)
			}
			if e.runs[r.ID].Intents[i].Request.Snapshot.Session.SessionID != "" {
				t.Fatal("reconciliation rewrote the original intent snapshot")
			}
			for _, id := range port.observed {
				if id != original {
					t.Fatal("reconciliation replaced its original operation identity")
				}
			}
			port.confirm = true
			if !e.advance(r.ID, worker) || e.runs[r.ID].Intents[0].Request.Operation.Status != b.OperationConfirmed {
				t.Fatal("original resource readiness was not reconciled")
			}
			if expired && (e.runs[r.ID].Allowed || !e.runs[r.ID].ResumeRequired) {
				t.Fatal("late readiness revived an expired input authorization")
			}
		})
	}
}
