package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"strings"
)

// This transport checkpoint cannot authorize business input or create resources.
type controlCheckpoint struct {
	Version  int                          `json:"version"`
	Binding  string                       `json:"binding"`
	Request  codex.RemoteControlRequest   `json:"request"`
	Response *codex.RemoteControlResponse `json:"response,omitempty"`
	Closed   bool                         `json:"closed"`
}

func (l *Lifecycle) journalPath(state State) (string, string, error) {
	if !filepath.IsAbs(l.setup.RunDirectory) || state.RemoteReadiness == nil || state.Control == nil || state.Control.Ready == nil {
		return "", "", errors.New("caller control checkpoint directory or original binding missing")
	}
	raw, _ := json.Marshal(struct {
		Project, Execution, Attempt, Generation, Config string
		Process                                         b.ProcessIdentity
		Ready                                           any
		Control                                         ControlTerminal
	}{l.setup.Snapshot.Session.ProjectID, l.setup.Snapshot.Session.ExecutionID, l.setup.Snapshot.Session.AttemptID, l.setup.Snapshot.ResourceGeneration, l.setup.Snapshot.ConfigurationDigest, state.Process, state.RemoteReadiness, *state.Control})
	binding := digest(raw)
	return filepath.Join(l.setup.RunDirectory, "orca-native-inspections", binding+".json"), binding, nil
}

func (l *Lifecycle) loadControlCheckpoint(state State) (controlCheckpoint, error) {
	path, binding, err := l.journalPath(state)
	j := controlCheckpoint{Version: 2, Binding: binding}
	if err != nil {
		return j, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return j, nil
	}
	if err != nil {
		return j, err
	}
	if len(raw) > 2<<20 || b.DecodeStrictJSON(raw, &j, "trailing native control checkpoint") != nil || j.Version != 2 || j.Binding != binding || j.Request.Validate(*state.Control.Ready) != nil || j.Request.Session != state.RemoteReadiness.SessionID {
		return j, errors.New("native control checkpoint changed or foreign")
	}
	if j.Request.Method == "original-turn" {
		found := false
		for _, turn := range state.Turns {
			if turn.Accepted && turn.WireFormat == remoteWireCommand && digest([]byte(wirePrompt(&turn))) == j.Request.InputDigest {
				found = true
			}
		}
		if !found {
			return j, errors.New("native control checkpoint original input missing")
		}
	}
	if j.Response != nil && validateControlResponse(j.Request, *j.Response) != nil {
		return j, errors.New("native control checkpoint response changed")
	}
	return j, nil
}

func (l *Lifecycle) saveControlCheckpoint(state State, j controlCheckpoint) error {
	path, binding, err := l.journalPath(state)
	if err != nil {
		return err
	}
	if j.Version != 2 || j.Binding != binding {
		return errors.New("native control checkpoint parent changed")
	}
	raw, err := json.Marshal(j)
	if err != nil {
		return err
	}
	return l.setup.ProcessHost.WriteExecutionFile(path, raw)
}

func validateControlResponse(q codex.RemoteControlRequest, r codex.RemoteControlResponse) error {
	if r.ID != q.ID || r.Binding != q.Binding || r.RequestDigest != codex.RemoteControlDigest(q) {
		return errors.New("original native control response binding changed")
	}
	if r.Error != "" {
		if r.Authentication != nil || r.Error != "original_turn_unverified" && r.Error != "authentication_stop_unverified" || (q.Method == "original-turn") != (r.Error == "original_turn_unverified") || r.Turn != nil && (q.Method != "original-turn" || !r.Turn.InspectionFailed) {
			return errors.New("native control error invalid")
		}
		return nil
	}
	if q.Method == "original-turn" && (r.Turn == nil || r.Authentication != nil) || q.Method == "authentication-stop" && (r.Authentication == nil || r.Turn != nil) {
		return errors.New("native control response kind changed")
	}
	return nil
}

