package orcatransport

import (
	"context"
	"crypto/sha256"
	"embed"
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

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

//go:embed extensions/pi-evidence.mjs extensions/pi-managed.mjs extensions/dsh-managed.mjs extensions/terminal-tools.mjs extensions/receipt.mjs
var terminalAssets embed.FS

type TerminalConfig struct {
	ReceiptVersion      int    `json:"receipt_version,omitempty"`
	Agent               string `json:"agent"`
	BundleSHA256        string `json:"bundle_sha256,omitempty"`
	EvidenceSHA256      string `json:"evidence_sha256,omitempty"`
	ProjectID           string `json:"project_id"`
	ExecutionID         string `json:"execution_id"`
	AttemptID           string `json:"attempt_id"`
	Generation          string `json:"generation"`
	ConfigurationDigest string `json:"configuration_digest"`
	SessionID           string `json:"session_id"`
	Executable          string `json:"executable"`
	SandboxExecutable   string `json:"sandbox_executable"`
	StateDirectory      string `json:"state_directory"`
	AuthenticationState string `json:"authentication_state"`
	RepositoryURL       string `json:"repository_url"`
	AccessMode          string `json:"access_mode"`
	Model               string `json:"model,omitempty"`
	ReasoningEffort     string `json:"reasoning_effort,omitempty"`
}

type TerminalMetadata struct {
	Version       int    `json:"version"`
	Contract      string `json:"contract"`
	RuntimeSHA256 string `json:"runtime_sha256"`
	BundleSHA256  string `json:"bundle_sha256"`
	PiSHA256      string `json:"pi_sha256"`
	DSHSHA256     string `json:"dsh_sha256"`
}

// Metadata is captured from the selected helper before accepting a Run, so an
// updated Runner can still read the original, frozen helper's evidence.
func TerminalRuntimeMetadata() (TerminalMetadata, error) {
	self, e := os.Executable()
	if e != nil {
		return TerminalMetadata{}, e
	}
	raw, e := os.ReadFile(self)
	if e != nil {
		return TerminalMetadata{}, e
	}
	pi, _ := terminalAssets.ReadFile("extensions/pi-evidence.mjs")
	dsh, _ := terminalAssets.ReadFile("extensions/dsh-managed.mjs")
	return TerminalMetadata{Version: 3, Contract: terminaltools.Contract, RuntimeSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), BundleSHA256: TerminalExtensionDigest(), PiSHA256: fmt.Sprintf("%x", sha256.Sum256(pi)), DSHSHA256: fmt.Sprintf("%x", sha256.Sum256(dsh))}, nil
}

type TerminalOwner struct {
	Job                 process.OwnedJobReference `json:"job"`
	RuntimeSHA256       string                    `json:"runtime_sha256"`
	ExtensionSHA256     string                    `json:"extension_bundle_sha256"`
	SessionID           string                    `json:"session_id"`
	RunID               string                    `json:"run_id"`
	ConfigurationDigest string                    `json:"configuration_digest"`
	AccessMode          string                    `json:"access_mode"`
}

func (c TerminalConfig) Command(runtime string) string {
	raw, _ := json.Marshal(c)
	return "& '" + strings.ReplaceAll(runtime, "'", "''") + "' '--terminal-configuration' '" + base64.RawURLEncoding.EncodeToString(raw) + "'"
}

