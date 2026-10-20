package orcatransport

import (
	"testing"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
)

func TestFixedControlReceiptBindsFrozenTurnModel(t *testing.T) {
	_, _, _, owner := fixtureFixedControl(t)
	state := State{RemoteReadiness: &owner.ready}
	turn := Turn{Command: "original-command", Prompt: "original input", InputDigest: digest([]byte("original input")), StepID: "audit.code.execute", Model: "reviewer", ReasoningEffort: "high", WireFormat: remoteModelWireCommand, Accepted: true}
	for _, mode := range []string{"original", "writer-model", "changed-effort", "reader-live", "foreign-owner", "foreign-input"} {
		t.Run(mode, func(t *testing.T) {
			value := codex.RemoteTurnReceipt{Readiness: owner.ready, ReaderWritersAbsent: true, Source: codex.RemoteTurnSource, InputDigest: digest([]byte(wirePrompt(&turn))), Started: true, Completed: true, Status: "completed", Model: "reviewer", ReasoningEffort: "high", NativeTurnID: "01a14502-5798-78f1-aaa3-2d7cd15594c8", UserItemID: "original-user", FinalItemID: "original-final", FinalText: "original audit", FinalDigest: digest([]byte("original audit"))}
			switch mode {
			case "writer-model":
				value.Model = "writer"
			case "changed-effort":
				value.ReasoningEffort = "medium"
			case "reader-live":
				value.ReaderWritersAbsent = false
			case "foreign-owner":
				value.Readiness.Owner.Created = "replacement"
			case "foreign-input":
				value.InputDigest = digest([]byte("another input"))
			}
			if err := validateRecordedTurn(state, &turn, value); (err == nil) != (mode == "original") {
				t.Fatalf("mode=%s receipt accepted=%v: %v", mode, err == nil, err)
			}
		})
	}
}
