package codexworker

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestRemoteTurnMatchesCompleteOriginalInputAndNativeFinal(t *testing.T) {
	for _, mode := range []string{"completed", "in-progress", "failed", "interrupted", "wrong-input", "duplicate", "later-input", "missing-turn", "missing-user", "missing-final-id", "commentary", "async", "duplicate-final", "no-completed-at"} {
		t.Run(mode, func(t *testing.T) {
			input := "[SteerRunner command=unique input=fullhash]\nReport precisely."
			sum := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
			at := int64(1234)
			user := map[string]any{"id": "user-item", "type": "userMessage", "content": []any{map[string]any{"type": "text", "text": input}}}
			final := map[string]any{"id": "final-item", "type": "agentMessage", "phase": "final_answer", "text": "exact final\n"}
			turn := HistoryTurn{ID: "01a14502-5798-78f1-aaa3-2d7cd15594c8", Status: "completed", CompletedAt: &at}
			switch mode {
			case "in-progress":
				turn.Status = "inProgress"
				turn.CompletedAt = nil
			case "failed", "interrupted":
				turn.Status = mode
			case "wrong-input":
				sum = fmt.Sprintf("%x", sha256.Sum256([]byte(input+"changed")))
			case "missing-turn":
				turn.ID = ""
			case "missing-user":
				user["id"] = ""
			case "missing-final-id":
				final["id"] = ""
			case "commentary":
				final["phase"] = "commentary"
			case "async":
				final["delivery"] = "async"
			case "no-completed-at":
				turn.CompletedAt = nil
			}
			u, _ := json.Marshal(user)
			f, _ := json.Marshal(final)
			turn.Items = []json.RawMessage{json.RawMessage(`{"type":"reasoning","id":"reasoning-item","content":["valid native reasoning content"],"summary":["summary"]}`), json.RawMessage(`{"type":"mcpToolCall","content":[{"type":"image","data":"public-fixture"}]}`), u, f}
			if mode == "duplicate-final" {
				turn.Items = append(turn.Items, f)
			}
			thread := HistoryThread{Turns: []HistoryTurn{turn}}
			if mode == "duplicate" {
				thread.Turns = append(thread.Turns, turn)
			}
			if mode == "later-input" {
				thread.Turns = append(thread.Turns, HistoryTurn{Items: []json.RawMessage{json.RawMessage(`{"type":"userMessage","id":"another","content":[{"type":"text","text":"other input"}]}`)}})
			}
			got, err := MatchRemoteTurn(thread, sum)
			wantErr := mode == "duplicate" || mode == "later-input" || mode == "missing-turn" || mode == "missing-user" || mode == "missing-final-id" || mode == "duplicate-final"
			if (err != nil) != wantErr {
				t.Fatalf("receipt=%+v err=%v", got, err)
			}
			if wantErr {
				return
			}
			if mode == "wrong-input" {
				if got.Started || got.Completed || got.FinalText != "" {
					t.Fatal(got)
				}
				return
			}
			if !got.Started || got.NativeTurnID != turn.ID || got.UserItemID != "user-item" {
				t.Fatal(got)
			}
			if got.Completed != (mode != "in-progress" && mode != "no-completed-at") {
				t.Fatal(got)
			}
			if mode == "commentary" || mode == "async" {
				if got.FinalText != "" {
					t.Fatal("non-final promoted", got)
				}
			} else if got.FinalText != "exact final\n" || got.FinalItemID != "final-item" {
				t.Fatal(got)
			}
		})
	}
}
