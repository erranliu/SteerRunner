package orcatransport

import "time"

// NativeProof is the frozen legacy wire format. New execution never recomputes it.
type NativeProof struct {
	Agent         string    `json:"agent,omitempty"`
	Pane          string    `json:"pane"`
	Provider      string    `json:"provider,omitempty"`
	BaseAuthority string    `json:"base_authority,omitempty"`
	BaseRevision  int64     `json:"base_revision,omitempty"`
	BaseRootAt    int64     `json:"base_root_at,omitempty"`
	Authority     string    `json:"authority,omitempty"`
	Incarnation   int64     `json:"incarnation,omitempty"`
	Revision      int64     `json:"revision,omitempty"`
	WorkingAt     int64     `json:"working_at,omitempty"`
	RootWorkingAt int64     `json:"root_working_at,omitempty"`
	EndedAt       int64     `json:"ended_at,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
	ReplyDigest   string    `json:"reply_digest,omitempty"`
	ReplyBody     string    `json:"reply_body,omitempty"`
	LastSignal    time.Time `json:"last_signal,omitempty"`
	LastQuery     time.Time `json:"last_query,omitempty"`
	Hint          string    `json:"hint,omitempty"`
	HintPending   bool      `json:"hint_pending,omitempty"`
	// Conflict is durable contrary evidence; Waiting is a recoverable evidence gap.
	Conflict string `json:"conflict,omitempty"`
	Waiting  string `json:"waiting,omitempty"`
}

func (p NativeProof) agentName() string {
	if p.Agent == "" {
		return "codex"
	}
	return p.Agent
}
