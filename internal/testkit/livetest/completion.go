package livetest

import orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"

// MainTurnCompleted follows the existing native contract: omitted outcome remains
// unreported, while a later original main-agent done and original body prove completion.
// Identity is independently fenced by the caller before invoking this predicate.
func MainTurnCompleted(v orca.AgentState, workingAt int64, prompt string) bool {
	return v.AgentType == "codex" && v.MainAgent != nil && v.MainAgent.State == "done" && (v.MainAgent.Outcome == "" || v.MainAgent.Outcome == "success") && v.MainAgent.StateStartedAt > workingAt && v.Prompt == prompt && !v.ToolOutput && !v.Interrupted && !v.SessionBoundary && !v.RestoredUnconfirmed && v.LastAssistantMessage != ""
}
