package codexworker

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/nativeaudit"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

// RemoteAgentConfig is public configuration. Authentication is resolved only on
// the machine that runs this helper, never transported by the Orca caller.
type RemoteAgentConfig struct {
	Executable      string   `json:"executable"`
	Args            []string `json:"args,omitempty"`
	Model           string   `json:"model,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
	AccessMode      string   `json:"access_mode,omitempty"`
	RepositoryURL   string   `json:"repository_url"`
}

const RemoteReadyPrefix = "SR_REMOTE_READY:"
const RemoteNativeTransport = "codex-remote-inline-v2"

type RemoteReadiness struct {
	NativeTransport     string            `json:"native_transport"`
	RuntimeSHA256       string            `json:"runtime_sha256"`
	Verified            bool              `json:"verified"`
	Contract            string            `json:"contract"`
	Workspace           string            `json:"workspace"`
	Repository          string            `json:"repository"`
	SessionID           string            `json:"session_id"`
	ConfigurationDigest string            `json:"configuration_digest"`
	Process             b.ProcessIdentity `json:"process"`
	CheckedAt           time.Time         `json:"checked_at"`
	ExpiresAt           time.Time         `json:"expires_at"`
	AccessMode          string            `json:"access_mode"`
}

func RemoteConfigurationDigest(c RemoteAgentConfig) string {
	raw, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func RemoteRuntimeDirectory(worktree string) string {
	key := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(worktree))))
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "SteerRunner", "remote-runtime", fmt.Sprintf("%x", key[:16]))
}

// RunRemoteAgent owns the credential broker and the original app-server tree.
// The TUI resumes that same server/thread only after its deterministic native
// command/exec probe passes. It never executes Git or publication on the host.
func RunRemoteAgent(ctx context.Context, c RemoteAgentConfig, input io.Reader, output, diagnostics io.Writer) error {
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
	auth, err := newAuthentication(nil, c.RepositoryURL)
	if err != nil {
		return err
	}
	defer auth.Close()
	if err = auth.prepare(ctx); err != nil {
		return err
	}
	worker.Auth = auth
	worker.OrcaHooks = true
	if err = auth.broker.installPublicCA(worker.Temp); err != nil {
		return err
	}
	roots, _ := WritableRoots(writeArgs)
	if err = auth.broker.installAPITransport(worker.Temp, roots); err != nil {
		return err
	}
	dir := RemoteRuntimeDirectory(wd)
	// Evidence lives outside the workspace and all four writable Git roots.
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err = host.WriteExecutionFile(filepath.Join(dir, "configuration.json"), encode(c)); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(dir, "ready.json")); err == nil {
		return errors.New("remote_runtime_already_exists; reconcile original session")
	}
	auth.onClose = func(revoked bool) {
		_ = host.WriteExecutionFile(filepath.Join(dir, "authentication-cleanup.json"), encode(map[string]any{"revocation_confirmed": revoked, "broker": auth.broker.facts()}))
	}
	owned, _, endpoint, err := LaunchServer(LaunchConfig{Executable: native, Workspace: wd, ServerDir: dir, Sandbox: permissions.Sandbox, WindowsSandbox: permissions.WindowsSandbox, Runtime: worker, WritableRoots: roots, Model: c.Model, ReasoningEffort: c.ReasoningEffort}, host.StartManaged, func(env []string) []string {
		return lifecycleProcessEnvironment(env, "remote-runtime", "remote-runtime", nil)
	})
	if err != nil {
		return err
	}
	defer owned.Close()
	client, err := ConnectAppServer(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.Close()
	identity := owned.Identity()
	h := &SessionHandle{Binding: Binding{Process: &identity, Endpoint: endpoint}, Paths: TurnPaths{TurnDir: dir}}
	backend := Backend{Config: Config{Executable: native, Workspace: wd, Sandbox: permissions.Sandbox, Authentication: auth, RemoteName: "origin"}, Process: ProcessPort{WriteAtomic: host.WriteExecutionFile}}
	if err = backend.checkSandboxReadiness(ctx, h, b.TurnInput{CommandID: "remote-preparation", LinkedWorktreeArgs: writeArgs}, client.Call); err != nil {
		return err
	}
	params, err := ThreadStartParams(wd, permissions.Sandbox, c.Model, nil)
	if err != nil {
		return err
	}
	params["developerInstructions"] = worker.instructions()
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
	bootstrap, err := TurnStartParams(wd, thread.Thread.ID, "remote-bootstrap", "Initialize this agent session. Reply READY only. Do not use tools or change files.", permissions.Sandbox, c.Model, c.ReasoningEffort, roots)
	if err != nil {
		return err
	}
	var turn struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if err = client.Call(ctx, "turn/start", bootstrap, &turn); err != nil {
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
				_ = client.RejectRequest(event.ID, "The initialization turn permits no tools or approvals.")
				return errors.New("remote_bootstrap_unexpected_request")
			}
			if event.Method == "turn/completed" {
				if _, e := decodeEnvironmentCompletion(event.Params, thread.Thread.ID, turn.Turn.ID); e != nil {
					return e
				}
				bootstrapped = true
			}
		}
	}
	mode := c.AccessMode
	if mode == "" {
		mode = "sandbox"
	}
	ready := RemoteReadiness{NativeTransport: RemoteNativeTransport, Verified: true, Contract: PolicyContract, Workspace: wd, Repository: c.RepositoryURL, SessionID: thread.Thread.ID, ConfigurationDigest: RemoteConfigurationDigest(c), Process: identity, CheckedAt: time.Now().UTC(), ExpiresAt: auth.expires, AccessMode: mode}
	ready.RuntimeSHA256, err = RuntimeBinarySHA256()
	if err != nil {
		return err
	}
	if err = host.WriteExecutionFile(filepath.Join(dir, "ready.json"), encode(ready)); err != nil {
		return err
	}
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
	args := remoteTUIArgs(c, endpoint, thread.Thread.ID)
	tui := exec.CommandContext(ctx, native, args...)
	tui.Dir = wd
	tui.Stdin, tui.Stdout, tui.Stderr = input, output, diagnostics
	// Only the app-server receives capability-bearing environment. Its frozen
	// policy owns all tool execution; native --remote rejects client overrides.
	tui.Env = lifecycleProcessEnvironment(os.Environ(), "remote-runtime", "remote-runtime", []string{"GH_TOKEN", "GITHUB_TOKEN"})
	return tui.Run()
}

func remoteTUIArgs(c RemoteAgentConfig, endpoint, session string) []string {
	// Inline mode preserves the preparation receipt. A second tracked Orca
	// prompt for helper admission would consume the next native input boundary.
	args := []string{"--remote", endpoint, "--strict-config", "--no-alt-screen"}
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
	if err != nil || json.Unmarshal(raw, &c) != nil {
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
