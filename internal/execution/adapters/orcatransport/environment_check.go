package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"strings"
	"time"
)

type RemoteEnvironmentSession struct {
	setup           codex.RemoteCheckSetup
	settings        contracts.OrcaSettings
	client          native.Client
	original        b.ProcessIdentity
	endpoint        string
	sequence        int
	removeDirectory bool
	removed         bool
	actorReady      bool
	startupFailure  string
}

func NewRemoteEnvironmentSession(setup codex.RemoteCheckSetup, endpoint string) (*RemoteEnvironmentSession, error) {
	var settings contracts.OrcaSettings
	if contracts.DecodeStrict(setup.Input.Execution.Settings, &settings) != nil || settings.AgentLaunch.RuntimeExecutable == "" {
		return nil, errors.New("remote_environment_runtime_unsupported")
	}
	if settings.Transport.Executable == "" {
		settings.Transport.Executable = "orca.exe"
	}
	s := &RemoteEnvironmentSession{setup: setup, settings: settings, endpoint: endpoint}
	if endpoint != "" {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(endpoint, "orca-check:"))
		if !strings.HasPrefix(endpoint, "orca-check:") || err != nil || json.Unmarshal(raw, &s.original) != nil || s.original.RuntimeID == "" || s.original.Incarnation == "" || s.original.Terminal == "" {
			return nil, errors.New("remote check original binding invalid")
		}
	}
	host := process.New(40 * time.Second)
	s.client = native.New(native.Config{CommandTimeout: "40s", Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		all := append(append([]string(nil), settings.Transport.Args...), args...)
		if setup.Input.Execution.Target.Environment != "" {
			all = append(all, "--environment", setup.Input.Execution.Target.Environment)
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return native.SerializeCLI(callCtx, func(ctx context.Context) ([]byte, error) {
			return host.OutputCommand(ctx, b.LaunchCommand{Executable: settings.Transport.Executable, Args: all, Timeout: timeout, BlockedEnvironment: []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}})
		})
	}})
	return s, nil
}

func (s *RemoteEnvironmentSession) Initialize(ctx context.Context) (codex.Binding, error) {
	if s.endpoint != "" {
		_, err := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original)
		return s.binding(), err
	}
	r, err := s.client.MetaCall(ctx, "repo", "list", "--json")
	if err != nil {
		return codex.Binding{}, err
	}
	var repos struct {
		Repos []struct {
			ID, Path, Kind    string
			GitRemoteIdentity struct {
				RemoteURL string `json:"remoteUrl"`
			}
		}
		Truncated bool
	}
	if json.Unmarshal(r.Payload, &repos) != nil || repos.Truncated || r.RuntimeID == "" {
		return codex.Binding{}, errors.New("remote check repository inventory incomplete")
	}
	var id string
	for _, repo := range repos.Repos {
		if repo.Kind == "git" && repo.GitRemoteIdentity.RemoteURL == s.setup.Input.Repository.URL {
			if id != "" {
				return codex.Binding{}, errors.New("ambiguous remote check repository")
			}
			id = repo.ID
			s.setup.SourcePath = repo.Path
		}
	}
	if id == "" {
		return codex.Binding{}, errors.New("remote check registered repository unavailable")
	}
	runtime := r.RuntimeID
	r, err = s.client.MetaCall(ctx, "worktree", "show", "--worktree", "path:"+s.setup.SourcePath, "--json")
	if err != nil {
		return codex.Binding{}, err
	}
	var trees struct {
		Worktree struct {
			ID, Path       string
			IsMainWorktree bool
		}
	}
	if json.Unmarshal(r.Payload, &trees) != nil || r.RuntimeID != runtime {
		return codex.Binding{}, errors.New("remote check source worktree inventory incomplete")
	}
	var source string
	for _, tree := range []struct {
		ID, Path       string
		IsMainWorktree bool
	}{trees.Worktree} {
		if tree.IsMainWorktree && sameRemotePath(tree.Path, s.setup.SourcePath) {
			source = tree.ID
		}
	}
	if source == "" {
		return codex.Binding{}, errors.New("remote check main source unavailable")
	}
	raw, _ := json.Marshal(s.setup)
	command := "& '" + strings.ReplaceAll(s.settings.AgentLaunch.RuntimeExecutable, "'", "''") + "' '--environment-session' '" + base64.RawURLEncoding.EncodeToString(raw) + "'"
	r, err = s.client.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+source, "--title", "SteerRunner environment check "+s.setup.CheckID, "--command", command, "--json")
	if err != nil {
		return codex.Binding{}, err
	}
	var result struct{ Terminal native.Terminal }
	if json.Unmarshal(r.Payload, &result) != nil || r.RuntimeID != runtime || result.Terminal.Handle == "" || result.Terminal.IncarnationID == "" || result.Terminal.WorktreeID != source {
		return codex.Binding{}, errors.New("remote check actor creation unverified")
	}
	s.original = b.ProcessIdentity{RuntimeID: runtime, Terminal: result.Terminal.Handle, Incarnation: result.Terminal.IncarnationID, WorktreeID: source}
	encoded, _ := json.Marshal(s.original)
	s.endpoint = "orca-check:" + base64.RawURLEncoding.EncodeToString(encoded)
	return s.binding(), nil
}
func sameRemotePath(a, b string) bool {
	return strings.EqualFold(strings.ReplaceAll(a, "\\", "/"), strings.ReplaceAll(b, "\\", "/"))
}
func (s *RemoteEnvironmentSession) binding() codex.Binding {
	p := s.original
	return codex.Binding{Process: &p, Endpoint: s.endpoint, Generation: p.Incarnation}
}

