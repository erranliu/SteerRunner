package orcatransport

import (
	"errors"
)

var errRemoteReadinessPending = errors.New("remote preparation receipt not yet available")

// Captured only while the original launch is unconfirmed and before the first
// business input. Later terminal output can never replace the saved receipt.
