//go:build live

package native

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func auditWorkerSandbox(t *testing.T, m livetest.Manifest, run contracts.RunView) {
	t.Helper()
	if m.Execution.Adapter != "codex-app-server" {
		t.Fatal("remote same-sandbox native permission audit is unsupported")
	}
	var settings contracts.CodexSettings
	if json.Unmarshal(m.Execution.Settings, &settings) != nil {
		t.Fatal("missing frozen Codex settings")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader, err := codex.OpenHistory(ctx, b.Command{Executable: settings.Executable}, run.Execution.Workspace, "", process.New(30*time.Second).StartManaged)
	if err != nil {
		t.Fatal(err)
	}
	verifiedTurns := 0
	commandAudit := livetest.CommandAudit{}
	defer reader.Close()
	thread, err := reader.ReadThread(ctx, run.Execution.SessionID, run.Execution.Workspace)
	if err != nil || !filepath.IsAbs(thread.Path) {
		t.Fatal("native permission audit requires original full history path", err)
	}
	file, err := os.Open(thread.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	audit, err := livetest.AuditSandboxHistory(file, run.Execution.SessionID, run.Execution.Workspace)
	save(t, "worker-sandbox-audit.json", audit)
	if err != nil {
		t.Fatal(err)
	}
	// Notification journals independently expose approval requests even when
	// native history does not retain an approval event.
	root := filepath.Join(m.Root, "s", "runs", run.RunID, "turns")
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "appserver.jsonl" {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if containsApprovalRequest(body) {
				t.Error("native approval request in worker journal")
			}
			if m.Level == "L2" {
				audit, auditErr := livetest.AuditCommandJournal(bytes.NewReader(body))
				commandAudit.Completed += audit.Completed
				commandAudit.PermissionErrors += audit.PermissionErrors
				if auditErr != nil {
					t.Error(auditErr)
				}
			}
		}
		if d.Name() == "turn-start.json" {
			var fact struct {
				Verified  bool      `json:"verified"`
				CommandID string    `json:"command_id"`
				Workspace string    `json:"workspace"`
				Contract  string    `json:"contract"`
				ExpiresAt time.Time `json:"expires_at"`
				CheckedAt time.Time `json:"checked_at"`
			}
			body, err := os.ReadFile(filepath.Join(filepath.Dir(path), "sandbox-readiness.json"))
			if err != nil {
				return err
			}
			if json.Unmarshal(body, &fact) != nil || !fact.Verified || fact.Contract != codex.PolicyContract || fact.CommandID != filepath.Base(filepath.Dir(path)) || fact.Workspace != run.Execution.Workspace || !fact.ExpiresAt.After(fact.CheckedAt) {
				t.Error("business turn lacks bound sandbox readiness evidence")
			}
			verifiedTurns++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if verifiedTurns == 0 {
		t.Fatal("native tool audit found no qualified business turns")
	}
	if m.Level == "L2" {
		save(t, "worker-command-audit.json", commandAudit)
		if commandAudit.Completed == 0 {
			t.Fatal("L2 requires actual worker command results")
		}
	}
	save(t, "sandbox-readiness-audit.json", map[string]any{"qualified_turns": verifiedTurns, "session": run.Execution.SessionID, "contract": codex.PolicyContract})
}

func containsApprovalRequest(body []byte) bool {
	for _, line := range bytes.Split(body, []byte{'\n'}) {
		var e struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(line, &e) == nil && strings.Contains(strings.ToLower(e.Method), "approval") {
			return true
		}
	}
	return false
}
