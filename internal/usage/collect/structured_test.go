package collect

import (
	"encoding/json"
	"testing"

	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

func TestStructuredUsageSelectsUniquePublicDirectSource(t *testing.T) {
	original := &orca.StructuredUsageIdentity{SessionID: "oas_original", WorktreeID: "repo::C:/remote/worktree", RuntimeID: "original-runtime", HostID: "sha256:original"}
	binding := parsers.Binding{Agent: "codex", SessionID: "provider-session", CWD: "C:/remote/worktree"}
	for _, scenario := range []string{"original", "legacy", "missing-link", "foreign-native-session", "foreign-workspace", "foreign-host", "foreign-provider", "foreign-cwd", "foreign-agent", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			var candidate native.UsageSession
			if err := json.Unmarshal([]byte(`{"agent":"codex","sessionId":"provider-session","cwd":"C:/remote/worktree","executionHostId":"local","structuredSession":{"sessionId":"oas_original","workspaceId":"repo::C:/remote/worktree"},"filePath":"C:/account/sessions/log-provider-session.jsonl","codexHome":"C:/account"}`), &candidate); err != nil {
				t.Fatal(err)
			}
			identity := original
			switch scenario {
			case "legacy":
				identity = nil
				candidate.StructuredSession = nil
			case "missing-link":
				candidate.StructuredSession = nil
			case "foreign-native-session":
				candidate.StructuredSession.SessionID = "oas_foreign"
			case "foreign-workspace":
				candidate.StructuredSession.WorkspaceID = "repo::C:/other"
			case "foreign-host":
				candidate.ExecutionHostID = "another-server-host"
			case "foreign-provider":
				candidate.SessionID = "another-provider"
			case "foreign-cwd":
				candidate.CWD = "C:/other"
			case "foreign-agent":
				candidate.Agent = "pi"
			}
			sessions := []native.UsageSession{candidate}
			if scenario == "duplicate" {
				sessions = append(sessions, candidate)
			}
			selected, err := selectUsageSession(sessions, binding, identity)
			if scenario == "original" || scenario == "legacy" {
				if err != nil || selected.FilePath != candidate.FilePath {
					t.Fatal(selected, err)
				}
			} else if err == nil {
				t.Fatal("ambiguous or foreign source admitted", selected)
			}
		})
	}
}

func TestStructuredUsageBusinessEndIndependentOfStop(t *testing.T) {
	binding := parsers.Binding{AttemptID: "attempt", Commands: []parsers.Command{{ID: "command", NativeTurnID: "native-turn"}}}
	r := &runstate.Record{Status: "reconciling", StopStatus: "unknown", Snapshot: b.LifecycleSnapshot{Session: b.SessionContext{SessionID: "oas_original"}}}
	if structuredUsageEnded(r, binding) {
		t.Fatal("unknown stop finalized running usage")
	}
	r.Result = &b.TurnEvidence{CommandID: "command", SessionID: "oas_original", TurnID: "native-turn", Completed: true}
	if !structuredUsageEnded(r, binding) {
		t.Fatal("original completed turn waited for resource settlement")
	}
	r.Result.TurnID = "foreign-turn"
	if structuredUsageEnded(r, binding) {
		t.Fatal("foreign turn finalized coverage")
	}
	r.Result = nil
	r.Workflow.StageResults = []workflow.StageResult{{CommandID: "command", AttemptID: "attempt", SessionID: "oas_original", TurnID: "native-turn"}}
	if !structuredUsageEnded(r, binding) {
		t.Fatal("committed original result lost after cleanup")
	}
	r.Workflow.StageResults[0].AttemptID = "foreign-attempt"
	if structuredUsageEnded(r, binding) {
		t.Fatal("foreign attempt finalized coverage")
	}
}
