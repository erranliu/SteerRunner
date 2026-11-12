package runstate

import (
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func TestRecordMessageAndWorkspaceAcquireEligibility(t *testing.T) {
	managed := Record{
		Request:          contracts.RunRequest{Workflow: contracts.Workflow{ID: "conversation", Version: 2}},
		Workflow:         workflow.Snapshot{StepID: "input"},
		Allowed:          true,
		Status:           "waiting_input",
		ManagedWorkspace: &ManagedWorkspace{Local: &b.WorkspaceIdentity{}},
		Snapshot: b.LifecycleSnapshot{Session: b.SessionContext{
			SessionID: "original-session", CurrentTurnID: "original-turn", LastStoppedTurnID: "original-turn",
		}},
	}
	ordinary := Record{
		Request:  contracts.RunRequest{Workflow: contracts.Workflow{ID: "conversation", Version: 1}},
		Workflow: workflow.Snapshot{StepID: "input"},
		Allowed:  true,
		Status:   "waiting_input",
	}
	development := Record{
		Request:  contracts.RunRequest{Workflow: contracts.Workflow{ID: "development", Version: 2}},
		Workflow: workflow.Snapshot{StepID: workflow.StepPRReady},
		Allowed:  true,
		Status:   "pr_ready",
	}
	pendingResource := operation.Intent{
		Request: b.LifecycleRequest{Operation: b.Operation{Status: b.OperationAccepted}},
		Receipt: &b.LifecycleReceipt{State: b.LifecycleStateFacts{ResourcePending: true}},
	}
	settledResource := pendingResource
	settledResource.Receipt = &b.LifecycleReceipt{}
	for _, tc := range []struct {
		name        string
		record      Record
		change      func(*Record)
		wantMessage bool
		wantAcquire bool
	}{
		{"ordinary conversation without a managed binding", ordinary, nil, true, false},
		{"ordinary conversation with a pending interrupt", ordinary, func(r *Record) { r.PendingInterrupt = "interrupt" }, false, false},
		{"completed development", development, nil, true, false},
		{"development with a pending stop", development, func(r *Record) { r.PendingStop = "stop" }, false, false},
		{"settled managed conversation", managed, nil, true, true},
		{"managed conversation with an active turn", managed, func(r *Record) { r.Snapshot.Session.LastStoppedTurnID = "" }, false, false},
		{"managed conversation without a worktree binding", managed, func(r *Record) { r.ManagedWorkspace = nil }, false, false},
		{"managed conversation with a held lease", managed, func(r *Record) { r.WorkspaceWriteLeaseID = "original-lease" }, false, false},
		{"managed conversation with an unresolved accepted operation", managed, func(r *Record) { r.Intents = []operation.Intent{pendingResource} }, false, false},
		{"managed conversation with a settled accepted operation", managed, func(r *Record) { r.Intents = []operation.Intent{settledResource} }, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.record
			if tc.change != nil {
				tc.change(&r)
			}
			view := r.View()
			if message := r.CanMessage(); message != tc.wantMessage || view.Controls.CanMessage != tc.wantMessage {
				t.Fatalf("message eligibility = %t, projected = %t, want %t", message, view.Controls.CanMessage, tc.wantMessage)
			}
			projectedAcquire := view.WorkspaceWrite != nil && view.WorkspaceWrite.CanAcquire
			if acquire := r.CanAcquireWorkspaceWrite(); acquire != tc.wantAcquire || projectedAcquire != tc.wantAcquire {
				t.Fatalf("acquire eligibility = %t, projected = %t, want %t", acquire, projectedAcquire, tc.wantAcquire)
			}
		})
	}
}
