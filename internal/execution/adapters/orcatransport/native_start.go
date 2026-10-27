package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func (l *Lifecycle) waitOriginalComposer(ctx context.Context, snapshot b.LifecycleSnapshot) error {
	if _, err := l.original(ctx, snapshot); err != nil {
		return err
	}
	budget, err := time.ParseDuration(l.profile.WaitTimeout)
	if err != nil {
		return err
	}
	result, err := l.client.MetaCallWithTimeout(ctx, budget+time.Second,
		"terminal", "wait", "--terminal", snapshot.Ref.External.Terminal,
		"--for", "tui-idle", "--timeout-ms", strconv.FormatInt(budget.Milliseconds(), 10), "--json")
	if err != nil {
		return err
	}
	var value struct {
		Wait struct {
			Satisfied bool `json:"satisfied"`
		} `json:"wait"`
	}
	if result.RuntimeID != snapshot.Ref.External.RuntimeID || result.CallUncertain || result.ErrorCode != "" ||
		json.Unmarshal(result.Payload, &value) != nil || !value.Wait.Satisfied {
		return errors.New("Original Codex composer is not verified idle; no task input was sent")
	}
	_, err = l.original(ctx, snapshot)
	return err
}

// A completed, independently queried original input proves that it started even
// when the bounded send receipt missed turn_started. Never replay a send, infer
// a start from READY/a screen/working alone, or authorize a streamed start here.
func (l *Lifecycle) queryCompletedOriginalStart(ctx context.Context, snapshot b.LifecycleSnapshot, state *State, turn *Turn, sample native.AgentState) (native.AgentState, error) {
	if turn.Started || !turn.Accepted || l.queryInput == nil || turn.Native.agentName() != "codex" ||
		turn.Native.BaseRootAt <= 0 || turn.Native.Conflict != "" || !sample.CanQueryCompletionInput() ||
		sample.MainAgent == nil || sample.MainAgent.State != "done" || sample.MainAgent.StateStartedAt <= turn.Native.BaseRootAt {
		return sample, nil
	}
	current, err := l.queryInput(ctx, state.Process, turn.Native.Pane, turn.Native.BaseRootAt, sample)
	if err != nil {
		// A failed post-read fence may still contain positive replacement
		// evidence. Preserve it just as the ordinary started-turn query does.
		if errors.Is(err, native.ErrStateIdentityChanged) {
			turn.Native.reject(err.Error())
		} else if current.MainAgent == nil || current.UpdatedAt >= current.MainAgent.StateStartedAt {
			turn.Native.rejectIdentity(current)
			turn.Native.rejectProvenance(current, turn)
		}
		if turn.Native.Conflict != "" {
			return current, errors.New(turn.Native.diagnostic())
		}
		return current, errors.New("Native original start query is unverified: " + err.Error())
	}
	if turn.Native.rejectIdentity(current) || turn.Native.rejectProvenance(current, turn) {
		return current, errors.New(turn.Native.diagnostic())
	}
	input := current.NativeInput
	if input == nil || !input.MatchesOriginalQuery(current) {
		return current, nil
	}
	if !input.MatchesOriginalProcess(state.Process) {
		turn.Native.reject("Native original start query belongs to another process")
		return current, errors.New(turn.Native.diagnostic())
	}
	candidate := *turn
	candidate.Started = true
	if !turn.Native.recordedInputMatches(current, &candidate) {
		return current, nil
	}
	if _, err := l.original(ctx, snapshot); err != nil {
		return current, err
	}
	turn.Started = true
	return current, nil
}
