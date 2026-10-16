package livetest

import (
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"testing"
)

func TestOriginalCompletionAllowsUnreportedOutcomeButRejectsStaleOrInterruptedFacts(t *testing.T) {
	original := orca.AgentState{AgentType: "codex", Prompt: "original", MainAgent: &orca.MainAgentState{State: "done", StateStartedAt: 11}, LastAssistantMessage: "actual original reply"}
	if !MainTurnCompleted(original, 10, "original") {
		t.Fatal("real native omission of optional outcome rejected")
	}
	tests := map[string]func(*orca.AgentState){"title-only": func(v *orca.AgentState) { v.State = "done"; v.MainAgent = nil }, "working": func(v *orca.AgentState) { v.MainAgent.State = "working" }, "stale": func(v *orca.AgentState) { v.MainAgent.StateStartedAt = 10 }, "wrong-prompt": func(v *orca.AgentState) { v.Prompt = "previous" }, "cancelled": func(v *orca.AgentState) { v.MainAgent.Outcome = "cancellation" }, "unknown-outcome": func(v *orca.AgentState) { v.MainAgent.Outcome = "unknown" }, "interrupted": func(v *orca.AgentState) { v.Interrupted = true }, "tool-output": func(v *orca.AgentState) { v.ToolOutput = true }, "boundary": func(v *orca.AgentState) { v.SessionBoundary = true }, "restored": func(v *orca.AgentState) { v.RestoredUnconfirmed = true }, "no-body": func(v *orca.AgentState) { v.LastAssistantMessage = "" }}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			v := original
			main := *original.MainAgent
			v.MainAgent = &main
			mutate(&v)
			if MainTurnCompleted(v, 10, "original") {
				t.Fatal("invalid completion accepted")
			}
		})
	}
}
