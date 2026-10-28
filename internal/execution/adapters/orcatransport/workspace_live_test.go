package orcatransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// A small opt-in probe of the changed startup path, not a delivery/PR scenario.
func TestOrcaWorkspaceStartupLive(t *testing.T) {
	root := os.Getenv("STEERRUNNER_ORCA_WORKSPACE_PROBE_ROOT")
	if root == "" {
		t.Skip("requires a generated Runner fixture")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	var fixture struct {
		Execution      contracts.Execution `json:"execution"`
		RepositoryPath string              `json:"repository_path"`
	}
	if err != nil || json.Unmarshal(raw, &fixture) != nil || contracts.ValidateExecution(fixture.Execution) != nil || fixture.Execution.Adapter != "orca-terminal" || !filepath.IsAbs(fixture.RepositoryPath) {
		t.Fatal("requires an independent Runner Orca fixture")
	}
	var settings contracts.OrcaSettings
	if err := contracts.DecodeStrict(fixture.Execution.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	p := Profile{Executable: settings.Transport.Executable, Args: settings.Transport.Args, WaitTimeout: settings.Transport.WaitTimeout, Environment: fixture.Execution.Target.Environment}
	profileID := "native-startup-probe"
	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	launch, err := (&agentlaunch.Config{Executable: settings.AgentLaunch.Executable, Args: settings.AgentLaunch.Args, Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort}).Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	repository := fixture.RepositoryPath
	base, err := exec.Command("git", "-C", repository, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	host := liveProbeHost{}
	client := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if p.Environment != "" {
			args = append(args, "--environment", p.Environment)
		}
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: p.Executable, Args: args, WorkingDirectory: repository, Timeout: timeout})
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if _, err := client.MetaCall(ctx, "repo", "add", "--path", repository, "--json"); err != nil {
		t.Fatal(err)
	}
	for _, recovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "saved-receipt-recovery"}[recovery], func(t *testing.T) {
			id := make([]byte, 16)
			if _, err := rand.Read(id); err != nil {
				t.Fatal(err)
			}
			runID := hex.EncodeToString(id)
			s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: p.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: runID, AttemptID: runID}, Session: b.SessionContext{ProjectID: repository, CorrelationID: "startup-probe", ExecutionID: runID, AttemptID: runID, WorkflowRunID: runID}, ConfigurationDigest: "startup-probe", ResourceGeneration: runID, WorkspacePlan: b.WorkspaceIntent{BaseCommit: strings.TrimSpace(string(base))}}
			data, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: ProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: repository, ProfileID: profileID, AcceptedConfigVersion: "startup-probe"}, payload)
			if err != nil {
				t.Fatal(err)
			}
			setup := b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{data}, ProfileID: profileID, AcceptedConfigVersion: "startup-probe", ProcessHost: host, RepositoryPath: repository, RunDirectory: root, AgentLaunchCommand: launch}
			port, err := NewLifecycle(setup)
			if err != nil {
				t.Fatal(err)
			}
			l := port.(*Lifecycle)
			defer l.closeStateFeed()
			injected := false
			calls := workspaceCalls(l, func(args []string) ([]byte, bool) {
				if recovery && !injected && args[0] == "terminal" && args[1] == "show" {
					injected = true
					return workspaceQueryError("probe_query_unavailable"), true
				}
				return nil, false
			})
			req := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
			req.Operation.ID += "-" + runID
			receipt, _, startErr := l.Execute(ctx, req)
			var creation workspaceCreation
			if json.Unmarshal(receipt.Operation.Receipt, &creation) != nil {
				t.Fatal("creation response was not retained", startErr)
			}
			record := map[string]any{"recovery": recovery, "worktree": creation.Worktree, "runtime": creation.RuntimeID, "passed": false, "cleaned_up": false}
			defer func() {
				cleanup, cleanupCancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cleanupCancel()
				handle := creation.Terminal.Handle
				if handle != "" {
					proof, closeErr := client.CloseTerminalChecked(cleanup, handle, creation.RuntimeID)
					if closeErr != nil || proof.Disposition != "confirmed" {
						t.Error("terminal cleanup unverified", closeErr)
						return
					}
				}
				_, removeErr := client.MetaCall(cleanup, "worktree", "rm", "--worktree", "id:"+creation.Worktree.ID, "--force", "--json")
				record["cleaned_up"] = removeErr == nil
				if removeErr != nil {
					t.Error("worktree cleanup unverified", removeErr)
				}
				record["calls"] = *calls
				raw, _ := json.MarshalIndent(record, "", "  ")
				if err := os.WriteFile(filepath.Join(root, "evidence", "orca-startup-"+t.Name()[strings.LastIndex(t.Name(), "/")+1:]+".json"), raw, 0600); err != nil {
					t.Error(err)
				}
			}()
			if recovery {
				if startErr == nil || !injected {
					t.Fatal("query fault was not exercised")
				}
				checkpoint := filepath.Join(root, "evidence", "startup-creation-receipt.json")
				wire, _ := json.Marshal(receipt)
				if err := os.WriteFile(checkpoint, wire, 0600); err != nil {
					t.Fatal(err)
				}
				req.Operation.Receipt, err = os.ReadFile(checkpoint)
				if err != nil {
					t.Fatal(err)
				}
				port, err = NewLifecycle(setup) // Restore only the persisted operation receipt.
				if err != nil {
					t.Fatal(err)
				}
				l = port.(*Lifecycle)
				defer l.closeStateFeed()
				restoredCalls := workspaceCalls(l, nil)
				receipt, _, startErr = l.Reconcile(ctx, req)
				assertWorkspaceReadonly(t, *restoredCalls)
			}
			if startErr != nil || receipt.Process == nil || receipt.Disposition != b.OperationConfirmed {
				t.Fatal("native workspace binding failed", startErr)
			}
			bind(&s, receipt)
			record["workspace_binding_verified"] = true
			// Trust only this probe checkout through session argv, never the user's
			// global config. Resource preparation has not sent an agent command yet.
			setup.Snapshot = s
			setup.AgentLaunchCommand = launch
			for _, path := range []string{filepath.FromSlash(creation.Worktree.Path), repository} {
				trustedPath, _ := json.Marshal(path)
				trust := "projects." + string(trustedPath) + ".trust_level=\"trusted\""
				setup.AgentLaunchCommand += " '-c' '" + strings.ReplaceAll(trust, "'", "''") + "'"
			}
			port, err = NewLifecycle(setup)
			if err != nil {
				t.Fatal(err)
			}
			l = port.(*Lifecycle)
			defer l.closeStateFeed()
			launchCalls := workspaceCalls(l, nil)
			candidate, err := reliable.Next(ctx, l, s)
			if err != nil || candidate.Action != b.LifecycleCreateResource {
				t.Fatal(candidate, err)
			}
			launchReq := request(s, candidate.Action, candidate.Input)
			launchReq.Operation.ID += "-" + runID
			launched, _, err := l.Execute(ctx, launchReq)
			if err != nil {
				t.Fatal("native startup was not accepted", err)
			}
			bind(&s, launched)
			record["agent_start_commands"] = 1
			startupCtx, startupCancel := context.WithTimeout(ctx, 45*time.Second)
			defer startupCancel()
			for launched.Disposition != b.OperationConfirmed {
				if err := waitWorkspaceTerminal(startupCtx); err != nil {
					t.Fatal(err)
				}
				launchReq.Snapshot = s
				launched, _, err = l.Reconcile(startupCtx, launchReq)
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, launched)
			}
			if _, err := l.ready(startupCtx, s); err != nil {
				t.Fatal("native main-agent readiness failed", err)
			}
			creates := 0
			for _, call := range *calls {
				if call[0] == "worktree" && call[1] == "create" {
					creates++
				}
			}
			if creates != 1 {
				t.Fatal("native workspace was recreated")
			}
			record["process"], record["passed"] = receipt.Process, true
			sends := 0
			for _, call := range *launchCalls {
				if call[0] == "terminal" && call[1] == "send" {
					sends++
				}
			}
			if sends != 1 {
				t.Fatal("agent startup was repeated", sends)
			}
			record["agent_start_commands"] = sends
			if recovery && receipt.Process.Incarnation != creation.Terminal.IncarnationID {
				t.Fatal(errors.New("recovery replaced the original PTY"))
			}
		})
	}
}
