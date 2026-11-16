package application

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type conversationTurn struct {
	command, turn, outcome string
}

var errConversationCursor = errors.New("invalid conversation cursor")

func (e *Engine) ReadConversationEvents(runID, cursor string, limit int) contracts.Response {
	if limit == 0 {
		limit = 64
	}
	if limit < 1 || limit > 100 || len(cursor) > 160 {
		return contracts.Rejected("", contracts.Fail(2, "invalid_arguments", "Invalid conversation event bounds."))
	}
	e.mu.Lock()
	r := e.runs[runID]
	e.mu.Unlock()
	if r == nil {
		return contracts.Rejected("", contracts.Fail(5, "run_not_found", "Original Run was not found."))
	}
	if !r.ManagedConversation() || r.Request.Execution.Adapter != "codex-app-server" {
		return contracts.Rejected("", contracts.Fail(3, "unsupported_capability", "Conversation events require a managed local Codex conversation."))
	}

	page := contracts.ConversationEventPage{
		Contract: contracts.ConversationEventsContract, RunID: runID,
		SessionID: r.Snapshot.Session.SessionID, Events: []contracts.ConversationEvent{},
	}
	var turns []conversationTurn
	for _, result := range r.Workflow.StageResults {
		if result.SessionID == page.SessionID && result.TurnID != "" {
			outcome := result.Outcome
			if outcome == "" {
				outcome = "unreported"
			}
			turns = append(turns, conversationTurn{result.CommandID, result.TurnID, outcome})
		}
	}
	session := r.Snapshot.Session
	if r.Snapshot.Ref.Protocol == b.ProtocolLocalCodexAppServer && session.CurrentTurnID != "" &&
		(len(turns) == 0 || turns[len(turns)-1].turn != session.CurrentTurnID) {
		turns = append(turns, conversationTurn{command: session.CurrentCommandID, turn: session.CurrentTurnID})
	}
	page.Active = len(turns) > 0 && turns[len(turns)-1].outcome == "" && !r.Closed
	if err := readConversationEvents(e.dir, &page, turns, cursor, limit); err != nil {
		if errors.Is(err, errConversationCursor) {
			return contracts.Rejected("", contracts.Fail(2, "invalid_cursor", "Conversation event cursor does not match the original record."))
		}
		return contracts.Rejected("", contracts.Fail(4, "environment_unavailable", "Original conversation events could not be read."))
	}
	return contracts.Success("", page)
}

func readConversationEvents(root string, page *contracts.ConversationEventPage, turns []conversationTurn, cursor string, limit int) error {
	digest := sha256.Sum256([]byte(page.RunID + "\x00" + page.SessionID))
	binding := hex.EncodeToString(digest[:12])
	var start int
	var offset int64
	if cursor != "" {
		parts := strings.Split(cursor, ":")
		if len(parts) != 3 || parts[0] != binding {
			return errConversationCursor
		}
		for start < len(turns) && turns[start].command != parts[1] {
			start++
		}
		value, err := strconv.ParseInt(parts[2], 10, 64)
		if start == len(turns) || err != nil || value < -1 || value == -1 && turns[start].outcome == "" {
			return errConversationCursor
		}
		offset = value
		page.NextCursor = cursor
	}
	budget := int64(8 << 20)
	textBudget := 256 << 10
	for _, turn := range turns[start:] {
		if offset != -1 {
			if len(page.Events) == limit || budget <= 0 || textBudget < 64<<10 {
				page.HasMore = true
				return nil
			}
			if !conversationPathID(page.RunID) || !conversationPathID(turn.command) {
				return errors.New("invalid original journal identity")
			}
			path := filepath.Join(root, "runs", page.RunID, "turns", turn.command, "appserver.jsonl")
			next, eof, err := readConversationJournal(path, page, turn, offset, limit, &budget, &textBudget)
			if err != nil {
				return err
			}
			page.NextCursor = fmt.Sprintf("%s:%s:%d", binding, turn.command, next)
			if !eof {
				page.HasMore = true
				return nil
			}
			if turn.outcome == "" {
				return nil
			}
			if len(page.Events) == limit {
				page.HasMore = true
				return nil
			}
			page.Events = append(page.Events, contracts.ConversationEvent{
				ID: turn.command + ":end", TurnID: turn.turn, Kind: "turn_settled", Status: turn.outcome,
			})
			page.NextCursor = binding + ":" + turn.command + ":-1"
		}
		offset = 0
	}
	return nil
}

