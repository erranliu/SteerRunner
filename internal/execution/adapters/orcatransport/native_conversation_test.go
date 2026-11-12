package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

func TestNativeConversationPagesOnlyReadOriginalPublicMessages(t *testing.T) {
	item := func(id string, sequence int64, body string) native.AgentItem {
		return native.AgentItem{ItemID: id, Sequence: sequence, Body: json.RawMessage(body)}
	}
	target := contracts.NativeSessionTarget{RunID: "run", AttemptID: "attempt", SessionID: "session", Generation: "generation"}
	state := agentSessionState{SessionID: target.SessionID, Epoch: "epoch"}
	tail := native.AgentPage{SessionID: state.SessionID, Epoch: state.Epoch, HasOlder: true, Items: []native.AgentItem{
		item("last", 6, `{"kind":"message","role":"assistant","blocks":[{"type":"text","text":"Last assistant message"}]}`),
		item("tool", 5, `{"kind":"message","role":"assistant","blocks":[{"type":"tool-result","text":"private output"}]}`),
		item("reply", 4, `{"kind":"message","role":"user","blocks":[{"type":"text","text":"Explicit user reply"}]}`),
	}}
	tail.Window.NextCursor = &native.AgentCursor{Epoch: state.Epoch, Sequence: 4}
	older := native.AgentPage{SessionID: state.SessionID, Epoch: state.Epoch, HasNewer: true, Items: []native.AgentItem{
		item("earlier", 3, `{"kind":"message","role":"assistant","blocks":[{"type":"text","text":"Earlier assistant message"}]}`),
		item("prompt", 2, `{"kind":"message","role":"user","blocks":[{"type":"text","text":"Composed internal instructions"}]}`),
	}}
	f := &agentFixture{pages: []native.AgentPage{tail, older}}
	inputs := map[string]bool{"Explicit user reply": true}
	first, err := readNativeConversationPage(context.Background(), f, state, target, "", 25, inputs)
	if err != nil || f.reads != 1 || len(first.Messages) != 2 || first.Messages[0].ID != "reply" || first.Messages[1].ID != "last" || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v, reads=%d, error=%v", first, f.reads, err)
	}
	second, err := readNativeConversationPage(context.Background(), f, state, target, first.NextCursor, 25, inputs)
	if err != nil || f.reads != 2 || len(second.Messages) != 1 || second.Messages[0].ID != "earlier" || second.NextCursor != "" {
		t.Fatalf("unexpected older page: %+v, reads=%d, error=%v", second, f.reads, err)
	}
	target.AttemptID = "another-attempt"
	if _, err := readNativeConversationPage(context.Background(), f, state, target, first.NextCursor, 25, inputs); !IsConversationCursorError(err) || f.reads != 2 {
		t.Fatalf("cursor crossed attempts or triggered a native read: %v", err)
	}
	target.AttemptID = "attempt"
	f.pages[1].Epoch = "replaced"
	if _, err := readNativeConversationPage(context.Background(), f, state, target, first.NextCursor, 25, inputs); err == nil {
		t.Fatal("accepted history from another native epoch")
	}
}

func TestNativeConversationBodyReadsOnlyTheSelectedItemAndKeepsUTF8Boundaries(t *testing.T) {
	target := contracts.NativeSessionTarget{RunID: "run", AttemptID: "attempt", SessionID: "session", Generation: "generation"}
	state := agentSessionState{SessionID: target.SessionID, Epoch: "epoch"}
	full := strings.Repeat("一", 25000)
	raw, _ := json.Marshal(agentHistoryMessage{Kind: "message", Role: "assistant", Blocks: []agentHistoryBlock{{Type: "text", Text: full}}})
	item := native.AgentItem{ItemID: "long", Revision: 4, Sequence: 1, Body: raw}
	f := &agentFixture{pages: []native.AgentPage{{SessionID: state.SessionID, Epoch: state.Epoch, Items: []native.AgentItem{item}}}, detail: item}
	preview, err := readNativeConversationPage(context.Background(), f, state, target, "", 25, nil)
	if err != nil || len(preview.Messages) != 1 || preview.Messages[0].BodyCursor == "" || len(preview.Messages[0].Text) != 4095 || f.details != 0 {
		t.Fatal("page did not provide a bounded lazy preview", err)
	}
	first, err := readNativeConversationPage(context.Background(), f, state, target, preview.Messages[0].BodyCursor, 25, nil)
	if err != nil || first.BodyOffset == nil || *first.BodyOffset != 0 || len(first.Messages) != 1 || len(first.Messages[0].Text) != 65535 || first.NextCursor == "" {
		t.Fatal("invalid first body segment", err)
	}
	last, err := readNativeConversationPage(context.Background(), f, state, target, first.NextCursor, 25, nil)
	if err != nil || last.BodyOffset == nil || *last.BodyOffset != 65535 || len(last.Messages) != 1 || first.Messages[0].Text+last.Messages[0].Text != full || last.NextCursor != "" || f.reads != 1 || f.details != 2 {
		t.Fatal("body segments failed to reconstruct the original item", err)
	}
	f.detail.Revision++
	if _, err := readNativeConversationPage(context.Background(), f, state, target, first.NextCursor, 25, nil); !IsConversationCursorError(err) {
		t.Fatalf("changed message revision accepted: %v", err)
	}
}
