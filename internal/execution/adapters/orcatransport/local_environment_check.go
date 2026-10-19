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

type LocalTerminalEnvironmentSession struct {
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

func NewLocalTerminalEnvironmentSession(setup codex.RemoteCheckSetup, endpoint string) (*LocalTerminalEnvironmentSession, error) {
	if setup.Input.Execution.Target.Kind != "local" || setup.Input.Execution.Target.Environment != "" {
		return nil, errors.New("remote_environment_check_unsupported: no remote helper actor")
	}
	var settings contracts.OrcaSettings
	if contracts.DecodeStrict(setup.Input.Execution.Settings, &settings) != nil || settings.AgentLaunch.RuntimeExecutable == "" {
		return nil, errors.New("remote_environment_runtime_unsupported")
	}
	if settings.Transport.Executable == "" {
		settings.Transport.Executable = "orca.exe"
	}
	s := &LocalTerminalEnvironmentSession{setup: setup, settings: settings, endpoint: endpoint}
	if endpoint != "" {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(endpoint, "orca-check:"))
		if !strings.HasPrefix(endpoint, "orca-check:") || err != nil || json.Unmarshal(raw, &s.original) != nil || s.original.RuntimeID == "" || s.original.Incarnation == "" || s.original.Terminal == "" {
			return nil, errors.New("remote check original binding invalid")
		}
	}
	host := process.New(40 * time.Second)
	s.client = native.New(native.Config{CommandTimeout: "40s", Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		all := append(append([]string(nil), settings.Transport.Args...), args...)
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		out, err := native.SerializeCLI(callCtx, func(ctx context.Context) ([]byte, error) {
			return host.OutputCommand(ctx, b.LaunchCommand{Executable: settings.Transport.Executable, Args: all, Timeout: timeout, BlockedEnvironment: []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}})
		})
		if native.IsCLIQueueError(err) {
			return nil, fmt.Errorf("native_command_queue_unavailable: %w", err)
		}
		if err != nil {
			return nil, errors.New("native_cli_request_unverified")
		}
		return out, nil
	}})
	return s, nil
}

func (s *LocalTerminalEnvironmentSession) Initialize(ctx context.Context) (codex.Binding, error) {
	if s.setup.Input.Execution.Target.Kind != "local" || s.setup.Input.Execution.Target.Environment != "" {
		return codex.Binding{}, errors.New("remote_environment_check_unsupported")
	}
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
		if repo.Kind == "git" && repo.GitRemoteIdentity.RemoteURL == s.setup.Input.Repository.URL &&
			sameRemotePath(repo.Path, s.setup.Input.Repository.LocalPath) {
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
	return strings.EqualFold(strings.TrimRight(strings.ReplaceAll(a, "\\", "/"), "/"), strings.TrimRight(strings.ReplaceAll(b, "\\", "/"), "/"))
}
func (s *LocalTerminalEnvironmentSession) binding() codex.Binding {
	p := s.original
	return codex.Binding{Process: &p, Endpoint: s.endpoint, Generation: p.Incarnation}
}

func (s *LocalTerminalEnvironmentSession) read(ctx context.Context) ([]string, error) {
	if _, err := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); err != nil {
		return nil, err
	}
	return s.readOutput(ctx)
}

