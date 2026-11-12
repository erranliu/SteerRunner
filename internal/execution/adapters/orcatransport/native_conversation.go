package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type conversationCursor struct {
	Binding string
	Before  *native.AgentCursor
	Body    *conversationBody
}

type conversationBody struct {
	ID       string
	Revision int64
	Sequence int64
	Index    int64
	Offset   int
	Digest   string
}

type conversationReader interface {
	History(context.Context, string, *native.AgentCursor, int) (native.AgentPage, error)
	Item(context.Context, string, string) (native.AgentItem, error)
}

func ReadNativeConversation(ctx context.Context, setup b.LifecycleSetup, target contracts.NativeSessionTarget, cursor string, limit int, userInputs map[string]bool) (contracts.NativeConversationPage, error) {
	profile, err := decodeAgentSessionProfile(setup)
	if err != nil {
		return contracts.NativeConversationPage{}, err
	}
	snapshot := setup.Snapshot
	state, err := readAgentSessionState(snapshot, profile.Transport, profile.Development, snapshot)
	if err != nil || state.SessionID != target.SessionID || state.Epoch == "" || state.RuntimeID == "" {
		return contracts.NativeConversationPage{}, errors.New("original Orca conversation binding is unavailable")
	}
	client := originalAgentClient(profile.Transport, profile.PairingHostID, snapshot.Ref.Agent.Name, state.RuntimeID)
	return readNativeConversationPage(ctx, client, state, target, cursor, limit, userInputs)
}

var errConversationCursor = errors.New("conversation cursor differs from the original session")

func IsConversationCursorError(err error) bool {
	return errors.Is(err, errConversationCursor)
}

func readNativeConversationPage(ctx context.Context, client conversationReader, state agentSessionState, target contracts.NativeSessionTarget, cursor string, limit int, userInputs map[string]bool) (contracts.NativeConversationPage, error) {
	out := contracts.NativeConversationPage{NativeSessionTarget: target, Messages: []contracts.NativeConversationMessage{}}
	binding, _ := json.Marshal(target)
	key := digest(append(binding, ("\x00" + state.Epoch)...))
	var before *native.AgentCursor
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		var saved conversationCursor
		if err != nil || json.Unmarshal(raw, &saved) != nil || saved.Binding != key || (saved.Before == nil) == (saved.Body == nil) {
			return out, errConversationCursor
		}
		if saved.Body != nil {
			return readConversationBody(ctx, client, state, out, key, *saved.Body, userInputs)
		}
		if saved.Before.Epoch != state.Epoch || saved.Before.Sequence <= 0 {
			return out, errConversationCursor
		}
		before = saved.Before
	}
	page, err := client.History(ctx, state.SessionID, before, limit)
	if err != nil {
		return out, err
	}
	if page.SessionID != state.SessionID || page.Epoch != state.Epoch || before == nil && page.HasNewer || len(page.Items) > limit {
		return out, errors.New("original conversation page changed")
	}
	out.Partial = len(page.RemovedItemIDs) > 0
	sort.Slice(page.Items, func(i, j int) bool {
		a, b := page.Items[i], page.Items[j]
		if a.Sequence == b.Sequence {
			return a.SequenceIndex < b.SequenceIndex
		}
		return a.Sequence < b.Sequence
	})
	seen := make(map[string]bool)
	for i, item := range page.Items {
		if item.ItemID == "" || seen[item.ItemID] || before != nil && item.Sequence > before.Sequence ||
			i > 0 && item.Sequence == page.Items[i-1].Sequence && item.SequenceIndex == page.Items[i-1].SequenceIndex {
			return out, errors.New("original conversation order is ambiguous")
		}
		seen[item.ItemID] = true
		role, value, valid := conversationText(item, userInputs)
		out.Partial = out.Partial || !valid
		if role == "" {
			out.Partial = out.Partial || item.Shortened
			continue
		}
		message := contracts.NativeConversationMessage{
			ID: item.ItemID, Role: role, Text: value, TurnID: item.TurnScope.TurnItemID,
		}
		if len(value) > 4096 || item.Shortened {
			message.Text = value[:conversationTextEnd(value, 0, 4096)]
			message.Truncated = true
			message.BodyCursor = encodeConversationCursor(conversationCursor{Binding: key, Body: &conversationBody{
				ID: item.ItemID, Revision: item.Revision, Sequence: item.Sequence, Index: item.SequenceIndex,
			}})
			if len(message.BodyCursor) > 1024 {
				message.BodyCursor = ""
				out.Partial = true
			}
		}
		out.Messages = append(out.Messages, message)
	}
	if page.HasOlder {
		next := page.Window.NextCursor
		if next == nil || next.Epoch != state.Epoch || next.Sequence <= 0 || before != nil && next.Sequence >= before.Sequence {
			return out, errors.New("original conversation cursor did not advance")
		}
		out.NextCursor = encodeConversationCursor(conversationCursor{Binding: key, Before: next})
	}
	return out, nil
}

