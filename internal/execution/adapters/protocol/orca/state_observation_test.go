package orca

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestObservationExplicitIncarnationSurvivesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, wire                 string
		valid, decodeError, absent bool
	}{
		{name: "zero", wire: "0", valid: true},
		{name: "positive", wire: "1", valid: true},
		{name: "max-safe", wire: "9007199254740991", valid: true},
		{name: "unsafe", wire: "9007199254740992"},
		{name: "negative", wire: "-1"},
		{name: "missing", absent: true},
		{name: "null", wire: "null", absent: true},
		{name: "fraction", wire: "0.5", decodeError: true},
		{name: "string", wire: `"0"`, decodeError: true},
		{name: "boolean", wire: "true", decodeError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := `{"origin":"hook","kind":"transition","authorityId":"authority","revision":2,"observedAt":20`
			if tc.wire != "" {
				wire += `,"incarnation":` + tc.wire
			}
			wire += "}"
			// Decode into a previously valid value as happens with reused receivers.
			o := StateObservation{Origin: "hook", Kind: "transition", AuthorityID: "old", Revision: 1, ObservedAt: 1}
			err := json.Unmarshal([]byte(wire), &o)
			if tc.decodeError {
				if err == nil {
					t.Fatal("invalid numeric wire accepted", wire)
				}
				return
			}
			if err != nil || o.ValidHookTransition() != tc.valid {
				t.Fatal("incorrect incarnation qualification", wire, o, err)
			}
			for range 2 {
				saved, err := json.Marshal(o)
				if err != nil || strings.Contains(string(saved), `"incarnation"`) == tc.absent {
					t.Fatal("round trip changed incarnation presence", string(saved), err)
				}
				var restored StateObservation
				if err = json.Unmarshal(saved, &restored); err != nil || restored != o || restored.ValidHookTransition() != tc.valid {
					t.Fatal("round trip changed evidence", restored, o, err)
				}
				o = restored
			}
		})
	}
}

func TestObservationMissingIncarnationDoesNotBecomeExplicitZero(t *testing.T) {
	explicit := StateObservation{Origin: "hook", Kind: "transition", AuthorityID: "authority", Revision: 2, ObservedAt: 20}
	var missing StateObservation
	if err := json.Unmarshal([]byte(`{"origin":"hook","kind":"transition","authorityId":"authority","revision":2,"observedAt":20}`), &missing); err != nil {
		t.Fatal(err)
	}
	if explicit == missing || !explicit.ValidHookTransition() || missing.ValidHookTransition() {
		t.Fatal("missing incarnation was conflated with initial zero")
	}
	var nilObservation *StateObservation
	if nilObservation.ValidHookTransition() {
		t.Fatal("absent observation qualified")
	}
}
