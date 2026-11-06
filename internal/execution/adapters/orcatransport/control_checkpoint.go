package orcatransport

import (
	"context"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
)

// This transport checkpoint cannot authorize business input or create resources.
type controlCheckpoint struct {
	Version  int                          `json:"version"`
	Binding  string                       `json:"binding"`
	Request  codex.RemoteControlRequest   `json:"request"`
	Response *codex.RemoteControlResponse `json:"response,omitempty"`
}

func (l *Lifecycle) inspectionGate(ctx context.Context) (func(), error) {
	l.inspectionOnce.Do(func() { l.inspectionSerial = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case l.inspectionSerial <- struct{}{}:
		return func() { <-l.inspectionSerial }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
