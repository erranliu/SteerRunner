package codexworker

import (
	"encoding/json"
	"errors"
)

// validateTurnCompletion binds the ordinary remote bootstrap to its original
// native thread and turn. It does not interpret business reports.
func validateTurnCompletion(raw json.RawMessage, threadID, turnID string) error {
	var p struct {
		ThreadID string                      `json:"threadId"`
		Turn     struct{ ID, Status string } `json:"turn"`
	}
	if json.Unmarshal(raw, &p) != nil || p.ThreadID != threadID || p.Turn.ID != turnID || p.Turn.Status != "completed" {
		return errors.New("original_native_turn_unsettled")
	}
	return nil
}
