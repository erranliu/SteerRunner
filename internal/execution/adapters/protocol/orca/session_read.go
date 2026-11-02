package orca

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

type SessionReadOptions struct {
	Agent, SessionID, TranscriptPath string
	Limit, MaxPages                  int
}

type SessionBlock struct{ Type, Text string }

type SessionMessage struct {
	ID, Role, Source string
	Timestamp        int64
	Shortened        bool
	Blocks           []SessionBlock
}

type SessionPage struct {
	Messages     []SessionMessage
	HasMore      bool
	BeforeOffset int64
	Raw          json.RawMessage `json:"-"`
}

var ErrSessionReadBudget = errors.New("Orca native session page budget exhausted")

// SessionReader owns one bounded, explicitly targeted connection. Next calls
// are serial; Close is safe during cancellation and never reconnects.
type SessionReader struct {
	ctx      context.Context
	cancel   context.CancelFunc
	conn     stateConnection
	options  SessionReadOptions
	previous int64
	pages    int
	done     bool
	err      error
	once     sync.Once
}

func OpenSessionReader(ctx context.Context, environment, runtime string, options SessionReadOptions) (*SessionReader, error) {
	if runtime == "" || options.Agent == "" || options.SessionID == "" || options.TranscriptPath == "" || options.Limit <= 0 || options.MaxPages <= 0 {
		return nil, errors.New("Orca native session requires an explicit identity and positive read bounds")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	conn, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		cancel()
		return nil, err
	}
	r := &SessionReader{ctx: ctx, cancel: cancel, conn: conn, options: options, previous: -1}
	context.AfterFunc(ctx, r.Close)
	return r, nil
}

func (r *SessionReader) Close() {
	r.once.Do(func() {
		r.cancel()
		_ = r.conn.close()
	})
}

func (r *SessionReader) Next() (SessionPage, error) {
	if r.err != nil {
		return SessionPage{}, r.err
	}
	if err := r.ctx.Err(); err != nil {
		return SessionPage{}, err
	}
	if r.done {
		return SessionPage{}, io.EOF
	}
	fail := func(err error) (SessionPage, error) {
		r.err = err
		r.Close()
		return SessionPage{}, err
	}
	if r.pages >= r.options.MaxPages {
		return fail(ErrSessionReadBudget)
	}
	params := map[string]any{"agent": r.options.Agent, "sessionId": r.options.SessionID, "transcriptPath": r.options.TranscriptPath, "limit": r.options.Limit}
	if r.previous >= 0 {
		params["beforeOffset"] = r.previous
	}
	r.pages++
	raw, err := callState(r.ctx, r.conn, "nativeChat.readSession", params)
	if err != nil {
		if r.ctx.Err() != nil && !errors.Is(err, ErrStateIdentityChanged) {
			err = r.ctx.Err()
		}
		return fail(err)
	}
	var value struct {
		Messages     *[]SessionMessage
		HasMore      *bool
		BeforeOffset *int64
		Error        string
	}
	if json.Unmarshal(raw, &value) != nil || value.Messages == nil || value.HasMore == nil || value.BeforeOffset == nil || value.Error != "" {
		return fail(errors.New("Orca native session page unavailable"))
	}
	if *value.BeforeOffset < 0 || *value.HasMore && (*value.BeforeOffset == 0 || r.previous >= 0 && *value.BeforeOffset >= r.previous) {
		return fail(errors.New("Orca native session pagination did not advance"))
	}
	r.previous, r.done = *value.BeforeOffset, !*value.HasMore
	return SessionPage{Messages: *value.Messages, HasMore: *value.HasMore, BeforeOffset: *value.BeforeOffset, Raw: append(json.RawMessage(nil), raw...)}, nil
}
