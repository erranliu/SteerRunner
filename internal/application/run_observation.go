package application

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
)

func (e *Engine) Observe(id, after string, limit int) contracts.Response {
	r, page, err := e.store.Observe(e.ctx, id, after, limit)
	if err != nil {
		return contracts.Rejected("", fault(err))
	}
	// Match the existing legacy read-only projection; a new read API cannot
	// grant controls that the resident's original inspection withholds.
	if orca.LegacyRemoteProfile(r.Profile) {
		r.Allowed, r.ResumeRequired = false, true
		r.Diagnostic = "Legacy remote helper Run is read-only; only exact original terminal stop is supported."
	}
	return contracts.Success("", contracts.RunObservation{Contract: contracts.RunObservationContract, Run: r.View(), EventPage: page})
}