// The same native retry ID repairs uncertain delivery to the original actor.
func (l *Lifecycle) dispatchControl(ctx context.Context, state State, q codex.RemoteControlRequest) (codex.RemoteControlResponse, error) {
	var result codex.RemoteControlResponse
	read := func() (bool, error) {
		lines, err := l.controlLines(ctx, state)
		if err != nil {
			return false, err
		}
		for _, line := range lines {
			if !strings.HasPrefix(line, codex.RemoteControlPrefix) {
				continue
			}
			raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteControlPrefix))
			var r codex.RemoteControlResponse
			if err != nil || b.DecodeStrictJSON(raw, &r, "trailing native control response") != nil {
				return false, errors.New("invalid native control response")
			}
			if r.ID != q.ID {
				continue
			}
			if err = validateControlResponse(q, r); err != nil {
				return false, err
			}
			result = r
			return true, nil
		}
		return false, nil
	}
	if found, err := read(); found || err != nil {
		return result, err
	}
	raw, _ := json.Marshal(q)
	r, err := l.client.MetaCall(ctx, "terminal", "send", "--terminal", state.Control.Process.Terminal, "--text", base64.RawURLEncoding.EncodeToString(raw), "--enter", "--retry-request", nativeRequest(q.ID), "--json")
	var value struct {
		Send struct {
			Handle   string
			Accepted bool
			Prompt   struct{ RequestID, ProcessIncarnation string }
		}
	}
	if err != nil || r.CallUncertain || r.RuntimeID != state.Control.Process.RuntimeID || json.Unmarshal(r.Payload, &value) != nil || !value.Send.Accepted || value.Send.Handle != state.Control.Process.Terminal || value.Send.Prompt.RequestID != nativeRequest(q.ID) || value.Send.Prompt.ProcessIncarnation != state.Control.Process.Incarnation {
		return result, errors.New("original native control send unresolved; same request retained")
	}
	for {
		if found, err := read(); found || err != nil {
			return result, err
		}
		if err := controlPause(ctx); err != nil {
			return result, err
		}
	}
}

func (l *Lifecycle) callControl(ctx context.Context, state State, method, input string) (codex.RemoteControlResponse, error) {
	var empty codex.RemoteControlResponse
	if err := l.validateControl(l.setup.Snapshot, state, true); err != nil {
		return empty, err
	}
	j, err := l.loadControlCheckpoint(state)
	if err != nil {
		return empty, err
	}
	if j.Closed {
		return empty, errors.New("original native control already closed")
	}
	if method == "original-turn" && j.Request.Method == "authentication-stop" {
		return empty, errors.New("original authentication revocation requested; no later receipt query")
	}
	if j.Request.ID != "" && j.Response == nil {
		r, err := l.dispatchControl(ctx, state, j.Request)
		if err != nil {
			return empty, err
		}
		j.Response = &r
		if err := l.saveControlCheckpoint(state, j); err != nil {
			return empty, err
		}
		if j.Request.Method == method && j.Request.InputDigest == input {
			return r, nil
		}
	}
	if method == "authentication-stop" && j.Response != nil && j.Request.Method == method && j.Response.Error == "" {
		return *j.Response, nil
	}
	// Never overwrite the only proof for a possibly live native reader.
	// A known query failure may advance only after its original tree settled.
	if j.Request.Method == "original-turn" && j.Response != nil {
		var original *Turn
		for i := range state.Turns {
			if digest([]byte(wirePrompt(&state.Turns[i]))) == j.Request.InputDigest {
				original = &state.Turns[i]
				break
			}
		}
		if original == nil || j.Response.Turn == nil || validateReaderExit(state, original, *j.Response.Turn) != nil {
			return empty, errors.New("original native reader settlement unverified; control proof retained")
		}
	}
	j.Request = codex.RemoteControlRequest{ID: contracts.NewID(), Binding: state.Control.Ready.Binding, Method: method, Session: state.RemoteReadiness.SessionID, InputDigest: input}
	j.Response = nil
	if err := l.saveControlCheckpoint(state, j); err != nil {
		return empty, err
	}
	r, err := l.dispatchControl(ctx, state, j.Request)
	if err != nil {
		return empty, err
	}
	j.Response = &r
	if err := l.saveControlCheckpoint(state, j); err != nil {
		return empty, err
	}
	return r, nil
}

func (l *Lifecycle) inspectionGate(ctx context.Context) (func(), error) {
	l.inspectionOnce.Do(func() { l.inspectionSerial = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case l.inspectionSerial <- struct{}{}:
		return func() { <-l.inspectionSerial }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
