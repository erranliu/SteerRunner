package orca

import (
	"context"
	"errors"
	"testing"
)

func TestDispatchCheckpointRequiresSuccessfulSingleDurableWrite(t *testing.T) {
	writes := 0
	failed := errors.New("checkpoint write failed")
	ctx := WithDispatchCheckpoint(context.Background(), func() error { writes++; return failed })
	if err := CheckpointDispatch(ctx); !errors.Is(err, failed) || writes != 1 {
		t.Fatal(err, writes)
	}
	if err := CheckpointDispatch(ctx); err == nil || writes != 1 {
		t.Fatal("reused dispatch authorization", err, writes)
	}
	canceled, cancel := context.WithCancel(WithDispatchCheckpoint(context.Background(), func() error { writes++; return nil }))
	cancel()
	if err := CheckpointDispatch(canceled); !errors.Is(err, context.Canceled) || writes != 1 {
		t.Fatal("canceled dispatch wrote intent", err, writes)
	}
}
