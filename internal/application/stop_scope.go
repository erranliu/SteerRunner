package application

import (
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func requiresFrozenOrcaScope(r *runstate.Record, scope string) bool {
	if scope == orca.LocalCodexStopScope || r.StopEvidenceScope != "" {
		return true
	}
	for _, profile := range r.Profile {
		if profile.ImplementationKey == orca.ProfileKey && profile.CodecVersion == orca.NativeProfileVersion {
			return true
		}
	}
	return false
}