func (s *RemoteEnvironmentSession) read(ctx context.Context) ([]string, error) {
	if _, err := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); err != nil {
		return nil, err
	}
	return s.readOutput(ctx)
}

// Poll only output while an original request is pending. Its response is not
// accepted until the original incarnation is checked again below.
func (s *RemoteEnvironmentSession) readOutput(ctx context.Context) ([]string, error) {
	r, err := s.client.MetaCall(ctx, "terminal", "read", "--terminal", s.original.Terminal, "--limit", "1000", "--json")
	if err != nil {
		return nil, err
	}
	var value struct {
		Terminal struct {
			Handle string
			Tail   []string
		}
	}
	if json.Unmarshal(r.Payload, &value) != nil || r.RuntimeID != s.original.RuntimeID || value.Terminal.Handle != s.original.Terminal {
		return nil, errors.New("remote check read identity mismatch")
	}
	return value.Terminal.Tail, nil
}

func (s *RemoteEnvironmentSession) call(ctx context.Context, request codex.RemoteCheckRequest) (codex.RemoteCheckResponse, error) {
	var response codex.RemoteCheckResponse
	if s.startupFailure != "" {
		return response, errors.New(s.startupFailure)
	}
	// The native operation is never resent after an ambiguous acceptance. The
	// durable terminal request and finite actor response remain for settlement.
	if request.Method == "next" {
		s.sequence++
		request.ID = digest([]byte(fmt.Sprintf("%s:next:%d", s.setup.CheckID, s.sequence)))[:32]
	} else {
		raw, _ := json.Marshal(request)
		request.ID = digest(append([]byte(s.setup.CheckID), raw...))[:32]
	}
	for !s.actorReady {
		lines, err := s.read(ctx)
		if err != nil {
			return response, err
		}
		ready := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, codex.RemoteCheckStartupFailurePrefix) {
				code := strings.TrimPrefix(line, codex.RemoteCheckStartupFailurePrefix)
				if codex.RemoteCheckStartupDiagnostic(errors.New(code)) != "" {
					s.startupFailure = code
					return response, errors.New(code)
				}
			}
			if strings.TrimSpace(line) == "SR_REMOTE_CHECK_ACTOR_READY" {
				ready = true
			}
		}
		if ready {
			s.actorReady = true
			break
		}
		select {
		case <-ctx.Done():
			return response, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if _, err := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); err != nil {
		return response, err
	}
	raw, _ := json.Marshal(request)
	// This terminal hosts a machine actor, not a model TUI. Send its complete
	// line as raw text so Orca does not wait for an unrelated model turn. The
	// actor deduplicates the request ID and payload; a lost receipt is unknown,
	// never an instruction to resend native work.
	payload := base64.RawURLEncoding.EncodeToString(raw) + "\r"
	r, err := s.client.MetaCall(ctx, "terminal", "send", "--terminal", s.original.Terminal, "--text", payload, "--json")
	var sent struct {
		Send struct {
			Handle       string
			Accepted     bool
			BytesWritten int
		}
	}
	if err != nil || r.CallUncertain || json.Unmarshal(r.Payload, &sent) != nil || r.RuntimeID != s.original.RuntimeID || sent.Send.Handle != s.original.Terminal || !sent.Send.Accepted || sent.Send.BytesWritten != len(payload) {
		return response, errors.New("remote check dispatch outcome unknown")
	}
	for {
		lines, err := s.readOutput(ctx)
		if err != nil {
			return response, err
		}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, codex.RemoteCheckPrefix) {
				continue
			}
			raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteCheckPrefix))
			var value codex.RemoteCheckResponse
			if e == nil && json.Unmarshal(raw, &value) == nil && value.ID == request.ID {
				if _, identityErr := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); identityErr != nil {
					return response, identityErr
				}
				if value.Error != "" {
					return value, errors.New(value.Error)
				}
				return value, nil
			}
		}
		select {
		case <-ctx.Done():
			return response, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
func (s *RemoteEnvironmentSession) Prepare(ctx context.Context) (codex.Binding, error) {
	r, err := s.call(ctx, codex.RemoteCheckRequest{Method: "prepare"})
	binding := r.Binding
	if binding.Process == nil {
		binding = s.binding()
	} else {
		p := *binding.Process
		p.RuntimeID = s.original.RuntimeID
		p.Terminal = s.original.Terminal
		p.Incarnation = s.original.Incarnation
		p.WorktreeID = s.original.WorktreeID
		binding.Process = &p
		binding.Endpoint = s.endpoint
	}
	return binding, err
}
func (s *RemoteEnvironmentSession) Start(ctx context.Context) (string, error) {
	r, e := s.call(ctx, codex.RemoteCheckRequest{Method: "start"})
	return r.Turn, e
}
func (s *RemoteEnvironmentSession) Next(ctx context.Context) (codex.EnvironmentCall, error) {
	r, e := s.call(ctx, codex.RemoteCheckRequest{Method: "next"})
	return r.Call, e
}
func (s *RemoteEnvironmentSession) Execute(ctx context.Context, a string, r contracts.CheckResources) (map[string]string, error) {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "execute", Action: a, Resources: r})
	return v.Facts, e
}
func (s *RemoteEnvironmentSession) Verify(ctx context.Context, a string, r contracts.CheckResources) (map[string]string, error) {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "verify", Action: a, Resources: r})
	return v.Facts, e
}
func (s *RemoteEnvironmentSession) Reply(c codex.EnvironmentCall, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, e := s.call(ctx, codex.RemoteCheckRequest{Method: "reply", Call: c, Status: status})
	return e
}
func (s *RemoteEnvironmentSession) Cleanup(ctx context.Context, a string, r contracts.CheckResources) (contracts.CheckResources, error) {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "cleanup", Action: a, Resources: r})
	if e != nil && v.Resources.BaseRef == "" {
		return r, e
	}
	return v.Resources, e
}
func (s *RemoteEnvironmentSession) SetCleanupScope(remove bool) { s.removeDirectory = remove }
func (s *RemoteEnvironmentSession) LocalRemoved() bool          { return s.removed }
func (s *RemoteEnvironmentSession) Stop(ctx context.Context) bool {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "stop", RemoveDirectory: s.removeDirectory})
	if s.startupFailure != "" && !s.actorReady {
		// This finite marker is emitted only before the actor accepts requests
		// or creates native/check resources. Close its exact original PTY.
		if _, err := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); err != nil {
			return false
		}
		proof, err := s.client.CloseTerminalChecked(ctx, s.original.Terminal, s.original.RuntimeID)
		s.removed = err == nil && proof.Disposition == "confirmed"
		return s.removed
	}
	s.removed = v.LocalRemoved
	if e != nil || !v.Stopped {
		return false
	}
	if _, e = s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); e != nil {
		return false
	}
	receipt, e := s.client.CloseTerminalChecked(ctx, s.original.Terminal, s.original.RuntimeID)
	return e == nil && receipt.Disposition == "confirmed"
}
