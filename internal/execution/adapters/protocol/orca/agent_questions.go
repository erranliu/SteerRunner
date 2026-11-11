package orca

import (
	"context"
	"encoding/json"
	"errors"
)

type AgentQuestionAnswer struct {
	QuestionID string   `json:"questionId"`
	OptionIDs  []string `json:"optionIds"`
	Other      *string  `json:"other,omitempty"`
}

type AgentQuestionReply struct {
	Envelope         AgentMutationEnvelope `json:"envelope"`
	ItemID           string                `json:"itemId"`
	ExpectedRevision int64                 `json:"expectedRevision"`
	Answers          []AgentQuestionAnswer `json:"answers"`
}

func (r AgentQuestionReply) Fingerprint() (string, error) {
	// The native host fingerprints its prompt mutation plan, whose name
	// differs from the public RPC name.
	return AgentFingerprint("agentSession.respondTo:question", r.Envelope.SessionID, map[string]any{
		"itemId": r.ItemID, "expectedRevision": r.ExpectedRevision, "answers": r.Answers,
	})
}

func (c AgentSessionClient) RespondToQuestion(ctx context.Context, r AgentQuestionReply) (json.RawMessage, error) {
	if r.Envelope.SessionID == "" || !agentSendOperationID.MatchString(r.Envelope.ClientOperationID) ||
		r.Envelope.ExpectedRuntimeFence == nil || *r.Envelope.ExpectedRuntimeFence <= 0 ||
		r.ItemID == "" || r.ExpectedRevision <= 0 || len(r.Answers) == 0 || len(r.Answers) > 4 {
		return nil, errors.New("incomplete native question reply")
	}
	fingerprint, err := r.Fingerprint()
	if err != nil || fingerprint != r.Envelope.PayloadFingerprint {
		return nil, errors.New("structured question reply fingerprint mismatch")
	}
	raw, err := c.call(ctx, "agentSession.respondToQuestion", r, true)
	if err != nil {
		return raw, err
	}
	return raw, agentDomain(raw)
}
