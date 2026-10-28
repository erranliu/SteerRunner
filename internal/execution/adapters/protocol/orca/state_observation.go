package orca

import "encoding/json"

// Orca starts its per-authority incarnation counter at zero. A missing or null
// wire field is different: preserve that absence through copies and JSON round
// trips rather than manufacturing an initial incarnation from Go's zero value.
func (o *StateObservation) UnmarshalJSON(raw []byte) error {
	type plain StateObservation
	var value struct {
		plain
		Incarnation *int64 `json:"incarnation"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	*o = StateObservation(value.plain)
	o.incarnationMissing = value.Incarnation == nil
	if value.Incarnation != nil {
		o.Incarnation = *value.Incarnation
	}
	return nil
}

func (o StateObservation) MarshalJSON() ([]byte, error) {
	type plain StateObservation
	var incarnation *int64
	if !o.incarnationMissing {
		incarnation = &o.Incarnation
	}
	return json.Marshal(struct {
		plain
		Incarnation *int64 `json:"incarnation,omitempty"`
	}{plain(o), incarnation})
}

func (o StateObservation) HasValidIncarnation() bool {
	return !o.incarnationMissing && o.Incarnation >= 0
}
