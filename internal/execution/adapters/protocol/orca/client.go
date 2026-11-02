// Package orca contains the public CLI and runtime query wire contracts. It knows no tasks,
// business steps, agent-private logs or orchestration lifecycle.
package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"time"
)

type Config struct {
	CommandTimeout string
	Output         func(context.Context, time.Duration, ...string) ([]byte, error)
}

const WorktreeTrashDirectory = ".orca-worktree-trash"

type Client struct{ config Config }

func New(config Config) Client { return Client{config: config} }

func (c Client) Raw(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	return c.config.Output(ctx, timeout, args...)
}
func (c Client) MetaCall(ctx context.Context, args ...string) (Result, error) {
	timeout, err := time.ParseDuration(c.config.CommandTimeout)
	if err != nil || timeout <= 0 {
		timeout = 40 * time.Second
	}
	return c.MetaCallWithTimeout(ctx, timeout, args...)
}

func (c Client) MetaCallWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (Result, error) {
	raw, callErr := c.Raw(ctx, timeout, args...)
	if len(bytes.TrimSpace(raw)) == 0 && IsCLIQueueError(callErr) {
		return Result{}, callErr
	}
	var exit *exec.ExitError
	uncertain := callErr != nil && (!errors.As(callErr, &exit) || !exit.Exited())
	result, parseErr := DecodeCLIResult(raw, uncertain)
	return result, errors.Join(callErr, parseErr)
}

type Terminal struct {
	Handle        string          `json:"handle"`
	RuntimeID     string          `json:"-"`
	IncarnationID string          `json:"incarnationId"`
	WorktreeID    string          `json:"worktreeId"`
	WorktreePath  string          `json:"worktreePath"`
	Branch        string          `json:"branch"`
	AgentIdentity string          `json:"agentIdentity"`
	Connected     bool            `json:"connected"`
	Orphaned      bool            `json:"orphaned"`
	Status        string          `json:"status"`
	AgentWait     json.RawMessage `json:"agentWait"`
}

func (t Terminal) Exited() bool { return t.Status == "exited" }

func (c Client) ShowTerminal(ctx context.Context, handle string) (Terminal, error) {
	if handle == "" {
		return Terminal{}, errors.New("missing original Orca terminal")
	}
	r, err := c.MetaCall(ctx, "terminal", "show", "--terminal", handle, "--json")
	if err != nil {
		return Terminal{}, err
	}
	var value struct {
		Terminal Terminal `json:"terminal"`
	}
	if err = json.Unmarshal(r.Payload, &value); err != nil {
		return Terminal{}, err
	}
	value.Terminal.RuntimeID = r.RuntimeID
	return value.Terminal, nil
}
func (c Client) InspectTerminal(ctx context.Context, identity TerminalIdentity) (Terminal, error) {
	t, err := c.ShowTerminal(ctx, identity.Handle)
	if err != nil {
		return t, err
	}
	return t, validateTerminalIdentity(t, identity)
}

type TerminalClosure struct {
	Handle    string `json:"handle"`
	PtyKilled bool   `json:"ptyKilled"`
}
type TerminalCloseIdentityError struct{}

func (*TerminalCloseIdentityError) Error() string { return "original terminal close is unverified" }

// CloseTerminal performs exactly one native close. PTY facts do not establish
// an execution's stop scope or authorize a retry after a lost response.
func (c Client) CloseTerminal(ctx context.Context, handle, runtime string) (Result, TerminalClosure, error) {
	if handle == "" || runtime == "" {
		return Result{}, TerminalClosure{}, errors.New("missing exact terminal close identity")
	}
	r, err := c.MetaCall(ctx, "terminal", "close", "--terminal", handle, "--json")
	if err != nil {
		return r, TerminalClosure{}, err
	}
	var value struct {
		Close TerminalClosure `json:"close"`
	}
	if json.Unmarshal(r.Payload, &value) != nil || r.RuntimeID != runtime || value.Close.Handle != handle {
		return r, TerminalClosure{}, &TerminalCloseIdentityError{}
	}
	return r, value.Close, nil
}
