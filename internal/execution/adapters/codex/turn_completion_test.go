package codexworker

import "testing"

func TestBootstrapCompletionRequiresOriginalThreadAndTurn(t *testing.T) {
	for _, raw := range []string{
		`{}`, `broken`,
		`{"threadId":"other","turn":{"id":"turn","status":"completed"}}`,
		`{"threadId":"thread","turn":{"id":"other","status":"completed"}}`,
		`{"threadId":"thread","turn":{"id":"turn","status":"failed"}}`,
	} {
		if validateTurnCompletion([]byte(raw), "thread", "turn") == nil {
			t.Fatal("foreign or incomplete bootstrap accepted", raw)
		}
	}
	if err := validateTurnCompletion([]byte(`{"threadId":"thread","turn":{"id":"turn","status":"completed"}}`), "thread", "turn"); err != nil {
		t.Fatal(err)
	}
}
