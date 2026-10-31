package livetest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
)

// Captured from the real remote smoke launch: both disabled file channels
// clear inherited values and restore the original shell environment.
func TestRemoteReceiptAuditIncludesDisabledFileChannels(t *testing.T) {
	m := Manifest{Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "paired"}, Settings: json.RawMessage(`{"agent":"codex","access_mode":"full_access","agent_launch":{"executable":"codex","model":"gpt-6.1-sol","reasoning_effort":"medium"}}`)}}
	const actual = "& { $srResultBefore=[Environment]::GetEnvironmentVariable('STEERLINE_RESULT_FILE'); try { [Environment]::SetEnvironmentVariable('STEERLINE_RESULT_FILE',$null); & { $srProgressBefore=[Environment]::GetEnvironmentVariable('STEERLINE_PROGRESS_FILE'); try { [Environment]::SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$null); & 'codex' '--config' 'sandbox_mode=danger-full-access' '--config' 'allow_login_shell=false' '--config' 'features.shell_snapshot=false' '--config' 'features.shell_snapshot_v2=false' '--config' 'approval_policy=never' '--config' 'approvals_reviewer=user' '--model' 'gpt-6.1-sol' '-c' 'model_reasoning_effort=\"medium\"' '-c' 'tui.animations=false' 'Initialize: READY only, no tools/changes. This rule ends after READY; later/compacted turns complete the current task.' } finally { [Environment]::SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$srProgressBefore) } } } finally { [Environment]::SetEnvironmentVariable('STEERLINE_RESULT_FILE',$srResultBefore) } }"
	policy := ReceiptAuditPolicy(m)
	journal := func(launch string) []byte {
		var raw []byte
		for _, c := range []struct {
			id   string
			args []string
		}{{"workspace", []string{"worktree", "create"}}, {"launch", []string{"terminal", "send", "--text", launch}}, {"implement", []string{"terminal", "send", "--text", "implement"}}} {
			for _, kind := range []string{"intent", "receipt"} {
				row := nativeAuditCall{Kind: kind, ID: c.id, Args: c.args}
				if kind == "receipt" {
					row.Response = json.RawMessage(`{"ok":true,"_meta":{"runtimeId":"runtime"}}`)
				}
				line, _ := json.Marshal(row)
				raw = append(append(raw, line...), '\n')
			}
		}
		return raw
	}
	if facts, err := AnalyzeNativeAudit(journal(actual), "runtime", policy); err != nil || !facts.Complete || facts.Creations != 1 || len(facts.Prompts) != 2 {
		t.Fatalf("real frozen launch rejected: %+v %v", facts, err)
	}
	bare, err := (&agentlaunch.Config{Executable: "codex", AccessMode: "full_access", Model: "gpt-6.1-sol", ReasoningEffort: "medium", RepositoryURL: RemoteURL}).Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	for name, changed := range map[string]string{
		"missing-both-wrappers":      bare,
		"missing-result-wrapper":     agentlaunch.ProgressCommand(bare, ""),
		"unexpected-enabled-channel": agentlaunch.FileCommands(bare, strings.Repeat("a", 32), ""),
		"changed-restoration":        strings.Replace(actual, "$srResultBefore", "$other", 1),
		"changed-model":              strings.Replace(actual, "gpt-6.1-sol", "other-model", 1),
		"changed-permission":         strings.Replace(actual, "sandbox_mode=danger-full-access", "sandbox_mode=read-only", 1),
		"extra-command":              actual + "; other",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := AnalyzeNativeAudit(journal(changed), "runtime", policy); err == nil {
				t.Fatal("changed startup accepted")
			}
		})
	}
}
