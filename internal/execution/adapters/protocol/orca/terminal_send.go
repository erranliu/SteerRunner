package orca

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// TerminalSend describes the text-plus-enter operation used by current callers.
// Text is already compiled by the caller; this layer does not normalize it.
type TerminalSend struct {
	Handle, Text, RequestID string
	WaitSubmitSeconds       uint
}

type TerminalPromptReceipt struct {
	RequestID          string   `json:"requestId"`
	ProcessIncarnation string   `json:"processIncarnation"`
	Stages             []string `json:"stages"`
	Provider           string   `json:"provider"`
	Observation        string   `json:"observation"`
	decoded            bool
}

// Decoded reports whether all optional prompt details have their native types.
// Some callers consume only core identity and never require these extensions.
func (p TerminalPromptReceipt) Decoded() bool { return p.decoded }

type TerminalSendReceipt struct {
	RuntimeID string                `json:"-"`
	Handle    string                `json:"handle"`
	Accepted  *bool                 `json:"accepted"`
	Prompt    TerminalPromptReceipt `json:"prompt"`
	decoded   bool
}

// Matches checks only complete native identity. Acceptance, dispatch uncertainty
// and the meaning of stages remain separate facts for each caller to interpret.
func (r TerminalSendReceipt) Matches(p TerminalIdentity, request string) bool {
	return r.decoded && p.RuntimeID != "" && p.Handle != "" && p.IncarnationID != "" && request != "" &&
		r.RuntimeID == p.RuntimeID && r.Handle == p.Handle && r.Prompt.ProcessIncarnation == p.IncarnationID && r.Prompt.RequestID == request
}

func (s TerminalSend) arguments() []string {
	args := []string{"terminal", "send", "--terminal", s.Handle, "--text", s.Text, "--enter", "--retry-request", s.RequestID, "--json"}
	if s.WaitSubmitSeconds != 0 {
		args = append(args, "--wait-submit", strconv.FormatUint(uint64(s.WaitSubmitSeconds), 10))
	}
	return args
}

func (s TerminalSend) validate() error {
	if s.Handle == "" || s.RequestID == "" {
		return errors.New("Orca terminal send requires an explicit terminal and request")
	}
	return nil
}

func decodeTerminalSend(result Result, callErr error) (Result, TerminalSendReceipt, error) {
	var value struct{ Send TerminalSendReceipt }
	detailErr := json.Unmarshal(result.Payload, &value)
	// Preserve the two existing native field scopes. An extension type error
	// cannot invalidate identity-only readers, and a later duplicate field
	// cannot erase a type error already observed by the full details decoder.
	var core struct {
		Send struct {
			Handle   string
			Accepted *bool
			Prompt   struct{ RequestID, ProcessIncarnation string }
		}
	}
	err := json.Unmarshal(result.Payload, &core)
	value.Send.Handle, value.Send.Accepted = core.Send.Handle, core.Send.Accepted
	value.Send.Prompt.RequestID, value.Send.Prompt.ProcessIncarnation = core.Send.Prompt.RequestID, core.Send.Prompt.ProcessIncarnation
	value.Send.Prompt.decoded = detailErr == nil
	value.Send.decoded, value.Send.RuntimeID = err == nil, result.RuntimeID
	return result, value.Send, errors.Join(callErr, err)
}

func (c Client) SendTerminal(ctx context.Context, send TerminalSend) (Result, TerminalSendReceipt, error) {
	if err := send.validate(); err != nil {
		return Result{}, TerminalSendReceipt{}, err
	}
	return decodeTerminalSend(c.MetaCall(ctx, send.arguments()...))
}

func (c Client) SendTerminalWithTimeout(ctx context.Context, timeout time.Duration, send TerminalSend) (Result, TerminalSendReceipt, error) {
	if err := send.validate(); err != nil {
		return Result{}, TerminalSendReceipt{}, err
	}
	return decodeTerminalSend(c.MetaCallWithTimeout(ctx, timeout, send.arguments()...))
}
