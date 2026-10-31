package application

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
)

func (e *Engine) Observe(id, after string, limit int) contracts.Response {
	r, page, err := e.store.Observe(e.ctx, id, after, limit)
	if err != nil {
		return contracts.Rejected("", fault(err))
	}
	// Match the existing legacy read-only projection; a new read API cannot
	// grant controls that the resident's original inspection withholds.
	return contracts.Success("", contracts.RunObservation{Contract: contracts.RunObservationContract, Run: publicRunView(r), EventPage: page})
}
