package application

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
)

func TestSRNewIntentBoundaryRejectsNewButReconcilesSaved(t *testing.T) {
	cases := []struct {
		action b.LifecycleAction
		input  b.LifecycleInput
		value  any
	}{
		{action: b.LifecycleReportCompletion, input: b.LifecycleInput{Completion: &b.CompletionInput{TurnID: "original-turn", Outcome: "succeeded", Subject: "saved", Body: "saved"}}},
		{action: b.LifecycleAcknowledge, input: b.LifecycleInput{Ack: &b.AcknowledgeInput{DeliveryID: "original-delivery"}}},
		{action: b.LifecycleCleanup, input: b.LifecycleInput{Cleanup: &b.CleanupInput{Resources: []b.ProcessIdentity{{Kind: "os", PID: 7, Created: "original"}}}}},
		{action: b.LifecycleHostAction, input: b.LifecycleInput{Host: &b.HostActionInput{Kind: b.HostRemoveFeedback, Worktree: t.TempDir(), WorkflowRun: "run", TurnID: "original-turn"}}},
	}
	// Encode the original on-disk union members directly. New must never be used
	// to reconstruct a previously committed request after its admission narrows.
	cases[0].value, cases[1].value = cases[0].input.Completion, cases[1].input.Ack
	cases[2].value, cases[3].value = cases[2].input.Cleanup, cases[3].input.Host
	for _, tc := range cases {
		t.Run(string(tc.action), func(t *testing.T) {
			e, w, db := operationEngine(t)
			candidate := b.LifecycleCandidate{Action: tc.action, Input: tc.input}
			if err := candidate.Validate(); err != nil {
				t.Fatalf("fixture must be a valid original input: %v", err)
			}
			before, err := json.Marshal(e.runs["run"])
			if err != nil {
				t.Fatal(err)
			}
			var body []byte
			var events int
			if err := db.QueryRow("SELECT body FROM runs WHERE id='run'").Scan(&body); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT count(*) FROM events").Scan(&events); err != nil {
				t.Fatal(err)
			}
			if _, err := e.saveIntent("run", e.runs["run"].Clone(), candidate); err == nil || err.Error() != "unsupported lifecycle operation action" {
				t.Fatalf("valid old action reached new intent construction: %v", err)
			}
			after, err := json.Marshal(e.runs["run"])
			if err != nil {
				t.Fatal(err)
			}
			var bodyAfter []byte
			var eventsAfter int
			if err := db.QueryRow("SELECT body FROM runs WHERE id='run'").Scan(&bodyAfter); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT count(*) FROM events").Scan(&eventsAfter); err != nil {
				t.Fatal(err)
			}
			p := w.port.(*operationCommitPort)
			if !bytes.Equal(before, after) || !bytes.Equal(body, bodyAfter) || events != eventsAfter || p.executes != 0 || p.reconciles != 0 || p.stops != 0 {
				t.Fatal("new intent rejection changed memory, SQLite or native call counts")
			}

			r := e.runs["run"].Clone()
			snapshot := r.Snapshot.Clone()
			ref := snapshot.Ref
			payload, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			op := b.Operation{Version: 1, ID: "original-operation", RunID: r.ID, AttemptID: snapshot.Session.AttemptID, Backend: ref.Backend, Protocol: ref.Protocol, Target: ref.Target, Action: string(tc.action), IdempotencyKey: "original-key", Request: payload, PayloadDigest: reliable.Digest(payload), Status: b.OperationUnknown, StartedAt: now, UpdatedAt: now, Ref: &ref,
				LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: snapshot.Session.ProjectID, ExecutionID: snapshot.Session.ExecutionID, ConfigurationDigest: snapshot.ConfigurationDigest, ResourceGeneration: snapshot.ResourceGeneration, InputBindingDigest: b.LifecycleBindingDigest(snapshot), Ref: ref}}
			decoded, err := b.CandidateForOperation(op)
			if err != nil || !reflect.DeepEqual(decoded.Input, tc.input) {
				t.Fatalf("original saved payload no longer decodes: %v", err)
			}
			request := b.LifecycleRequest{Snapshot: snapshot, Operation: op, Input: decoded.Input}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := (b.LifecycleReceipt{Operation: op, Disposition: b.OperationUnknown}).ValidateFor(request); err != nil {
				t.Fatal(err)
			}
			malformed := op
			malformed.Request = append(append([]byte(nil), payload...), []byte(" {}")...)
			if _, err := b.CandidateForOperation(malformed); err == nil {
				t.Fatal("saved decoder accepted trailing data")
			}
			r.Intents = []operation.Intent{{Request: request, Mode: string(operation.ReconcileOriginalIntent)}}
			r.Snapshot.Operations = []b.Operation{op}
			e.mu.Lock()
			err = e.commitLocked(r, nil, "original_fixture_saved")
			e.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := e.store.Runs()
			if err != nil || len(loaded) != 1 {
				t.Fatalf("original record could not reload: %v", err)
			}
			e.runs["run"] = loaded[0]
			if !e.advance("run", w) {
				t.Fatal("new writing restriction blocked the original reconciliation path")
			}
			settled := e.runs["run"].Intents[0]
			if p.executes != 0 || p.reconciles != 1 || p.stops != 0 || len(p.requests) != 1 || p.requests[0].Operation.ID != op.ID || !bytes.Equal(p.requests[0].Operation.Request, payload) || !reflect.DeepEqual(p.requests[0].Input, tc.input) || settled.Request.Operation.Status != b.OperationConfirmed || !bytes.Equal(settled.Request.Operation.Request, payload) {
				t.Fatal("original reconciliation was resent, rebound or reconstructed")
			}
			if err := settled.Receipt.ValidateFor(settled.Request); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSRNewIntentBoundaryKeepsSupportedPayloads(t *testing.T) {
	turn := &b.TurnInput{CommandID: "original-command", Prompt: "original-input"}
	cases := []struct {
		action b.LifecycleAction
		input  b.LifecycleInput
		value  any
	}{
		{b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &b.WorkspaceIntent{Path: "original-workspace"}}, &b.WorkspaceIntent{Path: "original-workspace"}},
		{b.LifecycleCreateResource, b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start"}}, &b.ResourceIntent{Purpose: "agent-start"}},
		{b.LifecyclePrepareSession, b.LifecycleInput{Turn: turn}, turn},
		{b.LifecycleStartTurn, b.LifecycleInput{Turn: turn}, turn},
		{b.LifecycleSubmitInput, b.LifecycleInput{Turn: turn}, turn},
		{b.LifecycleDeliverFeedback, b.LifecycleInput{Feedback: &b.FeedbackInput{Message: "original-answer"}}, &b.FeedbackInput{Message: "original-answer"}},
		{b.LifecycleStopObservation, b.LifecycleInput{Stop: &b.StopInput{TurnID: "original-turn"}}, &b.StopInput{TurnID: "original-turn"}},
		{b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{TurnID: "original-turn"}}, &b.StopInput{TurnID: "original-turn"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.action), func(t *testing.T) {
			e, _, _ := operationEngine(t)
			index, err := e.saveIntent("run", e.runs["run"].Clone(), b.LifecycleCandidate{Action: tc.action, Input: tc.input})
			if err != nil {
				t.Fatal(err)
			}
			expected, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			intent := e.runs["run"].Intents[index]
			if !bytes.Equal(intent.Request.Operation.Request, expected) || intent.Request.Operation.PayloadDigest != reliable.Digest(expected) || len(e.runs["run"].Snapshot.Operations) != 1 {
				t.Fatal("supported new intent payload or durable association changed")
			}
		})
	}
}
