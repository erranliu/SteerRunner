package livetest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func preSendFixture() (PreSendGate, []byte) {
	at := time.Unix(100, 0).UTC()
	gate := PreSendGate{CallID: "original-call", Terminal: "original", ReturnedAt: at, View: contracts.RunView{RunID: "run", Step: workflow.StepPublish, Status: "running", NewInputsAllowed: true, Execution: contracts.ExecutionView{SessionID: "orca-terminal:runtime:original:incarnation", AttemptID: "attempt", Generation: "generation"}, Operations: []contracts.OperationView{{ID: "operation", Action: "start_turn", Status: "in_flight"}}}}
	receipt, _ := json.Marshal(map[string]any{"kind": "receipt", "call_id": gate.CallID, "args": []string{"terminal", "show", "--terminal", gate.Terminal}, "ended_at": at.Add(time.Millisecond)})
	intent, _ := json.Marshal(map[string]any{"kind": "intent", "call_id": "retry", "args": []string{"terminal", "show", "--terminal", gate.Terminal}, "at": at.Add(6 * time.Second)})
	return gate, append(append(receipt, '\n'), intent...)
}

func TestPublicationPreSendRetryEvidence(t *testing.T) {
	gate, audit := preSendFixture()
	if !PreSendReadRetried(audit, gate) {
		t.Fatal("valid retry rejected")
	}
	for _, scenario := range []string{"legacy-step", "report-step", "implementation-step", "prepare", "attempted", "identity", "unauthorized", "same-call", "too-early", "wrong-terminal", "no-receipt", "sent"} {
		t.Run(scenario, func(t *testing.T) {
			gate, audit := preSendFixture()
			switch scenario {
			case "legacy-step":
				gate.View.Step = "publish"
			case "report-step":
				gate.View.Step = workflow.StepPublishReport
			case "implementation-step":
				gate.View.Step = workflow.StepImplement
			case "prepare":
				gate.View.Operations[0].Action = "prepare_session"
			case "attempted":
				gate.View.Execution.TurnStarted = true
			case "identity":
				gate.View.Execution.SessionID = "orca-terminal:runtime:other:incarnation"
			case "unauthorized":
				gate.View.NewInputsAllowed = false
			case "same-call":
				gate.CallID = "retry"
			case "too-early":
				gate.ReturnedAt = gate.ReturnedAt.Add(2 * time.Second)
			case "wrong-terminal":
				gate.Terminal = "other"
			case "no-receipt":
				audit = audit[len(audit)/2:]
			case "sent":
				send, _ := json.Marshal(map[string]any{"kind": "intent", "call_id": "send", "args": []string{"terminal", "send", "--terminal", gate.Terminal}, "at": gate.ReturnedAt.Add(time.Second)})
				audit = append(append(audit, '\n'), send...)
			}
			if PreSendReadRetried(audit, gate) {
				t.Fatal("false retry proof accepted")
			}
		})
	}
}

func TestPreSendEvidenceRequiresPhysicalBlockAndOriginalOperation(t *testing.T) {
	for _, scenario := range []string{"valid", "scope", "late-block", "replacement", "revoked", "missing-audit"} {
		t.Run(scenario, func(t *testing.T) {
			m := Manifest{Root: t.TempDir(), RunnerExe: "runner.exe"}
			root := filepath.Join(m.Root, "evidence")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			gate, audit := preSendFixture()
			view := gate.View
			view.Operations = append([]contracts.OperationView(nil), gate.View.Operations...)
			programs := []string{m.RunnerExe}
			blocked := gate.ReturnedAt.Add(-time.Second)
			switch scenario {
			case "scope":
				programs = append(programs, "shared-node.exe")
			case "late-block":
				blocked = gate.ReturnedAt.Add(time.Second)
			case "replacement":
				view.Operations[0].ID = "replacement"
			case "revoked":
				view.NewInputsAllowed = false
			case "missing-audit":
				audit = nil
			}
			for name, value := range map[string]any{"pre-send-returned.json": gate, "pre-send-blocked-run.json": view, "network-intent.json": map[string]any{"programs": programs}, "network-blocked.json": map[string]any{"at": blocked}} {
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "pre-send-retry-audit.jsonl"), audit, 0600); err != nil {
				t.Fatal(err)
			}
			if err := validatePreSendEvidence(m); (err == nil) != (scenario == "valid") {
				t.Fatal("invalid evidence verdict", err)
			}
		})
	}
}
