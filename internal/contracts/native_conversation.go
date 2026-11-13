package contracts

import "time"

const NativeConversationHistory = "native-conversation-history"

type NativeConversationMessage struct {
	ID            string     `json:"id"`
	Role          string     `json:"role"`
	Text          string     `json:"text"`
	TurnID        string     `json:"turn_id,omitempty"`
	TurnStartedAt *time.Time `json:"turn_started_at,omitempty"`
	BodyCursor    string     `json:"body_cursor,omitempty"`
	Truncated     bool       `json:"truncated,omitempty"`
}

type NativeConversationPage struct {
	NativeSessionTarget
	Messages   []NativeConversationMessage `json:"messages"`
	NextCursor string                      `json:"next_cursor,omitempty"`
	BodyOffset *int                        `json:"body_offset,omitempty"`
	Partial    bool                        `json:"partial"`
}
