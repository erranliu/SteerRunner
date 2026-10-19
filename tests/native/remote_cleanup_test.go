//go:build windows && live

package native

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orcatransport "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"
)

func remoteStopEvidence(m livetest.Manifest, view contracts.RunView) (any, error) {
	if view.StopStatus != "stopped" || view.NewInputsAllowed {
		return nil, errors.New("original Run stop/revocation not confirmed")
	}
	runtime, err := m.OrcaRuntimeID()
	if err != nil {
		return nil, err
	}
	store, err := storage.OpenReader(filepath.Join(m.Root, "s"))
	if err != nil {
		return nil, err
	}
	defer store.Close()
	records, err := store.Runs()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.ID != view.RunID {
			continue
		}
		var state orcatransport.State
		found := false
		for _, e := range record.Snapshot.Implementation {
			if e.ImplementationKey == orcatransport.StateKey {
				if json.Unmarshal(e.Payload, &state) != nil {
					return nil, errors.New("remote cleanup state invalid")
				}
				found = true
			}
		}
		if !found || state.Process != record.Snapshot.Ref.External || orcatransport.SessionID(state.Process) != view.Execution.SessionID {
			return nil, errors.New("remote cleanup original resource binding missing")
		}
		var settings contracts.OrcaSettings
		if contracts.DecodeStrict(m.Execution.Settings, &settings) != nil {
			return nil, errors.New("frozen cleanup settings invalid")
		}
		var evidence any
		if settings.AgentName() == "pi" || settings.AgentName() == "dsh" {
			var fact orcatransport.TerminalExit
			raw, readErr := os.ReadFile(filepath.Join(m.Root, "s", "terminal-agents", view.RunID, view.Execution.AttemptID, "exit.json"))
			if readErr != nil || contracts.DecodeStrict(raw, &fact) != nil || state.TerminalOwner == nil || !reflect.DeepEqual(fact.Owner, *state.TerminalOwner) || fact.Owner.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 || fact.Version != 1 || !fact.WritersAbsent || fact.AuthenticationClosed == nil || !*fact.AuthenticationClosed || fact.ClosedAt.IsZero() || process.Probe(&fact.Owner.Job.Owner).State != "absent" {
				return nil, errors.New("original terminal exit/authentication cleanup unverified")
			}
			evidence = fact
		} else {
			proof, actor, proofErr := orcatransport.ReadAuthenticationStopEvidence(filepath.Join(m.Root, "s"), record.Snapshot, state)
			if proofErr != nil {
				return nil, proofErr
			}
			if proof.Readiness.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 {
				return nil, errors.New("remote cleanup helper was not frozen")
			}
			expectedMode := "host-gh-repository-broker"
			if settings.GitHubAuth != nil {
				expectedMode = "github-app-repository-broker"
			}
			if proof.Mode != expectedMode {
				return nil, errors.New("remote cleanup authentication source changed")
			}
			path, pathErr := codex.RemoteExitPath(proof.Readiness)
			if pathErr != nil {
				return nil, pathErr
			}
			exitCtx, cancelExit := context.WithTimeout(context.Background(), 30*time.Second)
			raw, readErr := orca.ReadStopArtifact(exitCtx, m.Execution.Target.Environment, state.Process.RuntimeID, state.Process.WorktreeID, path)
			cancelExit()
			if readErr != nil || codex.VerifyRemoteExit(raw, proof.Readiness) != nil {
				return nil, errors.Join(readErr, errors.New("original signed native exit unverified"))
			}
			evidence = map[string]any{"proof": proof, "control": actor, "signed_native_exit": json.RawMessage(raw)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
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
		if state.Process.RuntimeID != runtime {
			return nil, errors.New("remote cleanup runtime changed")
		}
		inventory, err := livetest.WaitRemoteWorkspaceEmpty(ctx, client, state.Process)
		if err != nil {
			return nil, err
		}
		return map[string]any{"run_id": view.RunID, "authentication": evidence, "worktree": state.Process.WorktreeID, "runtime": inventory.Runtime, "remaining_terminals": 0, "complete_inventory": inventory.Verified, "inventory_observations": inventory.Observations, "inventory_checked_at": inventory.CheckedAt}, nil
	}
	return nil, errors.New("original Run missing from cleanup store")
}
