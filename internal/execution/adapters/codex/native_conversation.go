package codexworker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const (
	conversationCursorLimit  = 1024
	conversationPreviewLimit = 4096
	conversationSegmentLimit = 64 << 10
)

var (
	ErrConversationCursor      = errors.New("Codex conversation cursor differs from the original session")
	ErrConversationUnsupported = errors.New("Codex app-server does not publish paged thread items")
)

type codexConversationCursor struct {
	Binding string                 `json:"b"`
	Page    string                 `json:"p,omitempty"`
	Body    *codexConversationBody `json:"m,omitempty"`
}

type codexConversationBody struct {
	Turn   string `json:"t"`
	Item   string `json:"i"`
	Digest string `json:"d"`
	Offset int    `json:"o"`
	Limit  int    `json:"l"`
}

type codexThreadItem struct {
	TurnID string `json:"turnId"`
	Item   struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Text    string `json:"text"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"item"`
}

type codexThreadItems struct {
	Data []codexThreadItem `json:"data"`
	Next string            `json:"nextCursor"`
}

type codexConversationReader interface {
	listConversationItems(context.Context, string, string, int) (codexThreadItems, error)
}

func ReadNativeConversation(ctx context.Context, setup b.LifecycleSetup, target contracts.NativeSessionTarget, cursor string, limit int, userInputs map[string]bool) (contracts.NativeConversationPage, error) {
	history, err := nativeConversationBinding(setup, target)
	if err != nil {
		return contracts.NativeConversationPage{}, err
	}
	key := digestValue([]any{target, history.Identity()})
	selection, err := parseCodexConversationCursor(cursor, key)
	if err != nil {
		return contracts.NativeConversationPage{}, err
	}
	// The reader does not need the original worktree to exist. Its identity is
	// verified against the thread's recorded cwd, independent of process cwd.
	client, err := OpenHistory(ctx, history.Command, history.Home, history.Home, setup.ProcessHost.StartManaged)
	if err != nil {
		return contracts.NativeConversationPage{}, err
	}
	defer client.Close()
	if _, err := client.readThread(ctx, target.SessionID, history.Workspace, false); err != nil {
		return contracts.NativeConversationPage{}, err
	}
	return readCodexConversationPage(ctx, client, target, selection, limit, userInputs)
}

func nativeConversationBinding(setup b.LifecycleSetup, target contracts.NativeSessionTarget) (HistoryBinding, error) {
	snapshot := setup.Snapshot
	session := snapshot.Session
	command := session.CurrentCommandID
	if command == "" {
		command = session.PendingCommandID
	}
	if setup.ProcessHost == nil || !filepath.IsAbs(setup.RunDirectory) || !b.StableExecutionID(target.RunID) || !b.StableExecutionID(command) || !ValidNativeID(target.SessionID) ||
		session.WorkflowRunID != target.RunID || session.AttemptID != target.AttemptID || session.SessionID != target.SessionID ||
		snapshot.Ref.RunID != target.RunID || snapshot.Ref.AttemptID != target.AttemptID || snapshot.ResourceGeneration != target.Generation ||
		snapshot.Ref.Backend != b.BackendLocal || snapshot.Ref.Protocol != b.ProtocolLocalCodexAppServer || session.Agent.Name != "codex" {
		return HistoryBinding{}, errors.New("original Codex conversation identity is unverified")
	}
	profile, err := DecodeProfileConfigEnvelope(setup.ProfileData, ProfileConfigIdentity{
		ProjectID: session.ProjectID, ProfileID: setup.ProfileID, AcceptedConfigVersion: setup.AcceptedConfigVersion,
	})
	if err != nil {
		return HistoryBinding{}, err
	}
	path := localHistoryBindingPath(filepath.Join(setup.RunDirectory, "runs", target.RunID, "turns", command), target.SessionID)
	history, err := ReadHistoryBinding(path)
	if err != nil {
		return HistoryBinding{}, err
	}
	if history.ProjectID != session.ProjectID || history.CorrelationID != session.CorrelationID ||
		history.WorkflowID != session.WorkflowRunID || history.ExecutionID != session.ExecutionID ||
		history.AttemptID != session.AttemptID || history.SessionID != session.SessionID ||
		history.Protocol != snapshot.Ref.Protocol || history.TargetID != snapshot.Ref.Target.ID ||
		!sameWorkspace(history.Workspace, snapshot.Ref.WorkspacePath) || !filepath.IsAbs(history.Home) ||
		!sameWorkspace(history.Home, EnvironmentHome()) ||
		history.Command.Executable != profile.Executable || len(history.Command.Args) != 0 {
		return HistoryBinding{}, errors.New("original Codex conversation configuration or binding differs")
	}
	return history, nil
}

func (c *HistoryClient) listConversationItems(ctx context.Context, thread, cursor string, limit int) (codexThreadItems, error) {
	params := map[string]any{"threadId": thread, "limit": limit, "sortDirection": "desc"}
	if cursor != "" {
		params["cursor"] = cursor
	}
	raw, err := c.call(ctx, "thread/items/list", params)
	if err != nil {
		var rpc *historyRPCError
		if errors.As(err, &rpc) && rpc.code == -32601 {
			return codexThreadItems{}, ErrConversationUnsupported
		}
		if errors.As(err, &rpc) && rpc.code == -32602 && cursor != "" {
			return codexThreadItems{}, ErrConversationCursor
		}
		return codexThreadItems{}, err
	}
	var page codexThreadItems
	if err := json.Unmarshal(raw, &page); err != nil || page.Data == nil || len(page.Data) > limit {
		return codexThreadItems{}, errors.New("invalid Codex thread items page")
	}
	return page, nil
}

