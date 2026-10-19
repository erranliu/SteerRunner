package codexworker

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RemoteCheckSetup struct {
	ExecutionDeadline time.Time            `json:"execution_deadline,omitempty"`
	CleanupDeadline   time.Time            `json:"cleanup_deadline,omitempty"`
	CheckID           string               `json:"check_id"`
	Nonce             string               `json:"nonce"`
	SourcePath        string               `json:"source_path"`
	Input             contracts.CheckInput `json:"input"`
}
type RemoteCheckRequest struct {
	ID              string                   `json:"id"`
	Method          string                   `json:"method"`
	Action          string                   `json:"action,omitempty"`
	Resources       contracts.CheckResources `json:"resources"`
	Call            EnvironmentCall          `json:"call"`
	Status          string                   `json:"status,omitempty"`
	RemoveDirectory bool                     `json:"remove_directory,omitempty"`
}
type RemoteCheckResponse struct {
	ID           string                   `json:"id"`
	Error        string                   `json:"error,omitempty"`
	Binding      Binding                  `json:"binding"`
	Turn         string                   `json:"turn,omitempty"`
	Call         EnvironmentCall          `json:"call"`
	Facts        map[string]string        `json:"facts,omitempty"`
	Resources    contracts.CheckResources `json:"resources"`
	Stopped      bool                     `json:"stopped"`
	LocalRemoved bool                     `json:"local_removed"`
}

const RemoteCheckPrefix = "SR_REMOTE_CHECK:"
const RemoteCheckStartupFailurePrefix = "SR_REMOTE_CHECK_START_FAILED:"

func RemoteCheckStartupDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	switch err.Error() {
	case "remote_check_configuration_invalid", "remote_check_identity_invalid", "remote_check_settings_invalid", "remote_runtime_build_mismatch", "remote_runtime_executable_unavailable":
		return err.Error()
	}
	return ""
}

