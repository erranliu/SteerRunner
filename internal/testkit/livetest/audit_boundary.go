package livetest

import (
	"bytes"
	"errors"
)

// NativeAuditAfterProbe preserves the complete append-only journal while
// separating an already stopped fault-probe Run from the subsequent Run.
func NativeAuditAfterProbe(raw, stoppedProbe []byte) ([]byte, error) {
	if len(stoppedProbe) == 0 || stoppedProbe[len(stoppedProbe)-1] != '\n' || len(raw) <= len(stoppedProbe) || !bytes.HasPrefix(raw, stoppedProbe) {
		return nil, errors.New("stopped probe journal prefix changed or main journal missing")
	}
	return raw[len(stoppedProbe):], nil
}
