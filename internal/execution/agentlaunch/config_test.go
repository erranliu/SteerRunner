package agentlaunch

import (
	"encoding/base64"
	"encoding/json"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"strings"
	"testing"
)

func TestAgentLaunchCompilesAndProtectsPreferences(t *testing.T) {
	for _, effort := range []string{"high", "xhigh"} {
		c := &Config{Executable: "codex", Model: "gpt-6-luna", ReasoningEffort: effort, Args: []string{"--no-alt-screen"}}
		cmd, err := c.Command("codex")
		if err != nil || !strings.Contains(cmd, "'gpt-6-luna'") || !strings.Contains(cmd, "model_reasoning_effort=\""+effort+"\"") || !strings.Contains(cmd, "tui.animations=false") {
			t.Fatal(cmd, err)
		}
		copy := c.Clone()
		copy.Args[0] = "changed"
		if c.Args[0] == "changed" {
			t.Fatal("launch arguments aliased")
		}
	}
	for _, config := range []*Config{
		{Executable: "codex", ReasoningEffort: "high"},
		{Executable: "codex", Args: []string{"--dangerously-bypass-approvals-and-sandbox"}},
		{Executable: "codex", Model: "chosen", ReasoningEffort: "invalid"},
		{Executable: "codex", Model: "chosen", Args: []string{"--model", "different"}},
		{Executable: "codex", Model: "chosen", Args: []string{"-c", "model_reasoning_effort=\"low\""}},
		{Executable: "codex\nother"},
	} {
		if _, err := config.Command("codex"); err == nil {
			t.Fatal("invalid launch accepted", config)
		}
	}
	c := &Config{Executable: "agent's path", Args: []string{"$secret; other"}}
	cmd, err := c.Command("opaque")
	if err != nil || !strings.Contains(cmd, "'agent''s path'") || !strings.Contains(cmd, "'$secret; other'") {
		t.Fatal("shell quoting changed values", cmd, err)
	}
}

func TestRemoteRuntimeCarriesOnlyFrozenPublicConfiguration(t *testing.T) {
	t.Setenv("REMOTE_APP_KEY", "caller-private-key-never-copied")
	for _, mode := range []string{"sandbox", "full_access"} {
		c := Config{RuntimeExecutable: "C:/runner runtime/steerrunner-agent.exe", RepositoryURL: "https://github.com/erranliu/SteerlineTest.git", Executable: "codex", Model: "gpt-6-luna", ReasoningEffort: "high", AccessMode: mode}
		c.AuditModel = &workflow.ModelSettings{Model: "gpt-6.1-sol", ReasoningEffort: "high"}
		c.GitHubAuth = &codex.AuthenticationConfig{AppID: 3, InstallationID: 7, PrivateKeyEnv: "REMOTE_APP_KEY"}
		command, err := c.Command("codex")
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(command, " '--configuration' '")
		if len(parts) != 2 {
			t.Fatal(command)
		}
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(parts[1], "'"))
		var got codex.RemoteAgentConfig
		if err != nil || json.Unmarshal(raw, &got) != nil || got.AccessMode != mode || got.Model != c.Model || got.RepositoryURL != c.RepositoryURL {
			t.Fatalf("lost remote settings: %s %v", raw, err)
		}
		if got.AuditModel == nil || *got.AuditModel != *c.AuditModel || got.GitHubAuth == nil || *got.GitHubAuth != *c.GitHubAuth || strings.Contains(string(raw), "caller-private-key-never-copied") {
			t.Fatal("remote model preferences or non-secret authentication references changed")
		}
		copy := c.Clone()
		copy.AuditModel.Model = "other"
		copy.GitHubAuth.PrivateKeyEnv = "OTHER_KEY"
		if c.AuditModel.Model == copy.AuditModel.Model || c.GitHubAuth.PrivateKeyEnv == copy.GitHubAuth.PrivateKeyEnv {
			t.Fatal("frozen remote preferences/reference aliases its clone")
		}
	}
}
