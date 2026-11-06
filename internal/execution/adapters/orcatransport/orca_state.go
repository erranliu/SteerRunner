package orcatransport

import (
	"context"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type stateFeedFactory func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error)
