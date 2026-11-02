package orcatransport

import (
	"encoding/json"
	"reflect"
	"testing"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

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
