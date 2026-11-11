package orca

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOrcaResourceCreateCapability(t *testing.T) {
	for _, agent := range []string{"codex", "pi", ""} {
		t.Run("agent-"+agent, func(t *testing.T) {
			options := WorktreeCreate{Repository: "id:repo", Name: "分支 ' with space", BaseBranch: "fixed-head", Setup: "skip", NoParent: true, Agent: &agent}
			want := []string{"worktree", "create", "--repo", "id:repo", "--name", "分支 ' with space", "--base-branch", "fixed-head", "--no-parent", "--setup", "skip", "--json", "--agent", agent}
			if got := WorktreeCreateArgs(options); !reflect.DeepEqual(got, want) {
				t.Fatal("original worktree argv changed", got)
			}
			options.Agent = nil
			if got := WorktreeCreateArgs(options); !reflect.DeepEqual(got, want[:len(want)-2]) {
				t.Fatal("bare worktree acquired agent", got)
			}
		})
	}
	for _, scenario := range []string{"complete", "bare", "missing", "bad-worktree", "bad-agent", "bad-startup", "extension"} {
		t.Run("worktree-"+scenario, func(t *testing.T) {
			value := map[string]any{"worktree": Worktree{ID: "worktree", InstanceID: "instance", RepoID: "repo", Path: "C:/tree", Head: "head", Branch: "branch", BaseRef: "base", DisplayName: "name"}, "agentTerminalHandle": "agent", "startupTerminal": Terminal{Handle: "startup", IncarnationID: "incarnation"}}
			switch scenario {
			case "bare":
				delete(value, "agentTerminalHandle")
				delete(value, "startupTerminal")
			case "missing":
				value = map[string]any{}
			case "bad-worktree":
				value["worktree"] = "invalid"
			case "bad-agent":
				value["agentTerminalHandle"] = 42
			case "bad-startup":
				value["startupTerminal"] = true
			case "extension":
				value["resolved_runtime_id"] = []any{} // Execution checkpoint fields aren't native facts.
			}
			payload, _ := json.Marshal(value)
			result := Result{Payload: payload, RuntimeID: "runtime", CallUncertain: true, ErrorCode: "native-error"}
			decoded, err := DecodeWorktreeCreation(result)
			if (err != nil) != (scenario == "bad-worktree" || scenario == "bad-agent" || scenario == "bad-startup") {
				t.Fatal("worktree decoding changed", decoded, err)
			}
			if err == nil && scenario != "missing" && (decoded.Worktree.ID != "worktree" || decoded.Worktree.InstanceID != "instance" || scenario != "bare" && decoded.AgentTerminalHandle != "agent") {
				t.Fatal("native creation facts lost", decoded)
			}
			if result.RuntimeID != "runtime" || !result.CallUncertain || result.ErrorCode != "native-error" {
				t.Fatal("decode changed invocation facts")
			}
		})
	}
}
