package codexworker

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	"testing"
	"time"
)

func TestPublicMessageWhitelistAndIdentity(t *testing.T) {
	at := time.Now().UTC()
	for _, tt := range []struct {
		method, kind, thread, turn, text string
		want                             bool
	}{
		{"item/completed", "agentMessage", "thread", "turn", "公开消息", true},
		{"item/started", "commandExecution", "thread", "turn", "secret command output", true},
		{"item/completed", "mcpToolCall", "thread", "turn", "secret arguments", true},
		{"item/completed", "reasoning", "thread", "turn", "private", false},
		{"item/completed", "userMessage", "thread", "turn", "user", false},
		{"item/agentMessage/delta", "agentMessage", "thread", "turn", "partial", false},
		{"item/completed", "agentMessage", "other", "turn", "wrong", false},
		{"item/completed", "agentMessage", "thread", "old", "wrong", false},
	} {
		raw, _ := json.Marshal(map[string]any{"threadId": tt.thread, "turnId": tt.turn, "item": map[string]any{"type": tt.kind, "text": tt.text, "command": "sensitive", "arguments": "sensitive", "aggregatedOutput": "sensitive"}})
		m := publicMessage(codexapp.Envelope{Method: tt.method, Params: raw}, "thread", "turn", at)
		if (m != nil) != tt.want {
			t.Fatalf("%s/%s: %+v", tt.method, tt.kind, m)
		}
		if m != nil && (m.ObservedAt != at || (m.Kind == "tool" && m.Text == tt.text)) {
			t.Fatal("source time lost or tool content exposed")
		}
	}
}