func conversationPathID(id string) bool {
	_, err := hex.DecodeString(id)
	return err == nil && len(id) == 32
}

func readConversationJournal(path string, page *contracts.ConversationEventPage, turn conversationTurn, offset int64, limit int, budget *int64, textBudget *int) (int64, bool, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) && offset == 0 {
		return 0, true, nil
	}
	if err != nil || !strings.EqualFold(resolved, path) {
		return offset, false, errors.New("original journal path could not be verified")
	}
	file, err := os.Open(resolved)
	if err != nil {
		return offset, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return offset, false, errors.New("original journal is not a regular file")
	}
	if offset > 0 {
		var boundary [1]byte
		if _, err := file.ReadAt(boundary[:], offset-1); err != nil || boundary[0] != '\n' {
			return offset, false, errConversationCursor
		}
	}
	reader := bufio.NewReaderSize(io.NewSectionReader(file, offset, info.Size()-offset), 1<<20)
	for *budget > 0 && len(page.Events) < limit && *textBudget >= 64<<10 {
		line, err := reader.ReadSlice('\n')
		size := int64(len(line))
		oversized := errors.Is(err, bufio.ErrBufferFull)
		for errors.Is(err, bufio.ErrBufferFull) {
			var tail []byte
			tail, err = reader.ReadSlice('\n')
			size += int64(len(tail))
		}
		if errors.Is(err, io.EOF) {
			return offset, true, nil
		}
		if err != nil {
			return offset, false, err
		}
		offset += size
		*budget -= size
		if !oversized {
			if event := projectConversationEvent(line, page.SessionID, turn); event.Kind != "" {
				event.ID = fmt.Sprintf("%s:%d", turn.command, offset)
				page.Events = append(page.Events, event)
				*textBudget -= len(event.Text)
			}
		}
	}
	return offset, false, nil
}

func projectConversationEvent(line []byte, session string, turn conversationTurn) contracts.ConversationEvent {
	var envelope struct {
		Method string
		Params json.RawMessage
	}
	if json.Unmarshal(line, &envelope) != nil {
		return contracts.ConversationEvent{}
	}
	switch envelope.Method {
	case "item/agentMessage/delta", "item/started", "item/completed":
	default:
		return contracts.ConversationEvent{}
	}
	var params struct {
		ThreadID string
		TurnID   string
		ItemID   string
		Delta    string
		Item     struct {
			ID, Type, Text, Status string
			ExitCode               *int
			Success                *bool
		}
	}
	if json.Unmarshal(envelope.Params, &params) != nil || params.ThreadID != session || params.TurnID != turn.turn {
		return contracts.ConversationEvent{}
	}
	event := contracts.ConversationEvent{TurnID: turn.turn, ItemID: params.Item.ID}
	if envelope.Method == "item/agentMessage/delta" && params.ItemID != "" && len(params.ItemID) <= 256 && params.Delta != "" && len(params.Delta) <= 64<<10 {
		event.Kind, event.ItemID, event.Text = "assistant_delta", params.ItemID, params.Delta
		return event
	}
	if event.ItemID == "" || len(event.ItemID) > 256 {
		return contracts.ConversationEvent{}
	}
	switch params.Item.Type {
	case "agentMessage":
		if envelope.Method == "item/started" {
			event.Kind = "assistant_started"
			return event
		}
		if envelope.Method == "item/completed" && len(params.Item.Text) <= 64<<10 {
			event.Kind, event.Text = "assistant_completed", params.Item.Text
			return event
		}
	case "commandExecution", "fileChange", "mcpToolCall", "dynamicToolCall", "webSearch":
		event.Kind, event.Tool, event.Status = "tool", params.Item.Type, "running"
		if envelope.Method == "item/completed" {
			switch params.Item.Status {
			case "failed", "declined":
				event.Status = "failed"
			case "cancelled", "interrupted":
				event.Status = "cancelled"
			default:
				event.Status = "completed"
			}
			if params.Item.Success != nil && !*params.Item.Success || params.Item.ExitCode != nil && *params.Item.ExitCode != 0 {
				event.Status = "failed"
			}
		}
		return event
	}
	return contracts.ConversationEvent{}
}
