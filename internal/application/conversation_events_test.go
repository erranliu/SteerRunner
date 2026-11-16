package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestConversationEventsResumeOnlyOriginalJournal(t *testing.T) {
	root := t.TempDir()
	run, command := strings.Repeat("a", 32), strings.Repeat("b", 32)
	dir := filepath.Join(root, "runs", run, "turns", command)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "appserver.jsonl")
	line := func(body string) string { return body + "\n" }
	original := line(`{"method":"item/agentMessage/delta","params":{"threadId":"thread","turnId":"turn","itemId":"message","delta":"hello "}}`)
	foreign := line(`{"method":"item/agentMessage/delta","params":{"threadId":"other","turnId":"turn","itemId":"message","delta":"wrong"}}`)
	tool := line(`{"method":"item/completed","params":{"threadId":"thread","turnId":"turn","item":{"id":"tool","type":"commandExecution","status":"completed","exitCode":7,"command":"private"}}}`)
	suffix := `{"method":"item/agentMessage/delta","params":{"threadId":"thread","turnId":"turn","itemId":"message","delta":"world"}}`
	if err := os.WriteFile(path, []byte(original+foreign+tool+suffix), 0600); err != nil {
		t.Fatal(err)
	}

	turns := []conversationTurn{{command: command, turn: "turn"}}
	read := func(session, after string, limit int) (contracts.ConversationEventPage, error) {
		page := contracts.ConversationEventPage{RunID: run, SessionID: session}
		err := readConversationEvents(root, &page, turns, after, limit)
		return page, err
	}
	first, err := read("thread", "", 1)
	if err != nil || len(first.Events) != 1 || first.Events[0].Text != "hello " || !first.HasMore {
		t.Fatalf("first page: %+v %v", first, err)
	}
	if _, err := read("other", first.NextCursor, 10); err != errConversationCursor {
		t.Fatalf("foreign cursor: %v", err)
	}
	second, err := read("thread", first.NextCursor, 10)
	if err != nil || len(second.Events) != 1 || second.Events[0].Tool != "commandExecution" ||
		second.Events[0].Status != "failed" || second.Events[0].Text != "" || second.HasMore {
		t.Fatalf("filtered activities: %+v %v", second, err)
	}
	if err := os.WriteFile(path, []byte(original+foreign+tool+line(suffix)), 0600); err != nil {
		t.Fatal(err)
	}
	turns[0].outcome = "cancelled"
	last, err := read("thread", second.NextCursor, 10)
	if err != nil || len(last.Events) != 2 || last.Events[0].Text != "world" ||
		last.Events[1].Kind != "turn_settled" || last.Events[1].Status != "cancelled" {
		t.Fatalf("resume and settlement: %+v %v", last, err)
	}
	repeated, err := read("thread", last.NextCursor, 10)
	if err != nil || len(repeated.Events) != 0 || repeated.NextCursor != last.NextCursor {
		t.Fatalf("settled cursor: %+v %v", repeated, err)
	}
}
