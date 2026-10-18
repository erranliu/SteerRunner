package orca

import (
	"context"
	"errors"
	"sync/atomic"
)

type dispatchCheckpointKey struct{}
type dispatchCheckpoint struct {
	used  atomic.Bool
	write func() error
}

// WithDispatchCheckpoint binds one durable intent to the actual command boundary,
// after CLI queue acquisition. Waiting in the queue has no external effect.
func WithDispatchCheckpoint(ctx context.Context, write func() error) context.Context {
	return context.WithValue(ctx, dispatchCheckpointKey{}, &dispatchCheckpoint{write: write})
}

// CheckpointDispatch must precede OutputCommand, including fixture transports.
// A failed write or repeated invocation never authorizes a command.
func CheckpointDispatch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	checkpoint, _ := ctx.Value(dispatchCheckpointKey{}).(*dispatchCheckpoint)
	if checkpoint == nil {
		return nil
	}
	if !checkpoint.used.CompareAndSwap(false, true) {
		return errors.New("command dispatch checkpoint already consumed")
	}
	return checkpoint.write()
}
