package orca

import (
	"context"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Local IPC exposes authoritative snapshots, not notifications. The lifecycle
// owns query timing and completion proof; this feed never synthesizes events.
type localQueryFeed struct {
	process b.ProcessIdentity
	initial AgentState
	closed  context.Context
	cancel  context.CancelFunc
}

func openLocalQueryFeed(ctx context.Context, p b.ProcessIdentity) (StateFeed, error) {
	raw, err := QueryState(ctx, "", p.RuntimeID, "id:"+p.WorktreeID)
	if err != nil {
		return nil, err
	}
	initial, err := DecodeOriginalState(raw, p)
	if err != nil {
		return nil, err
	}
	closed, cancel := context.WithCancel(context.Background())
	return &localQueryFeed{process: p, initial: initial, closed: closed, cancel: cancel}, nil
}

func (f *localQueryFeed) Initial() AgentState          { return f.initial }
func (f *localQueryFeed) Drain() ([]AgentState, error) { return nil, f.closed.Err() }
func (f *localQueryFeed) Close()                       { f.cancel() }
func (f *localQueryFeed) Query(ctx context.Context) (AgentState, error) {
	if err := f.closed.Err(); err != nil {
		return AgentState{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(f.closed, cancel)
	defer stop()
	defer cancel()
	return QueryOriginalAgent(ctx, "", f.process, f.initial.PaneKey)
}
