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
	Closing  bool                         `json:"closing,omitempty"`
	Closed   bool                         `json:"closed"`
}

func (l *Lifecycle) journalPath(state State) (string, string, error) {
	if !filepath.IsAbs(l.setup.RunDirectory) || state.Control == nil {
		return "", "", errors.New("caller control checkpoint directory or original binding missing")
	}
	raw, _ := json.Marshal(struct {
		Project, Execution, Attempt, Generation, Configuration, Workspace string
		Business, Control                                                 b.ProcessIdentity
	}{l.setup.Snapshot.Session.ProjectID, l.setup.Snapshot.Session.ExecutionID, l.setup.Snapshot.Session.AttemptID, l.setup.Snapshot.ResourceGeneration, l.setup.Snapshot.ConfigurationDigest, strings.ToLower(strings.ReplaceAll(l.setup.Snapshot.Workspace.Path, "\\", "/")), state.Process, state.Control.Process})
	binding := digest(raw)
	return filepath.Join(l.setup.RunDirectory, "orca-native-inspections", binding+".json"), binding, nil
}

func (l *Lifecycle) loadControlCheckpoint(state State) (controlCheckpoint, error) {
	path, binding, err := l.journalPath(state)
	j := controlCheckpoint{Version: 3, Binding: binding}
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
	if len(raw) > 2<<20 || b.DecodeStrictJSON(raw, &j, "trailing native control checkpoint") != nil || j.Version != 3 || j.Binding != binding {
		return j, errors.New("native control checkpoint changed or foreign")
	}
	if j.Closed && (!j.Closing || j.Request.ID != "" && j.Response == nil) {
		return j, errors.New("closed control checkpoint has unsettled intent or query")
	}
	if j.Request.ID == "" {
		if j.Response != nil || j.Request != (codex.RemoteControlRequest{}) {
			return j, errors.New("empty control query has payload")
		}
	} else if state.Control.Ready == nil || state.RemoteReadiness == nil || j.Request.Validate(*state.Control.Ready) != nil || j.Request.Session != state.RemoteReadiness.SessionID {
		return j, errors.New("control query identity changed")
	}
	if j.Request.Method == "original-turn" {
		found := false
		for _, turn := range state.Turns {
			if turn.Accepted && (turn.WireFormat == remoteWireCommand || turn.WireFormat == remoteModelWireCommand) && digest([]byte(wirePrompt(&turn))) == j.Request.InputDigest {
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
	if j.Version != 3 || j.Binding != binding {
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
	switch q.Method {
	case "original-turn":
		if r.Workspace != nil || r.Turn == nil || r.Error != "" && (r.Error != "original_turn_unverified" || !r.Turn.InspectionFailed) {
			return errors.New("native turn response changed")
		}
	case "workspace-audit":
		if r.Turn != nil || r.Workspace == nil || r.Error != "" && r.Error != "workspace_audit_unverified" {
			return errors.New("native workspace response changed")
		}
	default:
		return errors.New("native control method invalid")
	}
	return nil
}

// The same native retry ID repairs uncertain delivery to the original actor.
func (l *Lifecycle) dispatchControl(ctx context.Context, state State, q codex.RemoteControlRequest) (codex.RemoteControlResponse, error) {
	var result codex.RemoteControlResponse
	read := func() (bool, error) {
		lines, err := l.controlLines(ctx, state, false)
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
	return l.callControlWithAudit(ctx, state, method, input, nil)
}

func sameWorkspaceQuery(a, b *codex.RemoteWorkspaceAuditQuery) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (l *Lifecycle) callControlWithAudit(ctx context.Context, state State, method, input string, audit *codex.RemoteWorkspaceAuditQuery) (codex.RemoteControlResponse, error) {
	var empty codex.RemoteControlResponse
	if err := l.validateControl(l.setup.Snapshot, state, true); err != nil {
		return empty, err
	}
	j, err := l.loadControlCheckpoint(state)
	if err != nil {
		return empty, err
	}
	if j.Closing || j.Closed {
		return empty, errors.New("original native control already closed")
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
		if j.Request.Method == method && j.Request.InputDigest == input && sameWorkspaceQuery(j.Request.Audit, audit) {
			return r, nil
		}
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
	if j.Request.Method == "workspace-audit" && j.Response != nil && (j.Response.Workspace == nil || !j.Response.Workspace.WritersAbsent) {
		return empty, errors.New("original workspace reader settlement unverified; control proof retained")
	}
	j.Request = codex.RemoteControlRequest{ID: contracts.NewID(), Binding: state.Control.Ready.Binding, Method: method, Session: state.RemoteReadiness.SessionID, InputDigest: input, Audit: audit}
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
