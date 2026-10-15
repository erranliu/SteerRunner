package codexworker

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"time"
)

// Only completed public assistant messages and fixed tool labels are sampled.
// No deltas, reasoning, commands, arguments, tool output or user input is read.
func publicMessage(e codexapp.Envelope, thread, turn string, at time.Time) *b.PublicMessage {
	if e.Method != "item/started" && e.Method != "item/completed" {
		return nil
	}
	var p struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		Item     struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"item"`
	}
	if json.Unmarshal(e.Params, &p) != nil || p.ThreadID != thread || p.TurnID != turn {
		return nil
	}
	kind, text := "tool", ""
	switch p.Item.Type {
	case "agentMessage":
		if e.Method != "item/completed" {
			return nil
		}
		kind, text = "assistant", strings.TrimSpace(p.Item.Text)
	case "commandExecution":
		text = "正在执行命令"
	case "fileChange":
		text = "正在修改文件"
	case "mcpToolCall", "dynamicToolCall":
		text = "正在调用工具"
	case "webSearch":
		text = "正在搜索"
	default:
		return nil
	}
	if kind == "tool" && e.Method == "item/completed" {
		text = strings.Replace(text, "正在", "已完成", 1)
	}
	if text == "" {
		return nil
	}
	return &b.PublicMessage{SessionID: thread, TurnID: turn, Kind: kind, Text: text, ObservedAt: at}
}
