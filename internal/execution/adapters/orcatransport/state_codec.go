package orcatransport

import (
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func readableStateVersion(version int) bool {
	return version == LocalStateVersion || version == StateVersion || version == NativeStateVersion
}

func decodeState(raw json.RawMessage, version int) (State, error) {
	var state State
	if !readableStateVersion(version) {
		return state, errors.New("unsupported Orca terminal state codec")
	}

	err := b.DecodeStrictJSON(raw, &state, "trailing terminal state")
	if err == nil {
		err = validateNativeProofFlags(state)
	}
	if err == nil && version == NativeStateVersion {
		err = validateNativeTerminalState(state)
	}
	return state, err
}

func validateNativeProofFlags(state State) error {
	for _, turn := range state.Turns {
		if turn.Native.Conflict != "" && turn.Native.Waiting != "" {
			return errors.New("native proof cannot be both conflicted and waiting")
		}
	}
	return nil
}

func validateNativeTerminalState(state State) error {

	if state.Control != nil || state.RemoteReadiness != nil || state.TerminalOwner != nil || state.TerminalReadiness != nil || state.AdmissionAttempted || state.AdmissionAccepted {
		return errors.New("native remote state contains helper preparation")
	}
	for _, turn := range state.Turns {
		if turn.Recorded != nil || turn.Extension != nil || turn.WireFormat != "" || turn.ReceiptRequest != "" || !turn.ReceiptQueriedAt.IsZero() {
			return errors.New("native remote state contains helper turn evidence")
		}
	}

	return nil
}
