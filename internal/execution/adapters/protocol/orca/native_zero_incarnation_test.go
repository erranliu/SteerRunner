package orca

import (
	"encoding/json"
	"testing"
)

func TestCorroboratedCompletionDistinguishesZeroIncarnationFromMissing(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		valid       bool
	}{
		{"zero", `,"incarnation":0`, true},
		{"positive", `,"incarnation":1`, true},
		{"missing", ``, false},
		{"null", `,"incarnation":null`, false},
		{"negative", `,"incarnation":-1`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var observation StateObservation
			raw := `{"origin":"hook","kind":"transition","authorityId":"original","revision":2,"observedAt":12000` + tc.field + `}`
			if err := json.Unmarshal([]byte(raw), &observation); err != nil {
				t.Fatal(err)
			}
			s := AgentState{State: "done", AgentType: "codex", PaneKey: "tab:pane", UpdatedAt: 12000, TurnStartedAt: 11000, MainAgent: &MainAgentState{State: "done", StateStartedAt: 12000}, MetadataObservation: &observation}
			s.ProviderSession.ID, s.ProviderSession.TranscriptPath = "original", "original.jsonl"
			if s.CanQueryCompletionInput() != tc.valid {
				t.Fatalf("explicit zero is a source epoch; missing/null/negative must remain unqualified: %s", raw)
			}
			encoded, err := json.Marshal(observation)
			if err != nil {
				t.Fatal(err)
			}
			var restored StateObservation
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			s.MetadataObservation = &restored
			if s.CanQueryCompletionInput() != tc.valid {
				t.Fatalf("round trip manufactured incarnation: %s", encoded)
			}
		})
	}
}
