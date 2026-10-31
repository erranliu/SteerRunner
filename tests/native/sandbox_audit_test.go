//go:build live

package native

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	terminal "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func auditWorkerSandbox(t *testing.T, m livetest.Manifest, run contracts.RunView) {
	t.Helper()
	if m.Execution.Adapter == "orca-terminal" && (run.Execution.Agent == "pi" || run.Execution.Agent == "dsh") {
		auditTerminalWorker(t, m, run)
		return
	}
	if m.Execution.Adapter != "codex-app-server" {
		if m.Execution.Target.Kind == "orca_environment" {
			var settings contracts.OrcaSettings
			if contracts.ValidateExecution(m.Execution) != nil || contracts.DecodeStrict(m.Execution.Settings, &settings) != nil || settings.AgentLaunch.RuntimeExecutable != "" || settings.AgentLaunch.RuntimeSHA256 != "" {
				t.Fatal("native remote launch contract differs")
			}
			raw, readErr := os.ReadFile(filepath.Join(*evidenceFlag, "native-calls.jsonl"))
			facts, auditErr := livetest.AnalyzeNativeAudit(raw, m.RemoteRuntimeID, livetest.ReceiptAuditPolicy(m))
			expectedCreations := 1
			if m.Driver == "development-multirun" {
				expectedCreations = 2
			}
			if readErr != nil || auditErr != nil || !facts.Complete || facts.Creations != expectedCreations || facts.Inspections != 0 {
				t.Fatal("native remote call audit failed", readErr, auditErr, facts)
			}
			save(t, "native-call-audit.json", facts)
			save(t, "native-launch-contract.json", map[string]any{"requested_model": settings.AgentLaunch.Model, "requested_reasoning_effort": settings.AgentLaunch.ReasoningEffort, "requested_access_mode": settings.AccessMode, "effective_parameters_verified": false, "helper_calls": 0})
		} else {
			auditLocalCodexWorker(t, m, run)
		}
		return
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

func auditTerminalWorker(t *testing.T, m livetest.Manifest, run contracts.RunView) {
	t.Helper()
	var settings contracts.OrcaSettings
	if json.Unmarshal(m.Execution.Settings, &settings) != nil {
		t.Fatal("frozen Pi settings missing")
	}
	directory := filepath.Join(m.Root, "s", "terminal-agents", run.RunID, run.Execution.AttemptID)
	read := func(name string, v any) {
		body, e := os.ReadFile(filepath.Join(directory, name+".json"))
		if e != nil || len(body) > 65536 || contracts.DecodeStrict(body, v) != nil {
			t.Fatal("bounded Pi native audit unavailable", name, e)
		}
	}
	var ready terminal.TerminalReceipt
	var owner terminal.TerminalOwner
	read("readiness", &ready)
	read("owner", &owner)
	p := ready.Preparation
	source := "pi-extension-v1"
	if settings.AgentName() == "dsh" {
		source = "dsh-plugin-v1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	metadataBytes, e := exec.CommandContext(ctx, settings.AgentLaunch.RuntimeExecutable, "--terminal-metadata").Output()
	var metadata terminal.TerminalMetadata
	if e != nil || contracts.DecodeStrict(metadataBytes, &metadata) != nil || metadata.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 {
		t.Fatal("frozen native helper metadata changed", e)
	}
	expectedSource := metadata.PiSHA256
	if settings.AgentName() == "dsh" {
		expectedSource = metadata.DSHSHA256
	}
	if ready.SourceSHA256 != expectedSource {
		t.Fatal("native extension differs from frozen helper")
	}
	if ready.Source != source || ready.Stage != "ready" || ready.RunID != run.RunID || ready.SessionID != owner.SessionID || ready.PID != owner.Job.Worker.PID || !p.Verified || p.Contract != terminaltools.Contract || p.AccessMode != settings.AccessMode || !p.ProviderEnvironmentAbsent || p.AccessMode != "full_access" || ready.Version != 4 || owner.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 || owner.ExtensionSHA256 != metadata.BundleSHA256 {
		t.Fatal("original Pi native permission/authentication audit failed")
	}
	job, e := process.AttachOwnedJob(owner.Job)
	if e != nil {
		t.Fatal("original Pi owned Job cannot be reattached", e)
	}
	job.Close()
	files, e := filepath.Glob(filepath.Join(directory, "*.json"))
	if e != nil {
		t.Fatal(e)
	}
	qualified := 0
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		if !b.StableExecutionID(name) {
			continue
		}
		var fact terminal.TerminalReceipt
		read(name, &fact)
		if fact.RunID != run.RunID || fact.SessionID != ready.SessionID || fact.InstanceID != ready.InstanceID || fact.PID != ready.PID || fact.CommandID != name || fact.Stage != "settled" || fact.Outcome != "completed" || fact.StopReason != "stop" || fact.NativeInputID == "" || fact.NativeReplyID == "" {
			t.Fatal("Pi business turn lacks original native settlement", name)
		}
		qualified++
	}
	if qualified < 2 {
		t.Fatal("Pi native audit lacks business implementation and publication turns")
	}
	save(t, settings.AgentName()+"-native-permission-audit.json", map[string]any{"readiness": ready, "owner": owner, "qualified_turns": qualified, "os_job_reattached": true})
}

func auditLocalCodexWorker(t *testing.T, m livetest.Manifest, run contracts.RunView) {
	t.Helper()
	var settings contracts.OrcaSettings
	if json.Unmarshal(m.Execution.Settings, &settings) != nil || settings.AgentLaunch.RuntimeExecutable == "" {
		t.Fatal("remote trusted runtime is required")
	}
	runtime, err := m.OrcaRuntimeID()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.TrimPrefix(run.Execution.SessionID, "orca-terminal:"+runtime+":"), ":")
	if !strings.HasPrefix(run.Execution.SessionID, "orca-terminal:"+runtime+":") || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatal("remote original terminal binding missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := orca.New(orca.Config{CommandTimeout: "30s", Output: func(ctx context.Context, _ time.Duration, args ...string) ([]byte, error) {
		all := append(append([]string(nil), settings.Transport.Args...), args...)
		if m.Execution.Target.Environment != "" {
			all = append(all, "--environment", m.Execution.Target.Environment)
		}
		return orca.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
			return exec.CommandContext(ctx, settings.Transport.Executable, all...).Output()
		})
	}})
	term, err := client.ShowTerminal(ctx, parts[0])
	if err != nil {
		t.Fatal(err)
	}
	original := b.ProcessIdentity{ExecutionHost: "local", RuntimeID: runtime, Terminal: parts[0], Incarnation: parts[1], WorktreeID: term.WorktreeID}
	raw, err := orca.QueryState(ctx, m.Execution.Target.Environment, runtime, "id:"+original.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := orca.DecodeOriginalState(raw, original)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := livetest.LocalPermissionAudit(ctx, client, original, settings.AgentLaunch.RuntimeExecutable, run.Execution.Workspace, state.ProviderSession.ID)
	save(t, "remote-worker-permission-audit.json", audit)
	if err != nil {
		t.Fatal(err)
	}
	mode := settings.AccessMode
	if mode == "" {
		mode = "sandbox"
	}
	if audit.Readiness.AccessMode != mode || audit.Readiness.Contract != codex.PolicyContract {
		t.Fatal("remote native access mode or policy changed")
	}
	if settings.AgentLaunch.RuntimeSHA256 == "" || audit.Readiness.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 {
		t.Fatal("remote runtime build is not frozen to the tested helper")
	}
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
