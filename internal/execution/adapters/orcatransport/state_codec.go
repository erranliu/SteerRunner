package orcatransport

import (
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func readableStateVersion(version int) bool {
	return version == LocalStateVersion || version == StateVersion
}

func decodeState(raw json.RawMessage, version int) (State, error) {
	var state State
	if !readableStateVersion(version) {
		return state, errors.New("unsupported Orca terminal state codec")
	}

	err := b.DecodeStrictJSON(raw, &state, "trailing terminal state")
	if err == nil {
		for _, turn := range state.Turns {
			if turn.Native.Conflict != "" && turn.Native.Waiting != "" {
				return state, errors.New("native proof cannot be both conflicted and waiting")
			}
		}
	}
	return state, err
}
