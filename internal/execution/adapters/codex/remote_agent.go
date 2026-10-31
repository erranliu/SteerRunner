package codexworker

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/nativeaudit"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RemoteAgentConfig is public configuration. Authentication is resolved only on
// the machine that runs this helper, never transported by the Orca caller.
type RemoteAgentConfig struct {
	models          model.Bindings  // transient assembly only; original configuration encoding is unchanged
	AuditModel      *model.Settings `json:"audit_model,omitempty"`
	Executable      string          `json:"executable"`
	Args            []string        `json:"args,omitempty"`
	Model           string          `json:"model,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	AccessMode      string          `json:"access_mode,omitempty"`
	RepositoryURL   string          `json:"repository_url"`
}

const RemoteReadyPrefix = "SR_REMOTE_READY:"
const RemoteFailurePrefix = "SR_REMOTE_AGENT_FAILURE:"
const RemoteNativeTransport = "codex-remote-inline-v4"

type RemoteReadiness struct {
	Input               *SessionInputFacts `json:"runner_input,omitempty"`
	Bootstrap           *SessionInputFacts `json:"runner_bootstrap,omitempty"`
	ExitPublicKey       []byte             `json:"exit_public_key,omitempty"`
	Owner               b.ProcessIdentity  `json:"owner"`
	NativeTransport     string             `json:"native_transport"`
	RuntimeSHA256       string             `json:"runtime_sha256"`
	Verified            bool               `json:"verified"`
	Contract            string             `json:"contract"`
	Workspace           string             `json:"workspace"`
	Repository          string             `json:"repository"`
	SessionID           string             `json:"session_id"`
	ConfigurationDigest string             `json:"configuration_digest"`
	Process             b.ProcessIdentity  `json:"process"`
	CheckedAt           time.Time          `json:"checked_at"`
	AccessMode          string             `json:"access_mode"`
}

func RemoteConfigurationDigest(c RemoteAgentConfig) string {
	raw, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func RemoteRuntimeDirectory(worktree string) string {
	key := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(worktree))))
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "SteerRunner", "remote-runtime", fmt.Sprintf("%x", key[:16]))
}

// RunRemoteAgent owns the original app-server tree and resumes the same thread
// in its TUI after the native bootstrap is ready. Git and publication use the
// execution host login through the original agent tools.
func RunRemoteAgent(ctx context.Context, c RemoteAgentConfig, input io.Reader, output, diagnostics io.Writer, bindings model.Bindings) (result error) {
	c.models = bindings.Clone()
	if err := c.validateModels(); err != nil {
		return err
	}
	if err := ValidateManagedArgs(c.Args); err != nil {
		return err
	}
	native, err := NativeExecutable(c.Executable)
	if err != nil {
		return errors.New("remote_native_executable_unavailable")
	}
	permissions, err := NativePermissions("windows", "orca", "codex", c.AccessMode)
	if err != nil {
		return err
	}
	host := process.New(30 * time.Second)
	version, err := host.OutputCommand(ctx, b.LaunchCommand{Executable: native, Args: []string{"--version"}})
	if err != nil || VerifyPolicyVersion(string(version)) != nil {
		return errors.New("remote_native_version_unsupported")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	git := workspace.Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 30 * time.Second, Start: host.StartManaged}
	repo, err := workspace.ResolveRepository(ctx, git, wd)
	if err != nil {
		return err
	}
	resources, err := workspace.ResolveLinkedResourcesAt(ctx, git, repo, wd)
	if err != nil {
		return err
	}
	writeArgs, _ := LinkedWorktreeWriteArgs(resources.Objects, resources.Refs, resources.Logs, resources.Admin)
	self, err := os.Executable()
	if err != nil {
		return err
	}
	rootPaths, _ := WritableRoots(writeArgs)
	if err = remoteTrustedPaths(wd, rootPaths, []string{self, native, RemoteRuntimeDirectory(wd)}); err != nil {
		return err
	}
	worker, err := prepareWorkerRuntime(wd, writeArgs)
	if err != nil {
		return err
	}
	worker.OrcaHooks = true
	roots, _ := WritableRoots(writeArgs)
	dir := RemoteRuntimeDirectory(wd)
	// Evidence lives outside the workspace and all four writable Git roots.
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(dir, "ready.json")); err == nil {
		return errors.New("remote_runtime_already_exists; reconcile original session")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = host.WriteExecutionFile(filepath.Join(dir, "configuration.json"), encode(c)); err != nil {
		return err
	}
	exitWriter, publicKey, err := newRemoteExitWriter(wd)
	if err != nil {
		return err
	}
	defer exitWriter.close()
	exclude, err := workspace.ResolveInfoExclude(ctx, git, repo, wd)
	if err != nil {
		return err
	}
	if err = excludeRemoteExit(exclude); err != nil {
		return err
	}
	owner, err := process.CurrentIdentity()
	if err != nil {
		return err
	}
	owned, _, endpoint, err := LaunchServer(LaunchConfig{Executable: native, Workspace: wd, ServerDir: dir, Sandbox: permissions.Sandbox, WindowsSandbox: permissions.WindowsSandbox, Runtime: worker, WritableRoots: roots, Model: c.Model, ReasoningEffort: c.ReasoningEffort}, host.StartManaged, func(env []string) []string {
		return lifecycleProcessEnvironment(env, "remote-runtime", "remote-runtime", nil)
	})
	if err != nil {
		return err
	}
	defer owned.Close()
	var ready *RemoteReadiness
	var tuiOwned b.StopHandle
	defer func() {
		// Never close a Job before querying its complete tree. Both the native
		// server/tools and the remote TUI are owned, non-breakaway Jobs.
		tuiAbsent := settleRemoteTree(tuiOwned)
		writersAbsent := settleRemoteTree(owned) && tuiAbsent
		if ready != nil {
			proof := RemoteExit{Readiness: *ready, WritersAbsent: writersAbsent, ClosedAt: time.Now().UTC()}
			if tuiOwned != nil {
				identity := tuiOwned.Identity()
				proof.TUI = &identity
			}
			result = errors.Join(result, exitWriter.write(proof))
		}
		if tuiOwned != nil {
			tuiOwned.Close()
		}
		if !writersAbsent {
			result = errors.Join(result, errors.New("original native tree cleanup unverified"))
		}
	}()
	client, err := ConnectAppServer(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.Close()
	identity := owned.Identity()
	params, err := ThreadStartParams(wd, permissions.Sandbox, c.Model, nil)
	if err != nil {
		return err
	}
	params["developerInstructions"] = worker.instructions()
	inputFacts, inputErr := sessionInput(params)
	if inputErr != nil {
		return inputErr
	}
	bootstrapParts := []model.InputPart{{Source: "native.bootstrap", Owner: model.RunnerInput, Text: BootstrapInstruction}}
	_, bootstrapBudget, err := model.FreezeInput(bootstrapParts...)
	if err != nil {
		return err
	}
	bootstrapFacts := &SessionInputFacts{Budget: bootstrapBudget, Parts: bootstrapParts, Fields: []string{"input[0].text"}}
	// Reuse the original readiness record; a pending input journal is explicitly
	// unverified and cannot authorize TUI/business activity.
	if err := host.WriteExecutionFile(filepath.Join(dir, "ready.json"), encode(RemoteReadiness{Input: inputFacts, Bootstrap: bootstrapFacts, Verified: false})); err != nil {
		return err
	}
	var thread struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err = client.Call(ctx, "thread/start", params, &thread); err != nil {
		return errors.New("remote_thread_start_failed")
	}
	if !ValidNativeID(thread.Thread.ID) {
		return errors.New("remote_thread_identity_invalid")
	}
	// Codex does not persist an empty thread's rollout. Complete the same
	// tool-free READY bootstrap used by the terminal launcher before the TUI
	// resumes it, so a second connection cannot invent a replacement session.
	bootstrap, err := TurnStartParams(wd, thread.Thread.ID, "remote-bootstrap", BootstrapInstruction, permissions.Sandbox, c.Model, c.ReasoningEffort, roots)
	if err != nil {
		return err
	}
	var turn struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	bootstrapReceipt, bootstrapErr := StartTurn(ctx, client, bootstrap, bootstrapBudget)
	turn.Turn.ID = bootstrapReceipt.Turn.ID
	if err = bootstrapErr; err != nil {
		return errors.New("remote_bootstrap_start_failed")
	}
	bootstrapCtx, cancelBootstrap := context.WithTimeout(ctx, 90*time.Second)
	defer cancelBootstrap()
	bootstrapped := false
	for !bootstrapped {
		select {
		case <-bootstrapCtx.Done():
			return errors.New("remote_bootstrap_unverified")
		case event, ok := <-client.Notifications():
			if !ok {
				return errors.New("remote_bootstrap_connection_lost")
			}
			if len(event.ID) > 0 {
				_ = client.RejectRequest(event.ID, codexapp.RejectInitializationRequest)
				return errors.New("remote_bootstrap_unexpected_request")
			}
			if event.Method == "turn/completed" {
				if e := validateTurnCompletion(event.Params, thread.Thread.ID, turn.Turn.ID); e != nil {
					return e
				}
				bootstrapped = true
			}
		}
	}
	mode := c.AccessMode
	prepared := RemoteReadiness{ExitPublicKey: publicKey, Owner: *owner, NativeTransport: RemoteNativeTransport, Verified: true, Contract: PolicyContract, Workspace: wd, Repository: c.RepositoryURL, SessionID: thread.Thread.ID, ConfigurationDigest: RemoteConfigurationDigest(c), Process: identity, CheckedAt: time.Now().UTC(), AccessMode: mode}
	prepared.Input, prepared.Bootstrap = inputFacts, bootstrapFacts
	prepared.RuntimeSHA256, err = RuntimeBinarySHA256()
	if err != nil {
		return err
	}
	if err = host.WriteExecutionFile(filepath.Join(dir, "ready.json"), encode(prepared)); err != nil {
		return err
	}
	ready = &prepared
	// The transport captures this finite receipt before any business input and
	// binds it to the original PTY incarnation and native provider session.
	fmt.Fprintln(output, RemoteReadyPrefix+base64.RawURLEncoding.EncodeToString(encode(ready)))
	// Keep draining notifications so the independent preparation connection
	// cannot stall the native server while the TUI owns business interaction.
	go func() {
		for event := range client.Notifications() {
			if strings.Contains(strings.ToLower(event.Method), "approval") {
				_ = host.WriteExecutionFile(filepath.Join(dir, "unexpected-approval.json"), encode(map[string]string{"method": event.Method}))
			}
		}
	}()
	bridgeToken := ""
	if c.AuditModel != nil {
		bridgeToken, err = remoteModelToken()
		if err != nil {
			return err
		}
		bridge, closeBridge, bridgeErr := startRemoteModelBridge(ctx, c, endpoint, thread.Thread.ID, bridgeToken)
		if bridgeErr != nil {
			return bridgeErr
		}
		defer closeBridge()
		endpoint = bridge
	}
	args := remoteTUIArgs(c, endpoint, thread.Thread.ID)
	tui := exec.Command(native, args...)
	tui.Dir = wd
	tui.Stdin, tui.Stdout, tui.Stderr = input, output, diagnostics
	// Only the app-server receives capability-bearing environment. Its frozen
	// policy owns all tool execution; native --remote rejects client overrides.
	blocked := []string{"GH_TOKEN", "GITHUB_TOKEN", remoteModelTokenEnv}
	tui.Env = lifecycleProcessEnvironment(os.Environ(), "remote-runtime", "remote-runtime", blocked)
	if bridgeToken != "" {
		tui.Env = setEnvironmentValues(tui.Env, map[string]string{remoteModelTokenEnv: bridgeToken})
	}
	started, err := host.StartManaged(tui)
	if err != nil {
		return err
	}
	tuiOwned = started
	waited := make(chan error, 1)
	go func() { waited <- tui.Wait() }()
	stopCtx, stop := context.WithCancel(ctx)
	defer stop()
	requested := make(chan bool, 1)
	go func() { requested <- awaitRemoteStop(stopCtx, prepared) }()
	select {
	case err = <-waited:
		return err
	case explicit := <-requested:
		tuiOwned.Kill()
		<-waited
		if explicit {
			return nil
		}
		return ctx.Err()
	case <-ctx.Done():
		tuiOwned.Kill()
		<-waited
		return ctx.Err()
	}
}

// Keep the socket below the execution-host state root, rather than the deep
// Git or protected evidence directory. The exact lease binds the original
// workspace and frozen public configuration; only its child gets write access.

func remoteTUIArgs(c RemoteAgentConfig, endpoint, session string) []string {
	// Inline mode preserves the preparation receipt. A second tracked Orca
	// prompt for helper admission would consume the next native input boundary.
	args := []string{"--remote", endpoint, "--strict-config", "--no-alt-screen"}
	if c.AuditModel != nil {
		args = append(args, "--remote-auth-token-env", remoteModelTokenEnv)
	}
	for _, arg := range c.Args {
		if arg != "--no-daemon" && arg != "--no-alt-screen" {
			args = append(args, arg)
		}
	}
	return append(args, "-c", "tui.animations=false", "resume", session)
}

func RuntimeBinarySHA256() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func remoteTrustedPaths(wd string, roots, paths []string) error {
	for _, trusted := range paths {
		for _, root := range append([]string{wd}, roots...) {
			rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(trusted))
			if err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
				return errors.New("remote_runtime_inside_worker_write_scope")
			}
		}
	}
	return nil
}

type RemoteAudit struct {
	Inspector       *b.ProcessIdentity       `json:"inspector,omitempty"`
	InspectorClosed bool                     `json:"inspector_closed"`
	Readiness       RemoteReadiness          `json:"readiness"`
	Native          nativeaudit.SandboxAudit `json:"native"`
}

// AuditRemoteAgent reads original native history on its execution host, and
// exports only permission counts/digests and the preparation receipt.
func AuditRemoteAgent(ctx context.Context, wd, session string) (RemoteAudit, error) {
	var audit RemoteAudit
	dir := RemoteRuntimeDirectory(wd)
	raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	if err != nil {
		return audit, err
	}
	if json.Unmarshal(raw, &audit.Readiness) != nil || !sameWorkspace(audit.Readiness.Workspace, wd) || audit.Readiness.SessionID != session || !audit.Readiness.Verified {
		return audit, errors.New("remote audit identity mismatch")
	}
	if _, err = os.Stat(filepath.Join(dir, "unexpected-approval.json")); err == nil {
		return audit, errors.New("remote native approval request observed")
	}
	var c RemoteAgentConfig
	raw, err = os.ReadFile(filepath.Join(dir, "configuration.json"))
	if err != nil || b.DecodeStrictJSON(raw, &c, "remote audit configuration has trailing JSON") != nil {
		return audit, errors.New("remote audit configuration unavailable")
	}
	if RemoteConfigurationDigest(c) != audit.Readiness.ConfigurationDigest {
		return audit, errors.New("remote audit configuration changed")
	}
	native, err := NativeExecutable(c.Executable)
	if err != nil {
		return audit, err
	}
	reader, err := OpenHistory(ctx, b.Command{Executable: native}, wd, "", process.New(30*time.Second).StartManaged)
	if err != nil {
		return audit, err
	}
	defer reader.Close()
	thread, err := reader.ReadThread(ctx, session, wd)
	if err != nil {
		return audit, err
	}
	f, err := os.Open(thread.Path)
	if err != nil {
		return audit, err
	}
	defer f.Close()
	audit.Native, err = nativeaudit.AuditAccessModeHistory(f, session, wd, audit.Readiness.AccessMode)
	return audit, err
}
