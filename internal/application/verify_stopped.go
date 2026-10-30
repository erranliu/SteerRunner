package application

import (
	"context"
	"fmt"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// VerifyStopped reads current-format records under the host's exclusive lease.
// It probes original identities without constructing an Engine, restarting
// observers, sending input or changing execution records.
func VerifyStopped(ctx context.Context, dir string, runs []*runstate.Record) contracts.StoppedVerification {
	result := contracts.StoppedVerification{StateDir: dir, ResourcesSettled: true, Runs: []contracts.StoppedRun{}}
	seen := map[string]bool{}
	for _, r := range runs {
		repository := r.RepositoryPath
		if r.Request.Repository.LocalPath != "" {
			repository = r.Request.Repository.LocalPath
		}
		setup := b.LifecycleSetup{Snapshot: r.Snapshot.Clone(), ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest, AgentLaunchCommand: r.LaunchCommand, ProcessHost: process.New(30 * time.Second), RepositoryPath: repository, RepositoryURL: r.Request.Repository.URL, RunDirectory: dir}
		scope := ""
		var scopeErr error
		if r.Request.Execution.Adapter == "orca-terminal" {
			scope, scopeErr = orca.StopEvidenceScope(setup)
		}
		settled := stoppedRecord(r, func(id *b.ProcessIdentity) bool {
			return process.NewServiceFacts().Probe(id).State == "absent"
		})
		if scopeErr != nil || scope != "" && r.Snapshot.Ref.External.Kind != "" {
			settled = false // Verify the original inventory, not a saved status label.
		}
		var probeErr error
		if !settled && r.ID != "" && r.InputDigest != "" && r.Request.Execution.Adapter == "orca-terminal" && r.Snapshot.Ref.RunID == r.ID && r.Snapshot.Session.ExecutionID == r.ID && r.Snapshot.ConfigurationDigest == r.InputDigest {
			probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			probeErr = orca.VerifyStopped(probeCtx, setup)
			cancel()
			settled = probeErr == nil
		}
		settled = settled && !seen[r.ID]
		seen[r.ID] = true
		v := contracts.StoppedRun{RunID: r.ID, InputDigest: r.InputDigest, ResourcesSettled: settled, StopEvidenceScope: scope}
		if !settled {
			v.Diagnostic = fmt.Sprintf("run %s: original native writer absence is unverified", r.ID)
			if scope != "" {
				v.Diagnostic = fmt.Sprintf("run %s: original Orca terminal absence is unverified", r.ID)
			}
			if probeErr != nil {
				v.Diagnostic += ": " + probeErr.Error()
			}
		}
		result.ResourcesSettled = result.ResourcesSettled && settled
		result.Runs = append(result.Runs, v)
	}
	return result
}

func stoppedRecord(r *runstate.Record, absent func(*b.ProcessIdentity) bool) bool {
	for _, intent := range r.Intents {
		if parserUnsettled(intent) {
			if intent.Receipt == nil || !validStoppedIdentity(intent.Receipt.OriginalHost) || !validStoppedIdentity(intent.Receipt.Process) || !absent(intent.Receipt.OriginalHost) || !absent(intent.Receipt.Process) {
				return false
			}
		}
	}
	if r.ID == "" || r.InputDigest == "" {
		return false
	}
	if r.StopStatus == "stopped" {
		if orca.LegacyRemoteProfile(r.Profile) {
			for _, intent := range r.Intents {
				if intent.Request.Operation.Action == string(b.LifecycleStopExecution) && intent.Request.Snapshot.Ref.External == r.Snapshot.Ref.External && intent.Receipt != nil && intent.Receipt.Disposition == b.OperationConfirmed && intent.Receipt.State.Stopped && intent.Receipt.State.WorkerAbsent && intent.Receipt.ValidateFor(intent.Request) == nil {
					return true
				}
			}
			return false
		}
		return true
	}
	return managedRecordWritersAbsent(r, r.Snapshot.Ref.External, absent)
}

func managedRecordWritersAbsent(r *runstate.Record, root b.ProcessIdentity, absent func(*b.ProcessIdentity) bool) bool {
	if root == r.Snapshot.Ref.External && codexOwnerStopped(r) {
		return true
	}
	// This is the same kill-on-close Job proof used by managed recovery:
	// both the sole owner and exact native root must have exited.
	return r.Request.Execution.Adapter == "codex-app-server" && r.NativeManaged &&
		r.NativeOwner != nil && r.NativeOwner.Kind == "os" && root.Kind == "os" &&
		validStoppedIdentity(r.NativeOwner) && validStoppedIdentity(&root) &&
		absent(r.NativeOwner) && absent(&root)
}

func codexOwnerStopped(r *runstate.Record) bool {
	return r.Request.Execution.Adapter == "codex-app-server" && codex.VerifyExecutionOwnerStopped(r.Snapshot)
}

func validStoppedIdentity(id *b.ProcessIdentity) bool {
	return id != nil && id.PID > 0 && id.Created != "" && id.Executable != ""
}
