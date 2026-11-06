//go:build windows && live

package native

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func localStopEvidence(m livetest.Manifest, view contracts.RunView) (any, error) {
	return structuredStopEvidence(m, view)
}
