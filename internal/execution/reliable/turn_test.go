package reliable

import (
	"errors"
	"testing"
)

func TestReliableTurnIdentityAndNotifications(t *testing.T) {
	state := TurnState{}
	next, err := BeginTurn(state, "session", "first", TurnAuthority{Allowed: true}, state.UpdatedAt)
	if err != nil || next.Number != 1 {
		t.Fatal(err)
	}
	if _, err := BeginTurn(next, "replacement", "other", TurnAuthority{Allowed: true}, state.UpdatedAt); err == nil {
		t.Fatal("session replaced")
	}
	stopped, err := StopTurn(next, "first", true, state.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BeginTurn(stopped, "session", "second", TurnAuthority{Allowed: true}, state.UpdatedAt)
	if err != nil || second.Number != 2 {
		t.Fatal(err)
	}
	if err := CurrentTurn(second, "first"); !errors.Is(err, ErrStale) {
		t.Fatal("late question changed current turn")
	}
	if _, err := StopTurn(second, "first", true, state.UpdatedAt); !errors.Is(err, ErrStale) {
		t.Fatal("late stop changed current turn")
	}
	duplicate, err := BeginTurn(second, "session", "second", TurnAuthority{}, state.UpdatedAt)
	if err != nil || duplicate != second {
		t.Fatal("duplicate start changed state")
	}
}
