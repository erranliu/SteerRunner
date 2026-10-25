package orca

import (
	"context"
	"errors"
	"strings"
	"time"
)

type preparationChannelKey struct{}

func PreparationOwnsCLI(ctx context.Context) bool { return ctx.Value(preparationChannelKey{}) == true }

// Only the two approved reads separate queue and execution time. The process
// owner reaps the timed-out child before returning and permitting another read.
func (c Client) PreparationCall(ctx context.Context, serialized bool, checkpoint func() error, args ...string) (Result, error) {
	command := strings.Join(args, " ")
	if command != "repo list --json" && command != "status --json" {
		return Result{}, errors.New("unsupported preparation query")
	}
	var result Result
	invoke := func(ctx context.Context) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, &CLIQueueError{Cause: err}
		}
		if err := checkpoint(); err != nil {
			return nil, &CLIQueueError{Cause: err}
		}
		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := callCtx.Err(); err != nil {
			return nil, &CLIQueueError{Cause: err}
		}
		callCtx = context.WithValue(callCtx, preparationChannelKey{}, true)
		var err error
		result, err = c.MetaCallWithTimeout(callCtx, 15*time.Second, args...)
		return nil, err
	}
	var err error
	if serialized {
		_, err = SerializeCLI(ctx, invoke)
	} else {
		_, err = invoke(ctx)
	}
	return result, err
}
