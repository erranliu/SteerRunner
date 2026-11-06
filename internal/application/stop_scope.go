package application

import (
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func requiresFrozenOrcaScope(r *runstate.Record, scope string) bool {
	return r.StopEvidenceScope != "" || orca.RequiresFrozenStopScope(r.Profile, scope)
}
