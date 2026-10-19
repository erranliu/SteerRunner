package application

import (
	"context"
	"fmt"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// VerifyStopped reads current-format records under the host's exclusive lease.
// It probes original identities without constructing an Engine, restarting
// observers, sending input or changing execution records.
func VerifyStopped(ctx context.Context, dir string, runs []*runstate.Record, checks ...*runstate.CheckRecord) contracts.StoppedVerification {
	result := contracts.StoppedVerification{StateDir: dir, ResourcesSettled: true, Runs: []contracts.StoppedRun{}}
	seen := map[string]bool{}
	for _, r := range runs {
		settled := stoppedRecord(r, func(id *b.ProcessIdentity) bool {
			return process.NewServiceFacts().Probe(id).State == "absent"
		})
		var probeErr error
		if !settled && r.ID != "" && r.InputDigest != "" && r.Request.Execution.Adapter == "orca-terminal" && r.Snapshot.Ref.RunID == r.ID && r.Snapshot.Session.ExecutionID == r.ID && r.Snapshot.ConfigurationDigest == r.InputDigest {
			repository := r.RepositoryPath
			if r.Request.Repository.LocalPath != "" {
				repository = r.Request.Repository.LocalPath
			}
			probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			probeErr = orca.VerifyStopped(probeCtx, b.LifecycleSetup{Snapshot: r.Snapshot.Clone(), ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest, AgentLaunchCommand: r.LaunchCommand, ProcessHost: process.New(30 * time.Second), RepositoryPath: repository, RepositoryURL: r.Request.Repository.URL, RunDirectory: dir})
			cancel()
			settled = probeErr == nil
		}
		settled = settled && !seen[r.ID]
		seen[r.ID] = true
		v := contracts.StoppedRun{RunID: r.ID, InputDigest: r.InputDigest, ResourcesSettled: settled}
		if !settled {
			v.Diagnostic = fmt.Sprintf("run %s: original native writer absence is unverified", r.ID)
			if probeErr != nil {
				v.Diagnostic += ": " + probeErr.Error()
			}
		}
		result.ResourcesSettled = result.ResourcesSettled && settled
		result.Runs = append(result.Runs, v)
	}
	result.Checks = []contracts.StoppedCheck{}
	for _, r := range checks {
		settled := r.View.CleanupStatus == "completed" && r.View.Resources.WriterStopped && !checkUnknown(r)
		result.Checks = append(result.Checks, contracts.StoppedCheck{CheckID: r.View.CheckID, InputDigest: r.View.InputDigest, ResourcesSettled: settled})
		result.ResourcesSettled = result.ResourcesSettled && settled
	}
	return result
}

func stoppedRecord(r *runstate.Record, absent func(*b.ProcessIdentity) bool) bool {
	if r.ID == "" || r.InputDigest == "" {
		return false
	}
	if r.StopStatus == "stopped" {
		return true
	}
	return managedRecordWritersAbsent(r, r.Snapshot.Ref.External, absent)
}

func managedRecordWritersAbsent(r *runstate.Record, root b.ProcessIdentity, absent func(*b.ProcessIdentity) bool) bool {
	// This is the same kill-on-close Job proof used by managed recovery:
	// both the sole owner and exact native root must have exited.
	return r.Request.Execution.Adapter == "codex-app-server" && r.NativeManaged &&
		r.NativeOwner != nil && r.NativeOwner.Kind == "os" && root.Kind == "os" &&
		validStoppedIdentity(r.NativeOwner) && validStoppedIdentity(&root) &&
		absent(r.NativeOwner) && absent(&root)
}

func validStoppedIdentity(id *b.ProcessIdentity) bool {
	return id != nil && id.PID > 0 && id.Created != "" && id.Executable != ""
}
