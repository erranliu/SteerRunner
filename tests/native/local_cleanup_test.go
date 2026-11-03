//go:build windows && live

package native

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orcatransport "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"
)

func localStopEvidence(m livetest.Manifest, view contracts.RunView) (any, error) {
	if m.Execution.Target.Kind != "local" {
		return nil, errors.New("helper cleanup proof is local only")
	}
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
		if settings.AgentName() == "pi" {
			var fact orcatransport.TerminalExit
			raw, readErr := os.ReadFile(filepath.Join(m.Root, "s", "terminal-agents", view.RunID, view.Execution.AttemptID, "exit.json"))
			if readErr != nil || contracts.DecodeStrict(raw, &fact) != nil || state.TerminalOwner == nil || !reflect.DeepEqual(fact.Owner, *state.TerminalOwner) || fact.Owner.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 || fact.Version != 2 || !fact.WritersAbsent || fact.ClosedAt.IsZero() || process.Probe(&fact.Owner.Job.Owner).State != "absent" {
				return nil, errors.New("original terminal exit/authentication cleanup unverified")
			}
			evidence = fact
		} else {
			if record.StopEvidenceScope != orcatransport.LocalCodexStopScope || view.StopEvidenceScope != record.StopEvidenceScope || state.RemoteReadiness == nil || state.RemoteReadiness.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 {
				return nil, errors.New("original local Codex stop scope or helper binding changed")
			}
			repository := record.RepositoryPath
			if record.Request.Repository.LocalPath != "" {
				repository = record.Request.Repository.LocalPath
			}
			verifyCtx, cancelVerify := context.WithTimeout(context.Background(), 30*time.Second)
			verifyErr := orcatransport.VerifyStopped(verifyCtx, b.LifecycleSetup{Snapshot: record.Snapshot, ProfileData: record.Profile, ProfileID: record.ID, AcceptedConfigVersion: record.InputDigest, AgentLaunchCommand: record.LaunchCommand, ProcessHost: process.New(30 * time.Second), RepositoryPath: repository, RepositoryURL: record.Request.Repository.URL, RunDirectory: filepath.Join(m.Root, "s")})
			cancelVerify()
			if verifyErr != nil {
				return nil, verifyErr
			}
			evidence = map[string]any{"scope": record.StopEvidenceScope, "helper": state.RemoteReadiness.Owner, "app_server": state.RemoteReadiness.Process, "control": state.Control}

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
		return map[string]any{"run_id": view.RunID, "stop_evidence": evidence, "worktree": state.Process.WorktreeID, "runtime": inventory.Runtime, "remaining_terminals": 0, "complete_inventory": inventory.Verified, "inventory_transient_read_failures": inventory.TransientReadFailures, "inventory_observations": inventory.Observations, "inventory_checked_at": inventory.CheckedAt}, nil
	}
	return nil, errors.New("original Run missing from cleanup store")
}