func encodeConversationCursor(cursor conversationCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func conversationText(item native.AgentItem, userInputs map[string]bool) (role, value string, valid bool) {
	var body agentHistoryMessage
	if json.Unmarshal(item.Body, &body) != nil {
		return "", "", false
	}
	if body.Kind != "message" || (body.Role != "user" && body.Role != "assistant") {
		return "", "", true
	}
	var text strings.Builder
	for _, block := range body.Blocks {
		switch {
		case block.Type == "text":
			text.WriteString(block.Text)
		case body.Role == "assistant" && isAsyncAgentQuestion(block) && block.State != "failed":
			for _, question := range parseAsyncAgentQuestions(block.Input) {
				if text.Len() != 0 {
					text.WriteString("\n\n")
				}
				text.WriteString(question.Question)
				for _, option := range question.Options {
					text.WriteString("\n- ")
					text.WriteString(option.Label)
				}
			}
		}
	}
	value = text.String()
	// Orca also labels assembled workflow instructions and materials as user
	// items. Human text must match a caller input independently saved by Runner.
	if body.Role == "user" && !userInputs[value] || strings.TrimSpace(value) == "" && !item.Shortened {
		return "", "", true
	}
	return body.Role, value, true
}

func conversationTextEnd(text string, start, limit int) int {
	end := min(len(text), start+limit)
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end--
	}
	return end
}

func readConversationBody(ctx context.Context, client conversationReader, state agentSessionState, page contracts.NativeConversationPage, key string, body conversationBody, userInputs map[string]bool) (contracts.NativeConversationPage, error) {
	if body.ID == "" || body.Offset < 0 || (body.Offset != 0 && body.Digest == "") {
		return page, errConversationCursor
	}
	item, err := client.Item(ctx, state.SessionID, body.ID)
	if err != nil {
		return page, err
	}
	if item.ItemID != body.ID || item.Revision != body.Revision || item.Sequence != body.Sequence ||
		item.SequenceIndex != body.Index || item.Shortened {
		return page, errConversationCursor
	}
	role, text, valid := conversationText(item, userInputs)
	if !valid || role == "" || len(text) > 16<<20 || body.Offset >= len(text) || !utf8.RuneStart(text[body.Offset]) {
		return page, errConversationCursor
	}
	hash := digest([]byte(text))
	if body.Digest != "" && body.Digest != hash {
		return page, errConversationCursor
	}
	offset := body.Offset
	page.BodyOffset = &offset
	end := conversationTextEnd(text, offset, 64<<10)
	page.Messages = append(page.Messages, contracts.NativeConversationMessage{
		ID: item.ItemID, Role: role, Text: text[body.Offset:end], TurnID: item.TurnScope.TurnItemID,
	})
	if end < len(text) {
		body.Offset, body.Digest = end, hash
		page.NextCursor = encodeConversationCursor(conversationCursor{Binding: key, Body: &body})
	}
	return page, nil
}
