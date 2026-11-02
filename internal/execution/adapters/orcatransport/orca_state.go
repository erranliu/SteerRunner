package orcatransport

import (
	"context"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type stateFeedFactory func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error)

func orcaStateResource(p b.ProcessIdentity) native.StateResource {
	return native.StateResource{RuntimeID: p.RuntimeID, WorktreeID: p.WorktreeID, Handle: p.Terminal, IncarnationID: p.Incarnation}
}

func openOrcaStateFeed(ctx context.Context, environment string, p b.ProcessIdentity) (native.StateFeed, error) {
	return native.OpenStateFeed(ctx, environment, orcaStateResource(p))
}
