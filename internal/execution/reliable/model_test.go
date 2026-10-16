package reliable

import (
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"testing"
)

func TestSRTurnModelBinding(t *testing.T) {
	old := []byte(`{"CommandID":"command","Prompt":"fixed","Purpose":"business"}`)
	var input b.TurnInput
	if err := json.Unmarshal(old, &input); err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(input)
	if string(wire) != string(old) {
		t.Fatalf("old intent bytes changed: %s", wire)
	}
	input.Model, input.ReasoningEffort = "reviewer", "high"
	same := input
	if !SameInput(&same, input) {
		t.Fatal("same model rejected")
	}
	same.Model = "executor"
	if SameInput(&same, input) {
		t.Fatal("model changed without new intent")
	}
	same = input
	same.ReasoningEffort = "low"
	if SameInput(&same, input) {
		t.Fatal("effort changed without new intent")
	}
}
