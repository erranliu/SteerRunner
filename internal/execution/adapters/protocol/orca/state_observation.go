package orca

import "encoding/json"

// Orca's sequencer starts at incarnation 0 and increments on pane rebind.
// Its wire codec requires an explicit nonnegative JavaScript safe integer.
const maxObservationIncarnation = int64(1<<53 - 1)

func (o *StateObservation) UnmarshalJSON(data []byte) error {
	type fields StateObservation
	var value StateObservation
	wire := struct {
		*fields
		Incarnation *int64 `json:"incarnation"`
	}{fields: (*fields)(&value)}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	value.incarnationMissing = wire.Incarnation == nil
	if wire.Incarnation != nil {
		value.Incarnation = *wire.Incarnation
	}
	*o = value
	return nil
}

func (o StateObservation) MarshalJSON() ([]byte, error) {
	type fields StateObservation
	wire := struct {
		fields
		Incarnation *int64 `json:"incarnation,omitempty"`
	}{fields: fields(o)}
	if !o.incarnationMissing {
		wire.Incarnation = &o.Incarnation
	}
	return json.Marshal(wire)
}

// ValidHookTransition is common evidence validation, not turn attribution.
// Callers must still verify their original process, input, baseline and state.
func (o *StateObservation) ValidHookTransition() bool {
	return o != nil && !o.incarnationMissing && o.Incarnation >= 0 && o.Incarnation <= maxObservationIncarnation &&
		o.Origin == "hook" && o.Kind == "transition" && o.AuthorityID != "" && o.Revision > 0 && o.ObservedAt > 0
}
