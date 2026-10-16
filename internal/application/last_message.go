package application

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"regexp"
	"strings"
	"unicode/utf8"
)

var credentialHint = regexp.MustCompile(`(?i)(?:bearer\s+\S+|(?:gh[pousr]_|github_pat_|sk-)[a-z0-9_-]+|(?:token|password|secret|api[_-]?key)["']?\s*[:=]\s*["']?[^\s,;"']+)`)

func (e *Engine) acceptMessage(id string, m b.PublicMessage) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id]
	if r == nil {
		return nil
	}
	s := r.Snapshot.Session
	if m.AttemptID != s.AttemptID || m.Generation != s.Generation || m.SessionID != s.SessionID || m.TurnID == "" || (m.TurnID != s.CurrentTurnID && m.TurnID != s.PendingTurnID) {
		return nil
	}
	if m.Kind != "assistant" && m.Kind != "tool" || m.ObservedAt.IsZero() {
		return nil
	}
	text := credentialHint.ReplaceAllString(e.redact(r, m.Text), "[REDACTED]")
	text = strings.ToValidUTF8(text, "")
	if len(text) > 1200 {
		text = text[:1200]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		text += "…"
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if e.lastMessages == nil {
		e.lastMessages = map[string]contracts.LastMessage{}
	}
	if old, ok := e.lastMessages[id]; ok && !m.ObservedAt.After(old.ObservedAt) {
		return nil
	}
	e.lastMessages[id] = contracts.LastMessage{RunID: id, AttemptID: m.AttemptID, SessionID: m.SessionID, TurnID: m.TurnID, Kind: m.Kind, Text: text, ObservedAt: m.ObservedAt}
	return nil
}
