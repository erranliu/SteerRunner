package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
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
type composerViewport interface {
	Restore(context.Context) (json.RawMessage, error)
}

func (l *Lifecycle) submitNativePaste(ctx context.Context, s b.LifecycleSnapshot, text string) (result json.RawMessage, resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	ctx = nativeInputPhase(ctx, "draft-before-paste")
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
		phase := "submit"
		if text != nil {
			phase = "paste"
		}
		ctx := nativeInputPhase(ctx, phase)
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
	evidence := struct {
		Paste, Submit   json.RawMessage
		ViewportFit     json.RawMessage `json:",omitempty"`
		ViewportRestore json.RawMessage `json:",omitempty"`
	}{Paste: pasted.Payload}
	receipt := func() json.RawMessage { raw, _ := json.Marshal(evidence); return raw }
	var viewport composerViewport
	defer func() {
		if viewport == nil {
			return
		}
		restoreCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer stop()
		var restoreErr error
		evidence.ViewportRestore, restoreErr = viewport.Restore(restoreCtx)
		resultErr = errors.Join(resultErr, restoreErr)
		result = receipt()
	}()
	if err != nil {
		return receipt(), err
	}
	ctx = nativeInputPhase(ctx, "draft-settle")
	stable := 0
	previousDraft := ""
	reflowAttempted := false
	for {
		draft, err = read()
		if err != nil {
			return receipt(), err
		}
		if !reflowAttempted && l.fitComposer != nil && nativeDraftNeedsReflow(draft, text) {
			reflowAttempted = true
			if _, err := l.original(ctx, s); err != nil {
				return receipt(), err
			}
			viewport, evidence.ViewportFit, err = l.fitComposer(nativeInputPhase(ctx, "draft-reflow"), s.Ref.External, nativeViewportRecorder(nativeInputPhase(ctx, "draft-reflow")))
			if err != nil {
				return receipt(), err
			}
			if _, err := l.original(ctx, s); err != nil {
				return receipt(), err
			}
			stable, previousDraft = 0, ""
			continue // discard the narrow observation, then require exact stable text
		}
		if nativePasteDraftMatches(draft, text) {
			if draft == previousDraft {
				stable++
			} else {
				stable = 1
			}
		} else {
			stable = 0
		}
		previousDraft = draft
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
	ctx = nativeInputPhase(ctx, "pre-submit-fence")
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

// This only asks for a different view; it NEVER accepts or rewrites whitespace.
// A real LF or other altered byte must still fail nativePasteDraftMatches after
// reflow. In particular the original comparator and all transcript fences stay.
func nativeDraftNeedsReflow(draft, text string) bool {
	if draft == text || !utf8.ValidString(draft) || !utf8.ValidString(text) {
		return false
	}
	d, t := []rune(draft), []rune(text)
	different := false
	i, j := 0, 0
	for i < len(d) && j < len(t) {
		if d[i] == t[j] {
			i++
			j++
			continue
		}
		if d[i] != '\n' {
			return false
		}
		// The renderer can replace a word separator or insert a row break
		// inside a word. This only permits requesting a fresh view. Neither
		// form is accepted by the unchanged exact comparator below.
		i++
		if t[j] == ' ' {
			j++
		}
		different = true
	}
	return different && i == len(d) && j == len(t)
}

// A single ConPTY write may become several Codex paste elements, with literal
// fragments between them. Match the complete rendered draft against the frozen
// input's scalar positions; collapsed text remains readiness, never final proof.
var nativePasteLabel = regexp.MustCompile(`^\[Pasted Content ([1-9][0-9]*) chars\](?: #([1-9][0-9]*))?`)

func nativePasteDraftMatches(draft, text string) bool {
	if !utf8.ValidString(draft) || !utf8.ValidString(text) {
		return false
	}
	if draft == text {
		return true
	}
	occurrences := map[int]int{}
	for draft != "" {
		if label := nativePasteLabel.FindStringSubmatch(draft); label != nil {
			count, err := strconv.Atoi(label[1])
			if err != nil || count > utf8.RuneCountInString(text) {
				return false
			}
			occurrences[count]++
			ordinal := 1
			if label[2] != "" {
				ordinal, err = strconv.Atoi(label[2])
			}
			if err != nil || ordinal != occurrences[count] || (label[2] != "" && ordinal < 2) {
				return false
			}
			for i := 0; i < count; i++ {
				_, size := utf8.DecodeRuneInString(text)
				text = text[size:]
			}
			draft = draft[len(label[0]):]
			continue
		}
		if text == "" {
			return false
		}
		dr, ds := utf8.DecodeRuneInString(draft)
		tr, ts := utf8.DecodeRuneInString(text)
		if dr != tr {
			return false
		}
		draft, text = draft[ds:], text[ts:]
	}
	return text == ""
}
