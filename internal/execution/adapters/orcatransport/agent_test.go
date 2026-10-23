package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestUnqualifiedAgentPreflightHasNoSideEffects(t *testing.T) {
	for _, agent := range []string{"pi", "dsh", "future-agent"} {
		t.Run(agent, func(t *testing.T) {
			l, host, snapshot := fixture(t)
			snapshot.Ref.Agent.Name = agent
			l.setup.Snapshot = snapshot
			report, err := l.PreflightLifecycle(context.Background(), l.setup)
			var failure *b.BackendError
			if !errors.As(err, &failure) || failure.Code != b.BackendUnsupported || !strings.Contains(failure.Diagnostic, "terminal_agent_evidence_unsupported") || len(report.Capabilities) != 0 {
				t.Fatal("missing native capability was admitted", report, err)
			}
			if _, err := l.PreparationFacts(context.Background(), snapshot); !errors.As(err, &failure) || failure.Code != b.BackendUnsupported {
				t.Fatal("preparation bypassed admission", err)
			}
			input := b.LifecycleInput{Workspace: &snapshot.WorkspacePlan}
			receipt, observer, err := l.Execute(context.Background(), request(snapshot, b.LifecyclePrepareWorkspace, input))
			if !errors.As(err, &failure) || failure.Code != b.BackendUnsupported || receipt.Disposition != b.OperationRejected || observer != nil {
				t.Fatal("direct execution bypassed admission", receipt, err)
			}
			if len(host.calls) != 0 || host.nativeSends != 0 {
				t.Fatal("rejected admission created or wrote a terminal")
			}
		})
	}
}

// Codex keeps its original mainAgent evidence contract. Managed agents use
// their formal extension channel and are tested in terminal_receipt_test.go.
func TestCodexNativeEvidenceIsolation(t *testing.T) {
	for _, agent := range []string{"codex"} {
		t.Run(agent, func(t *testing.T) {
			base := fixtureState(10, "previous", "done")
			start := fixtureState(11, "task", "working")
			end := fixtureState(12, "task", "done")
			base.AgentType, start.AgentType, end.AgentType = agent, agent, agent
			end.MainAgent.Outcome, end.LastAssistantMessage = "success", `{"summary":"本轮结果"}`
			now := time.Unix(100, 0).UTC()
			for _, kind := range []string{"success", "agent-changed", "missing-agent", "provider-changed", "missing-root", "unreported-outcome", "explicit-unknown", "failed", "cancelled", "session-start", "child-working", "tool-output", "restored", "truncated"} {
				t.Run(kind, func(t *testing.T) {
					turn := Turn{Accepted: true, Started: true, Prompt: "task", Native: nativeBaseline(base, now)}
					turn.Native.signal(start, &turn, now, false)
					current := end
					root := *end.MainAgent
					current.MainAgent = &root
					switch kind {
					case "agent-changed":
						current.AgentType = "foreign"
					case "missing-agent":
						current.AgentType = ""
					case "provider-changed":
						current.ProviderSession.ID = "foreign"
					case "missing-root":
						current.MainAgent = nil
					case "unreported-outcome":
						current.MainAgent.Outcome = ""
					case "explicit-unknown":
						current.MainAgent.Outcome = "unknown"
					case "cancelled":
						current.MainAgent.Outcome = "cancellation"
					case "failed":
						current.MainAgent.Outcome = "error"
					case "session-start":
						current.SessionBoundary = true
					case "child-working":
						current.MainAgent.State = "working"
					case "tool-output":
						current.ToolOutput = true
					case "restored":
						current.RestoredUnconfirmed = true
					case "truncated":
						current.LastAssistantMessage = strings.Repeat("x", 8000)
					}
					completed := turn.Native.query(current, &turn, now.Add(time.Second))
					want := kind == "success" || kind == "truncated" || kind == "unreported-outcome" && agent == "codex"
					if completed != want {
						t.Fatal("wrong root completion decision", completed, turn.Native)
					}
					if kind == "agent-changed" && (turn.Native.Diagnostic == "" || turn.Native.query(end, &turn, now.Add(2*time.Second))) {
						t.Fatal("a queried agent replacement was forgotten", turn.Native)
					}
					if kind == "missing-agent" && (turn.Native.Diagnostic != "" || !turn.Native.query(end, &turn, now.Add(2*time.Second))) {
						t.Fatal("temporarily missing metadata was treated as an agent replacement", turn.Native)
					}
					if kind == "success" && turn.Native.ReplyBody != end.LastAssistantMessage || kind == "truncated" && turn.Native.ReplyDigest != "" {
						t.Fatal("native result completeness lost", turn.Native)
					}
				})
			}
			proof := nativeBaseline(base, now)
			raw, err := json.Marshal(proof)
			if err != nil || (agent != "codex") != strings.Contains(string(raw), `"agent":`) {
				t.Fatal("legacy proof bytes/default changed", string(raw), err)
			}
			var restored NativeProof
			if json.Unmarshal(raw, &restored) != nil || restored.agentName() != agent {
				t.Fatal("native proof lost its frozen agent")
			}
			turn := Turn{Accepted: true, Started: true, Prompt: "task", Native: restored}
			missing := start
			missing.AgentType = ""
			turn.Native.signal(missing, &turn, now, false)
			if turn.Native.WorkingAt != 0 || turn.Native.Diagnostic != "" {
				t.Fatal("missing agent metadata started or permanently failed the turn")
			}
			foreign := start
			foreign.AgentType = "foreign"
			turn.Native.signal(foreign, &turn, now, false)
			if turn.Native.WorkingAt != 0 || turn.Native.Diagnostic == "" {
				t.Fatal("foreign agent hook started the original turn")
			}
		})
	}
}
