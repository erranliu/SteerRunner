package application

import (
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestSROfflineStoppedRequiresManagedOwnerAndRootAbsence(t *testing.T) {
	for _, tc := range []struct {
		name              string
		alter             func(*runstate.Record)
		owner, root, pass bool
	}{
		{"gone", func(*runstate.Record) {}, true, true, true},
		{"owner_alive", func(*runstate.Record) {}, false, true, false},
		{"root_alive", func(*runstate.Record) {}, true, false, false},
		{"unmanaged", func(r *runstate.Record) { r.NativeManaged = false }, true, true, false},
		{"unknown_owner", func(r *runstate.Record) { r.NativeOwner = nil }, true, true, false},
		{"unknown_root", func(r *runstate.Record) { r.Snapshot.Ref.External = b.ProcessIdentity{} }, true, true, false},
		{"orca", func(r *runstate.Record) { r.Request.Execution.Adapter = "orca-terminal" }, true, true, false},
		{"persisted_stopped", func(r *runstate.Record) { r.StopStatus = "stopped" }, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &runstate.Record{ID: "run", InputDigest: "input", NativeManaged: true, NativeOwner: &b.ProcessIdentity{Kind: "os", PID: 1, Created: "owner", Executable: "runner"}, Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "codex-app-server"}}}
			r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "os", PID: 2, Created: "root", Executable: "native"}
			tc.alter(r)
			got := stoppedRecord(r, func(id *b.ProcessIdentity) bool {
				if id.PID == 1 {
					return tc.owner
				}
				return tc.root
			})
			if got != tc.pass {
				t.Fatal("unverified original writer accepted", got)
			}
		})
	}
}
