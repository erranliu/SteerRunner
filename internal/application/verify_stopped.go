package application

import (
	"fmt"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// VerifyStopped reads current-format records under the host's exclusive lease.
// It probes original managed identities only, without constructing an Engine,
// invoking adapters, restarting observers or changing execution records.
func VerifyStopped(dir string, runs []*runstate.Record) contracts.StoppedVerification {
	result := contracts.StoppedVerification{StateDir: dir, ResourcesSettled: true, Runs: []contracts.StoppedRun{}}
	seen := map[string]bool{}
	for _, r := range runs {
		settled := stoppedRecord(r, func(id *b.ProcessIdentity) bool {
			return process.NewServiceFacts().Probe(id).State == "absent"
		})
		settled = settled && !seen[r.ID]
		seen[r.ID] = true
		v := contracts.StoppedRun{RunID: r.ID, InputDigest: r.InputDigest, ResourcesSettled: settled}
		if !settled {
			v.Diagnostic = fmt.Sprintf("run %s: original native writer absence is unverified", r.ID)
		}
		result.ResourcesSettled = result.ResourcesSettled && settled
		result.Runs = append(result.Runs, v)
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
