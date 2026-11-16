package contracts

const ConversationEventsCommand = "conversation-events"
const ConversationEventsContract = "steerrunner-conversation-events/v1"

type ConversationEvent struct {
	ID     string `json:"id"`
	TurnID string `json:"turn_id"`
	ItemID string `json:"item_id,omitempty"`
	Kind   string `json:"kind"`
	Text   string `json:"text,omitempty"`
	Tool   string `json:"tool,omitempty"`
	Status string `json:"status,omitempty"`
}

type ConversationEventPage struct {
	Contract   string              `json:"contract"`
	RunID      string              `json:"run_id"`
	SessionID  string              `json:"session_id"`
	Events     []ConversationEvent `json:"events"`
	NextCursor string              `json:"next_cursor,omitempty"`
	HasMore    bool                `json:"has_more"`
	Active     bool                `json:"active"`
}
