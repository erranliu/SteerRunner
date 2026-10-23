package orcatransport

import (
	"encoding/json"
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const previousStateVersion = 6

func readableStateVersion(version int) bool {
	return version == StateVersion || version == previousStateVersion
}

func decodeState(raw json.RawMessage, version int) (State, error) {
	var state State
	if !readableStateVersion(version) {
		return state, errors.New("unsupported Orca terminal state codec")
	}
	if version == previousStateVersion {
		// v6 used one prose field for waits and permanent rejections. Only its
		// explicit waiting value is recoverable. Ambiguous historical provenance
		// failures retain their original block; missing evidence cannot clear them.
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return state, err
		}
		if object == nil {
			return state, errors.New("invalid v6 terminal state")
		}
		var turns []map[string]json.RawMessage
		if value, exists := object["turns"]; exists {
			if err := json.Unmarshal(value, &turns); err != nil {
				return state, err
			}
		}
		for _, turn := range turns {
			if len(turn["native"]) == 0 {
				continue
			}
			var proof map[string]json.RawMessage
			if err := json.Unmarshal(turn["native"], &proof); err != nil {
				return state, err
			}
			if proof["conflict"] != nil || proof["waiting"] != nil {
				return state, errors.New("v6 native proof contains v7 evidence fields")
			}
			if value, exists := proof["diagnostic"]; exists {
				var diagnostic string
				if err := json.Unmarshal(value, &diagnostic); err != nil {
					return state, err
				}
				delete(proof, "diagnostic")
				key := "conflict"
				if diagnostic == "Native main agent is waiting; no next input was sent" {
					key = "waiting"
				}
				proof[key] = value
				turn["native"], _ = json.Marshal(proof)
			}
		}
		if object["turns"] != nil {
			object["turns"], _ = json.Marshal(turns)
		}
		raw, _ = json.Marshal(object)
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
