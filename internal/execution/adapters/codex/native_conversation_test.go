package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

type codexConversationFixture map[string]codexThreadItems

func (f codexConversationFixture) listConversationItems(_ context.Context, thread, cursor string, limit int) (codexThreadItems, error) {
	page, ok := f[cursor]
	if !ok || thread != "thread" || len(page.Data) > limit {
		return page, errors.New("wrong native history query")
	}
	return page, nil
}

func codexConversationItem(t *testing.T, id, kind, text string) codexThreadItem {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"turnId": "turn", "item": map[string]any{
		"id": id, "type": kind, "text": text, "content": []map[string]string{{"type": "text", "text": text}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var item codexThreadItem
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestNativeConversationCodexPagesFilterAndPreserveOrder(t *testing.T) {
	target := contracts.NativeSessionTarget{SessionID: "thread"}
	fixture := codexConversationFixture{
		"": {Data: []codexThreadItem{
			codexConversationItem(t, "final", "agentMessage", "final reply"),
			codexConversationItem(t, "tool", "commandExecution", "tool output"),
			codexConversationItem(t, "public", "agentMessage", "before final"),
		}, Next: "older"},
		"older": {Data: []codexThreadItem{
			codexConversationItem(t, "reasoning", "reasoning", "internal reasoning"),
			codexConversationItem(t, "human", "userMessage", "actual human input"),
			codexConversationItem(t, "prompt", "userMessage", "assembled instruction"),
		}},
	}
	users := map[string]bool{"actual human input": true}
	start := codexConversationCursor{Binding: "original"}
	first, err := readCodexConversationPage(t.Context(), fixture, target, start, 3, users)
	if err != nil || len(first.Messages) != 2 || first.Messages[0].ID != "public" || first.Messages[1].ID != "final" || first.NextCursor == "" {
		t.Fatalf("invalid latest page: %+v, %v", first, err)
	}
	if _, err := parseCodexConversationCursor(first.NextCursor, "different"); !errors.Is(err, ErrConversationCursor) {
		t.Fatalf("cursor crossed bindings: %v", err)
	}
	next, err := parseCodexConversationCursor(first.NextCursor, start.Binding)
	if err != nil {
		t.Fatal(err)
	}
	older, err := readCodexConversationPage(t.Context(), fixture, target, next, 3, users)
	if err != nil || len(older.Messages) != 1 || older.Messages[0].ID != "human" || older.Messages[0].Role != "user" || older.NextCursor != "" {
		t.Fatalf("invalid older page: %+v, %v", older, err)
	}
}

func TestNativeConversationCodexBodyChecksDigestAndByteOffsets(t *testing.T) {
	target := contracts.NativeSessionTarget{SessionID: "thread"}
	text := strings.Repeat("中文", 12000)
	fixture := codexConversationFixture{"": {Data: []codexThreadItem{codexConversationItem(t, "long", "agentMessage", text)}}}
	page, err := readCodexConversationPage(t.Context(), fixture, target, codexConversationCursor{Binding: "original"}, 1, nil)
	if err != nil || len(page.Messages) != 1 || !page.Messages[0].Truncated || len(page.Messages[0].Text) > 4096 || !utf8.ValidString(page.Messages[0].Text) {
		t.Fatalf("invalid preview: %v", err)
	}
	cursor, err := parseCodexConversationCursor(page.Messages[0].BodyCursor, "original")
	if err != nil {
		t.Fatal(err)
	}
	segment, err := readCodexConversationPage(t.Context(), fixture, target, cursor, 50, nil)
	if err != nil || segment.BodyOffset == nil || *segment.BodyOffset != 0 || len(segment.Messages) != 1 ||
		len(segment.Messages[0].Text) > 64<<10 || !utf8.ValidString(segment.Messages[0].Text) || segment.NextCursor == "" {
		t.Fatalf("invalid first segment: %v", err)
	}
	continuation, err := parseCodexConversationCursor(segment.NextCursor, "original")
	if err != nil {
		t.Fatal(err)
	}
	last, err := readCodexConversationPage(t.Context(), fixture, target, continuation, 50, nil)
	if err != nil || last.BodyOffset == nil || *last.BodyOffset != len(segment.Messages[0].Text) || len(last.Messages) != 1 ||
		segment.Messages[0].Text+last.Messages[0].Text != text || last.NextCursor != "" {
		t.Fatalf("invalid continuation: %v", err)
	}
	fixture[""] = codexThreadItems{Data: []codexThreadItem{codexConversationItem(t, "long", "agentMessage", text+" changed")}}
	if _, err = readCodexConversationPage(t.Context(), fixture, target, continuation, 1, nil); !errors.Is(err, ErrConversationCursor) {
		t.Fatalf("changed native body was accepted: %v", err)
	}
}