// Poll only output while an original request is pending. Its response is not
// accepted until the original incarnation is checked again below.
func (s *LocalTerminalEnvironmentSession) readOutput(ctx context.Context) ([]string, error) {
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

func (s *LocalTerminalEnvironmentSession) call(ctx context.Context, request codex.RemoteCheckRequest) (codex.RemoteCheckResponse, error) {
	var response codex.RemoteCheckResponse
	// Fixed execute/owner-query controls include a native RPC capped at 30s.
	// Queue and observation consume one bounded 40s budget, never the entire
	// model execution deadline. A timeout remains unknown and never resends.
	if request.Method == "execute" || request.Method == "verify" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
	}
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
			if retryCheckObservation(ctx, err) {
				continue
			}
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
	for {
		if _, err := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); err != nil {
			if retryCheckObservation(ctx, err) {
				continue
			}
			return response, err
		}
		break
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
	decoded := json.Unmarshal(r.Payload, &sent) == nil
	if r.RuntimeID != "" && r.RuntimeID != s.original.RuntimeID || decoded && sent.Send.Handle != "" && sent.Send.Handle != s.original.Terminal {
		return response, errors.New("remote check dispatch identity mismatch")
	}
	if err == nil && !r.CallUncertain && (!decoded || r.RuntimeID == "" || sent.Send.Handle != s.original.Terminal || !sent.Send.Accepted || sent.Send.BytesWritten != len(payload)) {
		return response, errors.New("remote check dispatch outcome unknown")
	}
	// A lost receipt is not a reason to send the machine request again. Only
	// the original actor's cached response can settle that exact request ID.
	for {
		lines, err := s.readOutput(ctx)
		if err != nil {
			if retryCheckObservation(ctx, err) {
				continue
			}
			return response, fmt.Errorf("remote_check_response_unverified: %w", err)
		}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Retained PTY stream rows can include an earlier input echo. Only
			// a complete frame for this request and the original actor is used.
			if marker := strings.Index(line, codex.RemoteCheckPrefix); marker >= 0 {
				line = line[marker:]
			}
			if !strings.HasPrefix(line, codex.RemoteCheckPrefix) {
				continue
			}
			raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteCheckPrefix))
			var value codex.RemoteCheckResponse
			if e == nil && json.Unmarshal(raw, &value) == nil && value.ID == request.ID {
				// Retain the original matching response while retrying only
				// its original identity observation. A stream need not repeat
				// an already observed frame after a transient show failure.
				for {
					if _, identityErr := s.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, s.original); identityErr != nil {
						if retryCheckObservation(ctx, identityErr) {
							continue
						}
						return response, fmt.Errorf("remote_check_identity_unverified: %w", identityErr)
					}
					break
				}
				if value.Error != "" {
					return value, errors.New(value.Error)
				}
				return value, nil
			}
		}
		select {
		case <-ctx.Done():
			return response, fmt.Errorf("remote_check_response_missing: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}
func (s *LocalTerminalEnvironmentSession) Prepare(ctx context.Context) (codex.Binding, error) {
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
func (s *LocalTerminalEnvironmentSession) Start(ctx context.Context) (string, error) {
	r, e := s.call(ctx, codex.RemoteCheckRequest{Method: "start"})
	if e != nil && r.InputNotSent && r.Turn == "" {
		e = &codex.EnvironmentInputRejectedError{Reason: codex.EnvironmentDiagnostic(e)}
	}
	return r.Turn, e
}
func (s *LocalTerminalEnvironmentSession) Next(ctx context.Context) (codex.EnvironmentCall, error) {
	r, e := s.call(ctx, codex.RemoteCheckRequest{Method: "next"})
	return r.Call, e
}
func (s *LocalTerminalEnvironmentSession) Execute(ctx context.Context, a string, r contracts.CheckResources) (map[string]string, error) {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "execute", Action: a, Resources: r})
	return v.Facts, e
}
func (s *LocalTerminalEnvironmentSession) Verify(ctx context.Context, a string, r contracts.CheckResources) (map[string]string, error) {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "verify", Action: a, Resources: r})
	return v.Facts, e
}
func (s *LocalTerminalEnvironmentSession) Reply(c codex.EnvironmentCall, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, e := s.call(ctx, codex.RemoteCheckRequest{Method: "reply", Call: c, Status: status})
	return e
}
func (s *LocalTerminalEnvironmentSession) Cleanup(ctx context.Context, a string, r contracts.CheckResources) (contracts.CheckResources, error) {
	v, e := s.call(ctx, codex.RemoteCheckRequest{Method: "cleanup", Action: a, Resources: r})
	if e != nil && v.Resources.BaseRef == "" {
		return r, e
	}
	return v.Resources, e
}
func (s *LocalTerminalEnvironmentSession) SetCleanupScope(remove bool) { s.removeDirectory = remove }
func (s *LocalTerminalEnvironmentSession) LocalRemoved() bool          { return s.removed }
func (s *LocalTerminalEnvironmentSession) Stop(ctx context.Context) bool {
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

func retryCheckObservation(ctx context.Context, err error) bool {
	if ctx.Err() != nil || !(strings.Contains(err.Error(), "native_cli_request_unverified") || strings.Contains(err.Error(), "native_command_queue_unavailable")) {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(200 * time.Millisecond):
		return true
	}
}
