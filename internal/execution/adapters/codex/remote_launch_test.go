package codexworker

import (
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"strings"
	"testing"
)

func TestRemoteTUIUsesServerOwnedPermissionsAndRoots(t *testing.T) {
	args := remoteTUIArgs(RemoteAgentConfig{Args: []string{"--no-daemon", "--no-alt-screen"}, Model: "server-model", ReasoningEffort: "high"}, "ws://127.0.0.1:1234", "original-session")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--remote ws://127.0.0.1:1234") || !strings.HasSuffix(joined, "resume original-session") {
		t.Fatal("TUI lost original server/session")
	}
	if strings.Count(joined, "--no-alt-screen") != 1 {
		t.Fatal("inline TUI must preserve the preparation receipt without duplicate flags")
	}
	for _, forbidden := range []string{"--no-daemon", "--add-dir", "sandbox_", "approval_", "shell_environment_policy", "server-model", "model_reasoning_effort"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("TUI tried to override server-owned setting %q", forbidden)
		}
	}
	audited := RemoteAgentConfig{AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}}
	args = remoteTUIArgs(audited, "ws://127.0.0.1:1234", "original-session")
	if !strings.Contains(strings.Join(args, " "), "--remote-auth-token-env "+remoteModelTokenEnv) {
		t.Fatal("native bridge authentication flag missing")
	}
}
