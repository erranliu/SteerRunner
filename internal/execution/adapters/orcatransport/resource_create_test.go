package orcatransport

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestWorkspaceCreationFacts(t *testing.T) {
	checkedAt := time.Date(2026, 11, 5, 12, 34, 56, 789, time.FixedZone("fixture", 3600))
	originalTarget := b.ExecutionTarget{ID: "target", ConnectionID: "connection", ExternalRuntime: "original-runtime", ExecutionHost: "original-host", WorktreeID: "before", Branch: "before", BaseRevision: "before"}
	for _, tc := range []struct{ branch, want string }{
		{"refs/heads/topic", "topic"}, {"topic", "topic"}, {"refs/heads/refs/heads/topic", "refs/heads/topic"}, {"REFS/heads/topic", "REFS/heads/topic"},
	} {
		t.Run(tc.branch, func(t *testing.T) {
			worktree := native.Worktree{ID: "original-worktree", InstanceID: "original-instance", RepoID: "native-repo", Path: `D:\original workspace`, Branch: tc.branch, Head: "actual-head", BaseRef: "native-base"}
			before := worktree
			workspace, target := workspaceCreationFacts("original-project", originalTarget, worktree, "requested-base", checkedAt)
			wantWorkspace := b.WorkspaceInspection{InspectionScope: "revision", Repository: "original-project", Worktree: `D:\original workspace`, Branch: tc.want, Base: "requested-base", Head: "actual-head", CheckedAt: checkedAt}
			wantTarget := b.ExecutionTarget{ID: "target", ConnectionID: "connection", ExternalRuntime: "original-runtime", ExecutionHost: "original-host", WorktreeID: "original-worktree", Branch: tc.want, BaseRevision: "requested-base"}
			if workspace != wantWorkspace || target != wantTarget || worktree != before || originalTarget.WorktreeID != "before" || originalTarget.Branch != "before" || originalTarget.BaseRevision != "before" {
				t.Fatal("creation projection changed fields, zero values, time or inputs", workspace, target)
			}
		})
	}
}

func TestWorkspaceCreationCodecPreservesCheckpoint(t *testing.T) {
	// Frozen pre-extraction checkpoint layout; don't build the oracle with the
	// new decoder or change its persisted field order through embedding.
	type originalCheckpoint struct {
		RuntimeID            string          `json:"runtime_id"`
		ResolvedRepositoryID string          `json:"resolved_repository_id,omitempty"`
		ResolvedRuntimeID    string          `json:"resolved_runtime_id,omitempty"`
		Worktree             native.Worktree `json:"worktree"`
		AgentTerminalHandle  string          `json:"agentTerminalHandle,omitempty"`
		StartupTerminal      native.Terminal `json:"startupTerminal"`
		Terminal             native.Terminal `json:"selected_terminal"`
	}
	for _, scenario := range []string{"complete", "bare", "missing", "bad-native", "bad-runtime", "bad-resolved", "bad-selected", "extension"} {
		t.Run(scenario, func(t *testing.T) {
			value := map[string]any{"runtime_id": "saved", "resolved_runtime_id": "resolved-runtime", "resolved_repository_id": "repo", "worktree": map[string]any{"id": "worktree", "instanceId": "instance"}, "agentTerminalHandle": "agent", "startupTerminal": map[string]any{"handle": "startup"}, "selected_terminal": map[string]any{"handle": "selected", "incarnationId": "incarnation"}}
			switch scenario {
			case "bare":
				delete(value, "agentTerminalHandle")
				delete(value, "startupTerminal")
			case "missing":
				value = map[string]any{}
			case "bad-native":
				value["startupTerminal"] = true
			case "bad-runtime":
				value["runtime_id"] = 42
			case "bad-resolved":
				value["resolved_runtime_id"] = []any{}
			case "bad-selected":
				value["selected_terminal"] = "invalid"
			case "extension":
				value["unused"] = []any{}
			}
			payload, _ := json.Marshal(value)
			var old originalCheckpoint
			oldErr := json.Unmarshal(payload, &old)
			got, err := decodeWorkspaceCreation(native.Result{Payload: payload, RuntimeID: "envelope"})
			if (err != nil) != (oldErr != nil) {
				t.Fatal("checkpoint field acceptance changed", err, oldErr)
			}
			if err == nil {
				oldBytes, _ := json.Marshal(old)
				gotBytes, _ := json.Marshal(got)
				if !reflect.DeepEqual(gotBytes, oldBytes) || digest(gotBytes) != digest(oldBytes) {
					t.Fatal("persisted checkpoint bytes changed", string(gotBytes), string(oldBytes))
				}
			}
		})
	}
}

func TestWorkspaceCreationBindsCLIResult(t *testing.T) {
	// Keep a frozen payload that distinguishes response metadata from legacy
	// checkpoint extensions. Binding must not substitute the discovered runtime
	// for the response runtime, even when they disagree.
	result := native.Result{RuntimeID: "response-runtime", Payload: json.RawMessage(`{
		"runtime_id":"payload-runtime",
		"resolved_runtime_id":"payload-resolved-runtime",
		"resolved_repository_id":"payload-repo",
		"worktree":{"id":"original-worktree","repoId":"response-repo"},
		"selected_terminal":{"handle":"original-terminal"}
	}`)}
	for _, tc := range []struct {
		name, selector, runtime, wantRuntime, wantRepo string
	}{
		{"local", "path:C:/project", "discovered", "discovered", "payload-repo"},
		{"paired", "id:original-repo", "discovered", "discovered", "original-repo"},
		{"unresolved", "id:original-repo", "", "payload-resolved-runtime", "payload-repo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bindWorkspaceCreation(result, tc.selector, tc.runtime)
			if err != nil || got.RuntimeID != "response-runtime" || got.ResolvedRuntimeID != tc.wantRuntime ||
				got.ResolvedRepositoryID != tc.wantRepo || got.Worktree.ID != "original-worktree" ||
				got.Worktree.RepoID != "response-repo" || got.Terminal.Handle != "original-terminal" {
				t.Fatalf("CLI result binding changed: %+v, %v", got, err)
			}
		})
	}
}
