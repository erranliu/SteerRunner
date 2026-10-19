package orca

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// StateSubscription reads public runtime state. It neither sends agent input nor
// interprets titles, screen text or worktree activity as completion.
type StateSubscription struct {
	socket stateConnection
	id     string
}

func SubscribeState(ctx context.Context, environment, runtime, worktree string) (*StateSubscription, json.RawMessage, error) {
	if runtime == "" || worktree == "" {
		return nil, nil, errors.New("missing original Orca state target")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return nil, nil, err
	}
	id, err := socket.request(ctx, "session.tabs.subscribe", map[string]string{"worktree": worktree})
	if err != nil {
		_ = socket.close()
		return nil, nil, err
	}
	sub := &StateSubscription{socket: socket, id: id}
	initial, err := sub.Next(ctx)
	if err != nil {
		sub.Close()
		return nil, nil, err
	}
	var snapshot struct{ Type string }
	if json.Unmarshal(initial, &snapshot) != nil || snapshot.Type != "snapshot" {
		sub.Close()
		return nil, nil, errors.New("Orca subscription did not supply an initial snapshot")
	}
	return sub, initial, nil
}

func (s *StateSubscription) Next(ctx context.Context) (json.RawMessage, error) {
	return s.socket.next(ctx, s.id)
}

func (s *StateSubscription) Close() { _ = s.socket.close() }

// A separate bounded connection leaves the streaming reader uninterrupted.
func QueryState(ctx context.Context, environment, runtime, worktree string) (json.RawMessage, error) {
	if runtime == "" || worktree == "" {
		return nil, errors.New("missing original Orca state target")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return nil, err
	}
	defer socket.close()
	return callState(ctx, socket, "session.tabs.list", map[string]string{"worktree": worktree})
}

// QueryWorktreeAgents reads the public hook-store projection. Callers must
// select and validate the exact original worktree/pane; the worktree's overall
// status is not a main-agent completion fact. worktree.ps has a row limit, not a
// cursor: expand to its reported total and accept only a complete snapshot.
func QueryWorktreeAgents(ctx context.Context, environment, runtime string) (json.RawMessage, error) {
	if runtime == "" {
		return nil, errors.New("missing original Orca runtime")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return nil, err
	}
	defer socket.close()
	limit := 100
	for attempt := 0; attempt < 3; attempt++ {
		raw, err := callState(ctx, socket, "worktree.ps", map[string]int{"limit": limit})
		if err != nil {
			return nil, err
		}
		var page struct {
			Worktrees  []json.RawMessage `json:"worktrees"`
			TotalCount *int              `json:"totalCount"`
			Truncated  *bool             `json:"truncated"`
		}
		if json.Unmarshal(raw, &page) != nil || page.TotalCount == nil || page.Truncated == nil || *page.TotalCount < len(page.Worktrees) {
			return nil, errors.New("Orca main-agent query completeness unavailable")
		}
		if !*page.Truncated && *page.TotalCount == len(page.Worktrees) {
			return raw, nil
		}
		if !*page.Truncated || *page.TotalCount <= limit {
			return nil, errors.New("Orca main-agent query remains incomplete at requested limit")
		}
		limit = *page.TotalCount
	}
	return nil, errors.New("Orca main-agent query completeness not established within query budget")
}
