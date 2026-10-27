package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Multiline ConPTY pastes can still occupy Codex's composer after Orca's
// combined paste/Enter returned accepted. Keep these two effects separate:
// paste exactly once, observe its draft, then submit exactly once. A draft is
// readiness only; original transcript/main-agent evidence still owns completion.
func (l *Lifecycle) submitNativePaste(ctx context.Context, s b.LifecycleSnapshot, text string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if strings.ContainsAny(text, "\x00\x1b") {
		return nil, errors.New("native paste contains terminal control characters; no input sent")
	}
	read := func() (string, error) {
		if _, err := l.original(ctx, s); err != nil {
			return "", err
		}
		r, err := l.client.MetaCall(ctx, "terminal", "read", "--terminal", s.Ref.External.Terminal, "--limit", "80", "--json")
		var v struct {
			Terminal struct {
				Handle, Source string
				Draft          *string
			}
		}
		if err != nil {
			return "", err
		}
		if r.RuntimeID != s.Ref.External.RuntimeID || r.CallUncertain || r.ErrorCode != "" || json.Unmarshal(r.Payload, &v) != nil || v.Terminal.Handle != s.Ref.External.Terminal || v.Terminal.Source != "screen" {
			return "", errors.New("original native composer screen unavailable; no submit")
		}
		if v.Terminal.Draft == nil {
			return "", nil
		}
		return *v.Terminal.Draft, nil
	}
	draft, err := read()
	if err != nil {
		return nil, err
	}
	if draft != "" {
		return nil, errors.New("original native composer already contains a draft; no input sent")
	}
	send := func(text *string) (native.Result, error) {
		if _, err := l.original(ctx, s); err != nil {
			return native.Result{}, err
		}
		args := []string{"terminal", "send", "--terminal", s.Ref.External.Terminal}
		size := 1
		if text != nil {
			args = append(args, "--text", *text)
			size = len(*text)
		} else {
			args = append(args, "--enter")
		}
		args = append(args, "--json")
		r, err := l.client.MetaCall(ctx, args...)
		var v struct {
			Send struct {
				Handle       string
				Accepted     *bool
				BytesWritten int
			}
		}
		if err != nil {
			return r, err
		}
		if r.RuntimeID != s.Ref.External.RuntimeID || r.CallUncertain || r.ErrorCode != "" || json.Unmarshal(r.Payload, &v) != nil || v.Send.Handle != s.Ref.External.Terminal || v.Send.Accepted == nil || !*v.Send.Accepted || v.Send.BytesWritten != size {
			return r, errors.New("original native input effect unverified; do not resend")
		}
		_, err = l.original(ctx, s)
		return r, err
	}
	payload := "\x1b[200~" + text + "\x1b[201~"
	pasted, err := send(&payload)
	evidence := struct{ Paste, Submit json.RawMessage }{Paste: pasted.Payload}
	receipt := func() json.RawMessage { raw, _ := json.Marshal(evidence); return raw }
	if err != nil {
		return receipt(), err
	}
	expected := fmt.Sprintf("[Pasted Content %d chars]", utf8.RuneCountInString(text))
	stable := 0
	for {
		draft, err = read()
		if err != nil {
			return receipt(), err
		}
		if draft == text || draft == expected {
			stable++
		} else {
			stable = 0
		}
		if stable >= 2 {
			break
		}
		select {
		case <-ctx.Done():
			return receipt(), errors.New("original paste did not settle; no Enter sent and no input resent")
		case <-time.After(250 * time.Millisecond):
		}
	}
	// The original main/provider must still be at the frozen pre-input boundary.
	state, err := l.state(s)
	if err != nil {
		return receipt(), err
	}
	turn := turnFor(&state, command(s))
	if turn == nil {
		return receipt(), errors.New("original paste baseline missing")
	}
	sample, err := l.queryNative(ctx, state.Process, turn.Native.Pane)
	if err != nil {
		return receipt(), err
	}
	current := sample.AgentState
	if current.PaneKey != turn.Native.Pane || current.ProviderSession.ID != turn.Native.Provider || current.AgentType != "codex" || current.MainAgent == nil || current.MainAgent.State != "done" || current.MainAgent.StateStartedAt != turn.Native.BaseRootAt || current.SessionBoundary || current.RestoredUnconfirmed || current.Interrupted {
		return receipt(), errors.New("original native session changed during paste; no Enter sent")
	}
	entered, err := send(nil)
	evidence.Submit = entered.Payload
	return receipt(), err
}