func parseCodexConversationCursor(raw, binding string) (codexConversationCursor, error) {
	cursor := codexConversationCursor{Binding: binding}
	if raw == "" {
		return cursor, nil
	}
	if len(raw) > conversationCursorLimit {
		return cursor, ErrConversationCursor
	}
	value, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(value, &cursor) != nil || cursor.Binding != binding ||
		cursor.Body == nil && cursor.Page == "" {
		return cursor, ErrConversationCursor
	}
	if body := cursor.Body; body != nil &&
		(body.Turn == "" || body.Item == "" || len(body.Digest) != 64 || body.Offset < 0 || body.Limit < 1 || body.Limit > 50) {
		return cursor, ErrConversationCursor
	}
	return cursor, nil
}

func encodeCodexConversationCursor(cursor codexConversationCursor) string {
	raw, _ := json.Marshal(cursor)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > conversationCursorLimit {
		return ""
	}
	return encoded
}

func readCodexConversationPage(ctx context.Context, reader codexConversationReader, target contracts.NativeSessionTarget, cursor codexConversationCursor, limit int, userInputs map[string]bool) (contracts.NativeConversationPage, error) {
	out := contracts.NativeConversationPage{NativeSessionTarget: target, Messages: []contracts.NativeConversationMessage{}}
	if cursor.Body != nil {
		limit = cursor.Body.Limit
	}
	native, err := reader.listConversationItems(ctx, target.SessionID, cursor.Page, limit)
	if err != nil {
		return out, err
	}
	if native.Next != "" && native.Next == cursor.Page {
		return out, ErrConversationCursor
	}
	seen := make(map[string]bool)
	for _, item := range native.Data {
		message, visible := item.publicMessage(userInputs)
		if !visible {
			continue
		}
		if message.ID == "" || message.TurnID == "" || seen[message.ID] {
			return out, errors.New("Codex conversation message identity is unverified")
		}
		seen[message.ID] = true
		if body := cursor.Body; body != nil {
			if message.ID == body.Item && message.TurnID == body.Turn {
				return readCodexConversationBody(out, cursor, message)
			}
			continue
		}
		if len(message.Text) > conversationPreviewLimit {
			message.BodyCursor = encodeCodexConversationCursor(codexConversationCursor{Binding: cursor.Binding, Page: cursor.Page, Body: &codexConversationBody{
				Turn: message.TurnID, Item: message.ID, Digest: digestValue(message.Text), Limit: limit,
			}})
			message.Text = message.Text[:codexConversationTextEnd(message.Text, 0, conversationPreviewLimit)]
			message.Truncated = true
			out.Partial = out.Partial || message.BodyCursor == ""
		}
		out.Messages = append(out.Messages, message)
	}
	if cursor.Body != nil {
		return out, ErrConversationCursor
	}
	// App-server lists newest first. The public page is chronological.
	slices.Reverse(out.Messages)
	if native.Next != "" {
		out.NextCursor = encodeCodexConversationCursor(codexConversationCursor{Binding: cursor.Binding, Page: native.Next})
		out.Partial = out.Partial || out.NextCursor == ""
	}
	return out, nil
}

func (item codexThreadItem) publicMessage(userInputs map[string]bool) (contracts.NativeConversationMessage, bool) {
	message := contracts.NativeConversationMessage{ID: item.Item.ID, TurnID: item.TurnID}
	switch item.Item.Type {
	case "agentMessage":
		message.Role, message.Text = "assistant", item.Item.Text
	case "userMessage":
		var text strings.Builder
		for _, part := range item.Item.Content {
			if part.Type != "text" {
				return message, false
			}
			text.WriteString(part.Text)
		}
		message.Role, message.Text = "user", text.String()
		// App-server uses the user role for assembled Runner input as well.
		if !userInputs[message.Text] {
			return message, false
		}
	default:
		return message, false
	}
	return message, strings.TrimSpace(message.Text) != "" && utf8.ValidString(message.Text)
}

func readCodexConversationBody(out contracts.NativeConversationPage, cursor codexConversationCursor, message contracts.NativeConversationMessage) (contracts.NativeConversationPage, error) {
	body := *cursor.Body
	if body.Digest != digestValue(message.Text) || body.Offset >= len(message.Text) || !utf8.RuneStart(message.Text[body.Offset]) {
		return out, ErrConversationCursor
	}
	offset := body.Offset
	end := codexConversationTextEnd(message.Text, offset, conversationSegmentLimit)
	if end < len(message.Text) {
		body.Offset = end
		cursor.Body = &body
		out.NextCursor = encodeCodexConversationCursor(cursor)
		if out.NextCursor == "" {
			return out, ErrConversationCursor
		}
	}
	message.Text = message.Text[offset:end]
	out.BodyOffset = &offset
	out.Messages = append(out.Messages, message)
	return out, nil
}

func codexConversationTextEnd(text string, start, limit int) int {
	end := min(len(text), start+limit)
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end--
	}
	return end
}