// ServeRemoteCheck keeps the existing finite environment-check session and
// its one-use write gate on the execution host. The caller receives only
// bound native facts. A lost response is read/replayed from the actor cache;
// no native operation is dispatched a second time for the same request ID.
func ServeRemoteCheck(ctx context.Context, setup RemoteCheckSetup, input io.Reader, output io.Writer) error {
	if !b.StableExecutionID(setup.CheckID) || !b.StableExecutionID(setup.Nonce) || !filepath.IsAbs(setup.SourcePath) {
		return errors.New("remote_check_identity_invalid")
	}
	var settings contracts.OrcaSettings
	if contracts.DecodeStrict(setup.Input.Execution.Settings, &settings) != nil {
		return errors.New("remote_check_settings_invalid")
	}
	if settings.AgentLaunch.RuntimeSHA256 != "" {
		actual, e := RuntimeBinarySHA256()
		if e != nil || actual != settings.AgentLaunch.RuntimeSHA256 {
			return errors.New("remote_runtime_build_mismatch")
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return errors.New("remote_runtime_executable_unavailable")
	}
	root := filepath.Join(filepath.Dir(executable), "checks", setup.CheckID[:12])
	var session *EnvironmentSession
	owned := false
	defer func() {
		if session != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			session.Stop(stopCtx)
		}
	}()
	cache := map[string]struct {
		digest   string
		response RemoteCheckResponse
	}{}
	fmt.Fprintln(output, "SR_REMOTE_CHECK_ACTOR_READY")
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var request RemoteCheckRequest
		line := strings.TrimSpace(scanner.Text())
		raw, err := base64.RawURLEncoding.DecodeString(line)
		if err != nil || contracts.DecodeStrict(raw, &request) != nil || !b.StableExecutionID(request.ID) {
			return errors.New("invalid remote check request")
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(raw))
		response := RemoteCheckResponse{ID: request.ID}
		if old, ok := cache[request.ID]; ok {
			if old.digest != digest {
				return errors.New("remote check request ID conflict")
			}
			response = old.response
		} else {
			budget := 40 * time.Second
			if request.Method == "prepare" {
				budget = 100 * time.Second
			}
			callCtx, cancel := context.WithTimeout(ctx, budget)
			if !setup.ExecutionDeadline.IsZero() && request.Method != "cleanup" && request.Method != "stop" && request.Method != "verify" {
				cancel()
				callCtx, cancel = context.WithDeadline(ctx, setup.ExecutionDeadline)
			}
			if !setup.CleanupDeadline.IsZero() && (request.Method == "cleanup" || request.Method == "stop") {
				cancel()
				callCtx, cancel = context.WithDeadline(ctx, setup.CleanupDeadline)
			}
			err = nil
			stage := ""
			if callCtx.Err() != nil {
				response.Error = "io_timeout_unverified"
				cancel()
				cache[request.ID] = struct {
					digest   string
					response RemoteCheckResponse
				}{digest, response}
				body, _ := json.Marshal(response)
				fmt.Fprintln(output, RemoteCheckPrefix+base64.RawURLEncoding.EncodeToString(body))
				continue
			}
			switch request.Method {
			case "prepare":
				stage = "remote_check_ownership_failed"
				if session != nil {
					err = errors.New("original remote check already prepared")
					break
				}
				if err = os.MkdirAll(filepath.Dir(root), 0700); err != nil {
					break
				}
				if err = os.Mkdir(root, 0700); err != nil {
					break
				}
				owned = true
				if err = os.WriteFile(filepath.Join(root, ".check-owner"), []byte(setup.CheckID+"\n"+setup.Nonce+"\n"), 0600); err != nil {
					break
				}
				host := process.New(30 * time.Second)
				git := workspace.Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 30 * time.Second, Start: host.StartManaged}
				repoPath := filepath.Join(root, "r")
				wd := filepath.Join(root, "w")
				remote, readErr := git.Output(callCtx, setup.SourcePath, "remote", "get-url", setup.Input.Repository.Remote)
				stage = "remote_check_source_identity_failed"
				if readErr != nil || strings.TrimSpace(string(remote)) != setup.Input.Repository.URL {
					err = errors.New("remote source repository identity changed")
					break
				}
				stage = "remote_check_clone_failed"
				if err = workspace.CloneIsolated(callCtx, git, root, setup.SourcePath, repoPath, setup.Input.Repository.Remote, setup.Input.Repository.BaseRevision); err != nil {
					break
				}
				stage = "remote_check_origin_failed"
				if _, err = git.Output(callCtx, repoPath, "remote", "set-url", setup.Input.Repository.Remote, setup.Input.Repository.URL); err != nil {
					break
				}
				stage = "remote_check_repository_failed"
				repo, resolveErr := workspace.ResolveRepository(callCtx, git, repoPath)
				if resolveErr != nil {
					err = resolveErr
					break
				}
				intent := b.WorkspaceIntent{Path: wd, Branch: "steerline-check/" + setup.CheckID + "/head", BaseCommit: setup.Input.Repository.BaseRevision}
				stage = "remote_check_worktree_failed"
				if _, err = workspace.Prepare(callCtx, git, repo, root, &intent); err != nil {
					break
				}
				stage = "remote_check_git_roots_failed"
				resources, resolveErr := workspace.ResolveLinkedResources(callCtx, git, repo, &intent)
				if resolveErr != nil {
					err = resolveErr
					break
				}
				args, _ := LinkedWorktreeWriteArgs(resources.Objects, resources.Refs, resources.Logs, resources.Admin)
				localInput := setup.Input
				localInput.Repository.LocalPath = ""
				localInput.Execution = contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}}
				localInput.Execution.Settings = encode(contracts.CodexSettings{AccessMode: settings.AccessMode, Executable: settings.AgentLaunch.Executable, Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort})
				stage = "remote_check_configuration_failed"
				session, err = NewEnvironmentSession(EnvironmentSetup{CheckID: setup.CheckID, Nonce: setup.Nonce, Directory: root, Workspace: wd, Input: localInput, LinkedWriteArgs: args})
				if err != nil {
					break
				}
				stage = "remote_check_native_session_failed"
				response.Binding, err = session.Prepare(callCtx)
				response.Binding.Workspace = wd
			case "start":
				if session == nil {
					err = errors.New("remote session missing")
				} else {
					response.Turn, err = session.Start(callCtx)
				}
			case "next":
				if session == nil {
					err = errors.New("remote session missing")
				} else {
					response.Call, err = session.Next(callCtx)
				}
			case "execute":
				if session == nil {
					err = errors.New("remote session missing")
				} else {
					response.Facts, err = session.Execute(callCtx, request.Action, request.Resources)
				}
			case "verify":
				if session == nil {
					err = errors.New("remote session missing")
				} else {
					response.Facts, err = session.Verify(callCtx, request.Action, request.Resources)
				}
			case "reply":
				if session == nil {
					err = errors.New("remote session missing")
				} else {
					err = session.Reply(request.Call, request.Status)
				}
			case "cleanup":
				if session == nil {
					err = errors.New("remote session missing")
				} else {
					response.Resources, err = session.Cleanup(callCtx, request.Action, request.Resources)
				}
			case "stop":
				response.Stopped = session == nil || session.Stop(callCtx)
				if response.Stopped && !owned {
					_, statErr := os.Stat(root)
					response.LocalRemoved = errors.Is(statErr, os.ErrNotExist)
				}
				if response.Stopped && request.RemoveDirectory && owned {
					owner, e := os.ReadFile(filepath.Join(root, ".check-owner"))
					real, re := filepath.EvalSymlinks(root)
					if e == nil && re == nil && strings.EqualFold(real, root) && string(owner) == setup.CheckID+"\n"+setup.Nonce+"\n" {
						response.LocalRemoved = os.RemoveAll(root) == nil
					}
				}
			default:
				err = errors.New("unsupported remote check operation")
			}
			cancel()
			if err != nil {
				response.Error = EnvironmentDiagnostic(err)
				if response.Error == "preparation_unverified" && stage != "" {
					response.Error = stage
				}
				for _, code := range []string{"native_probe_failed", "native_probe_rejected", "native_probe_outcome_unknown", "native_probe_evidence_invalid", "native_tool_identity_mismatch", "native_tool_nonce_mismatch", "original_native_turn_unsettled", "unexpected_native_approval_or_tool_request"} {
					if err.Error() == code {
						response.Error = code
						break
					}
				}
			}
			cache[request.ID] = struct {
				digest   string
				response RemoteCheckResponse
			}{digest, response}
		}
		body, _ := json.Marshal(response)
		fmt.Fprintln(output, RemoteCheckPrefix+base64.RawURLEncoding.EncodeToString(body))
	}
	return scanner.Err()
}