func TerminalExtensionDigest() string {
	h := sha256.New()
	for _, name := range []string{"pi-evidence.mjs", "pi-managed.mjs", "dsh-managed.mjs", "terminal-tools.mjs", "receipt.mjs"} {
		raw, _ := terminalAssets.ReadFile("extensions/" + name)
		h.Write([]byte(name))
		h.Write(raw)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// RunTerminalAgent keeps the original TUI on Orca's stdio. Only native tools use
// the sandbox CLI; this path never starts a Codex model/server or a Pi SDK agent.
func RunTerminalAgent(ctx context.Context, c TerminalConfig, input io.Reader, output, diagnostics io.Writer) (result error) {
	if c.Agent != "pi" && c.Agent != "dsh" || c.AccessMode != "sandbox" && c.AccessMode != "full_access" || !filepath.IsAbs(c.StateDirectory) || !filepath.IsAbs(c.AuthenticationState) || !b.StableExecutionID(c.ExecutionID) || c.SessionID == "" {
		return errors.New("native terminal frozen launch configuration invalid")
	}
	metadata, err := TerminalRuntimeMetadata()
	if err != nil {
		return err
	}
	sourceSHA := metadata.PiSHA256
	if c.Agent == "dsh" {
		sourceSHA = metadata.DSHSHA256
	}
	if c.BundleSHA256 != "" && c.BundleSHA256 != metadata.BundleSHA256 || c.EvidenceSHA256 != "" && c.EvidenceSHA256 != sourceSHA {
		return errors.New("native terminal helper sources changed")
	}
	working, err := os.Getwd()
	if err != nil {
		return err
	}
	for _, name := range []string{"config.json", "owner.json", "startup-exit.json"} {
		if _, err = os.Stat(filepath.Join(c.StateDirectory, name)); !os.IsNotExist(err) {
			return errors.New("original native terminal runtime already exists; reconcile only")
		}
	}
	if err = os.MkdirAll(c.StateDirectory, 0700); err != nil {
		return err
	}
	if err = os.MkdirAll(c.AuthenticationState, 0700); err != nil {
		return err
	}
	host := process.New(time.Minute)
	root, err := process.CurrentIdentity()
	if err != nil {
		return err
	}
	startupOwner := TerminalOwner{Job: process.OwnedJobReference{Owner: *root}, RuntimeSHA256: metadata.RuntimeSHA256, ExtensionSHA256: metadata.BundleSHA256, SessionID: c.SessionID, RunID: c.ExecutionID, ConfigurationDigest: c.ConfigurationDigest, AccessMode: c.AccessMode}
	workerStarted, authenticationClosed := false, true
	// A positive failure receipt is written only after synchronous preparation
	// commands have returned and their owned Jobs are closed. Missing evidence
	// never proves no worker. The reader also requires this exact helper to exit.
	defer func() {
		if !workerStarted {
			failure := "native terminal preparation returned without starting a worker"
			if result != nil {
				failure = result.Error()
			}
			for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITEA_TOKEN"} {
				if secret := os.Getenv(name); secret != "" {
					failure = strings.ReplaceAll(failure, secret, "[REDACTED]")
				}
			}
			failure = fmt.Sprintf("%.2048s", failure)
			raw, _ := json.Marshal(TerminalStartupFailure{Version: 1, Failure: failure, Owner: startupOwner, NoWorkerStarted: true, AuthenticationClosed: authenticationClosed, ClosedAt: time.Now().UTC()})
			result = errors.Join(result, host.WriteExecutionFile(filepath.Join(c.StateDirectory, "startup-exit.json"), raw))
		}
	}()
	git := workspace.Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 30 * time.Second, Start: host.StartManaged}
	repo, err := workspace.ResolveRepository(ctx, git, working)
	if err != nil {
		return err
	}
	roots, err := workspace.ResolveLinkedResourcesAt(ctx, git, repo, working)
	if err != nil {
		return err
	}
	rootPaths := []string{roots.Objects, roots.Refs, roots.Logs, roots.Admin}
	sandbox, err := codex.NativeExecutable(c.SandboxExecutable)
	if err != nil {
		return err
	}
	version, err := host.OutputCommand(ctx, b.LaunchCommand{Executable: sandbox, Args: []string{"--version"}})
	if err != nil || codex.VerifyPolicyVersion(string(version)) != nil {
		return errors.New("Pi native sandbox version unsupported")
	}
	agentExecutable, err := exec.LookPath(c.Executable)
	if err != nil {
		return err
	}
	moduleRoot := filepath.Join(filepath.Dir(agentExecutable), "node_modules")
	if filepath.Base(filepath.Dir(agentExecutable)) == ".bin" {
		moduleRoot = filepath.Dir(filepath.Dir(agentExecutable))
	}
	packageRoot := filepath.Join(moduleRoot, "@earendil-works", "pi-coding-agent")
	expectedName, expectedVersion, binName, entry := "@earendil-works/pi-coding-agent", "0.87.1", "pi", "dist/bundle/cli.js"
	if c.Agent == "dsh" {
		packageRoot = filepath.Join(moduleRoot, "@deepseek-ai", "dsh")
		expectedName, expectedVersion, binName, entry = "@deepseek-ai/dsh", "0.2.1-rc.2", "dsh", "lib/bin.js"
	}
	var packageMetadata struct {
		Name    string            `json:"name"`
		Version string            `json:"version"`
		Bin     map[string]string `json:"bin"`
	}
	raw, err := os.ReadFile(filepath.Join(packageRoot, "package.json"))
	if err != nil || json.Unmarshal(raw, &packageMetadata) != nil || packageMetadata.Name != expectedName || packageMetadata.Version != expectedVersion || packageMetadata.Bin[binName] != entry {
		return errors.New("Pi native package/version unsupported")
	}
	node, err := exec.LookPath("node.exe")
	if err != nil {
		return err
	}
	identity := codex.PolicyResourceIdentity{ProjectID: c.ProjectID, ExecutionID: c.ExecutionID, AttemptID: c.AttemptID, Generation: c.Generation, ConfigDigest: c.ConfigurationDigest}
	auth, err := codex.PrepareTerminalAuthentication(ctx, working, c.RepositoryURL, c.AuthenticationState, identity, rootPaths, os.Environ(), c.AccessMode)
	if err != nil {
		if auth != nil {
			authenticationClosed = auth.Close()
		}
		return err
	}
	authenticationClosed = false
	defer func() { authenticationClosed = auth.Close() }()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	policy := terminaltools.Config{AccessMode: c.AccessMode, Workspace: working, WriteRoots: auth.WriteRoots, SandboxExecutable: sandbox, Shell: shell, StateDirectory: c.StateDirectory}
	if err = policy.Validate(); err != nil {
		return err
	}
	for _, name := range []string{"pi-evidence.mjs", "pi-managed.mjs", "dsh-managed.mjs", "terminal-tools.mjs", "receipt.mjs"} {
		body, _ := terminalAssets.ReadFile("extensions/" + name)
		if err = host.WriteExecutionFile(filepath.Join(c.StateDirectory, name), body); err != nil {
			return err
		}
	}
	config := map[string]any{"version": 1, "run_id": c.ExecutionID, "session_id": c.SessionID, "cwd": working, "directory": c.StateDirectory, "tool_host": self, "tool_policy": policy, "auth_probe": auth.Probe, "authentication_expires_at": auth.ExpiresAt, "extension_bundle_sha256": TerminalExtensionDigest(), "reasoning_effort": c.ReasoningEffort, "model": c.Model}
	if c.Agent == "dsh" {
		config["dsh_tools_module"] = filepath.Join(moduleRoot, "@deepseek-ai", "dsh-tools", "lib", "index.js")
		config["dsh_llm_module"] = filepath.Join(moduleRoot, "@deepseek-ai", "dsh-llm", "lib", "index.js")
	}
	configBytes, _ := json.Marshal(config)
	if err = host.WriteExecutionFile(filepath.Join(c.StateDirectory, "config.json"), configBytes); err != nil {
		return err
	}
	args := []string{filepath.Join(packageRoot, "dist", "bundle", "cli.js"), "--offline", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-themes", "--no-approve", "--tools", "read,write,edit,powershell", "--session-id", c.SessionID, "--session-dir", filepath.Join(c.StateDirectory, "sessions"), "-e", filepath.Join(c.StateDirectory, "pi-managed.mjs"), "--steerrunner-evidence-config", filepath.Join(c.StateDirectory, "config.json")}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	if c.ReasoningEffort != "" {
		args = append(args, "--thinking", c.ReasoningEffort)
	}
	if c.Agent == "dsh" {
		if err = prepareDSHProfile(ctx, host, c, moduleRoot, shell); err != nil {
			return err
		}
		args = []string{filepath.Join(packageRoot, entry), "--profile", "runner"}
	}
	command := exec.Command(node, args...)
	command.Dir = working
	command.Stdin = input
	command.Stdout = output
	command.Stderr = diagnostics
	envObject := map[string]string{}
	for _, entry := range auth.Environment {
		key, value, _ := strings.Cut(entry, "=")
		envObject[key] = value
	}
	envBytes, _ := json.Marshal(envObject)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if upper == "GH_TOKEN" || upper == "GITHUB_TOKEN" || upper == "GH_CONFIG_DIR" || strings.HasPrefix(upper, "GIT_CONFIG_") || strings.HasPrefix(upper, "SR_TERMINAL_") {
			continue
		}
		if c.Agent == "dsh" && (upper == "DSH_HOME" || strings.HasPrefix(upper, "DSH_TUI_") || upper == "HOME" || upper == "USERPROFILE") {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env, "SR_TERMINAL_TOOL_ENV="+string(envBytes), "SR_TERMINAL_CONFIG="+filepath.Join(c.StateDirectory, "config.json"), "PI_OFFLINE=1", "PI_TELEMETRY=0")
	if c.Agent == "dsh" {
		home := filepath.Join(c.StateDirectory, "home")
		command.Env = append(command.Env, "HOME="+home, "USERPROFILE="+home, "DSH_HOME="+filepath.Join(home, ".dsh"), "DSH_TUI_PRESET=runner", "DSH_TUI_BACKEND=dsh")
	}
	owned, err := host.StartManaged(command)
	if err != nil {
		return err
	}
	workerStarted = true
	defer owned.Close()
	job, err := process.DescribeOwnedJob(owned)
	if err != nil {
		owned.Kill()
		command.Wait()
		return err
	}
	runtimeBytes, err := os.ReadFile(self)
	if err != nil {
		owned.Kill()
		command.Wait()
		return err
	}
	ownerRecord := TerminalOwner{Job: job, RuntimeSHA256: fmt.Sprintf("%x", sha256.Sum256(runtimeBytes)), ExtensionSHA256: TerminalExtensionDigest(), SessionID: c.SessionID, RunID: c.ExecutionID, ConfigurationDigest: c.ConfigurationDigest, AccessMode: c.AccessMode}
	ownerBytes, _ := json.Marshal(ownerRecord)
	if err = host.WriteExecutionFile(filepath.Join(c.StateDirectory, "owner.json"), ownerBytes); err != nil {
		owned.Kill()
		command.Wait()
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		owned.Kill()
		<-done
		err = ctx.Err()
	}
	owned.Kill()
	tree, ok := owned.(b.ProcessTreeHandle)
	if !ok {
		return errors.New("Pi owned writers cannot be verified")
	}
	until := time.Now().Add(2 * time.Second)
	for !tree.WritersAbsent() && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if !tree.WritersAbsent() {
		return errors.New("Pi owned writers remain unverified")
	}
	authenticationClosed = auth.Close()
	exitBytes, _ := json.Marshal(TerminalExit{Version: 1, Owner: ownerRecord, WritersAbsent: true, AuthenticationClosed: &authenticationClosed, ClosedAt: time.Now().UTC()})
	if e := host.WriteExecutionFile(filepath.Join(c.StateDirectory, "exit.json"), exitBytes); e != nil {
		return e
	}
	return err
}
