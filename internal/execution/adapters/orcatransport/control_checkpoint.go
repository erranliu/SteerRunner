package orcatransport

import "context"

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
