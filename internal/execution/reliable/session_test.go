package reliable

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestSessionReceiptSeparatesAcceptedInputFromTurnStart(t *testing.T) {
	s := b.LifecycleSnapshot{Session: b.SessionContext{SessionID: "session", PendingCommandID: "command"}, PendingTurn: &b.TurnInput{CommandID: "command", Prompt: "fixed input"}}
	before, _ := json.Marshal(s)
	receipt := b.LifecycleReceipt{State: b.LifecycleStateFacts{SessionID: "session", TurnID: "turn", InputAccepted: true}}
	accepted, started, err := ApplySessionReceipt(s, nil, receipt, false)
	if err != nil || started || accepted.Session.CurrentCommandID != "" || accepted.Session.PendingCommandID != "command" || accepted.Session.PendingTurnID != "turn" || accepted.PendingTurn != nil {
		t.Fatal("accepted input was mistaken for an already started turn", accepted, started, err)
	}
	fromEvent := RecordTurnStart(accepted.Session, "turn")
	receipt.State.TurnStarted = true
	fromReceipt, started, err := ApplySessionReceipt(s, nil, receipt, false)
	if err != nil || !started || !reflect.DeepEqual(fromEvent, fromReceipt.Session) || fromReceipt.Session.CurrentCommandID != "command" || fromReceipt.Session.CurrentTurnID != "turn" {
		t.Fatal("event and receipt bound different original commands", fromEvent, fromReceipt, err)
	}
	after, _ := json.Marshal(s)
	if !bytes.Equal(after, before) {
		t.Fatal("receipt application modified the caller's original snapshot")
	}
}

func TestSessionReceiptRejectsForeignBindingWithoutPartialChanges(t *testing.T) {
	s := b.LifecycleSnapshot{Session: b.SessionContext{SessionID: "original", PendingCommandID: "command"}, PendingTurn: &b.TurnInput{CommandID: "command"}}
	before, _ := json.Marshal(s)
	receipt := b.LifecycleReceipt{Workspace: &b.WorkspaceInspection{Worktree: "tree", Branch: "branch", Base: "base", Head: "head"}, State: b.LifecycleStateFacts{SessionID: "replacement", TurnID: "turn", InputAccepted: true, TurnStarted: true}}
	got, started, err := ApplySessionReceipt(s, nil, receipt, false)
	returned, _ := json.Marshal(got)
	after, _ := json.Marshal(s)
	if err == nil || started || !bytes.Equal(returned, before) || !bytes.Equal(after, before) {
		t.Fatal("foreign session leaked partial workspace or turn changes", got, started, err)
	}
}
