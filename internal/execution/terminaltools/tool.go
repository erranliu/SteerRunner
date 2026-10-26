// Package terminaltools binds terminal-agent tools to the existing Windows
// process host. It does not create agent sessions or call an agent SDK.
package terminaltools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

const Contract = "windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never"

// Config is compiled by the trusted launch, never from a model tool argument.
// Authentication remains in inherited process environment, not this document.
type Config struct {
	AccessMode        string   `json:"access_mode"`
	Workspace         string   `json:"workspace"`
	WriteRoots        []string `json:"write_roots"`
	SandboxExecutable string   `json:"sandbox_executable"`
	Shell             string   `json:"shell"`
	StateDirectory    string   `json:"state_directory"`
}

func (c Config) Validate() error {
	if runtime.GOOS != "windows" {
		return errors.New("terminal tools require Windows")
	}
	if c.AccessMode != "sandbox" && c.AccessMode != "full_access" {
		return errors.New("invalid terminal tool access mode")
	}
	for _, p := range append([]string{c.Workspace, c.StateDirectory, c.Shell}, c.WriteRoots...) {
		if !filepath.IsAbs(p) || strings.ContainsAny(p, "\x00\r\n") {
			return errors.New("terminal tool path must be absolute")
		}
	}
	if !strings.EqualFold(filepath.Clean(c.Shell), filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")) {
		return errors.New("terminal tool shell differs from fixed Windows shell")
	}
	for _, root := range append([]string{c.Workspace}, c.WriteRoots...) {
		rel, err := filepath.Rel(root, c.StateDirectory)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return errors.New("terminal tool state is inside a writable root")
		}
	}
	if c.AccessMode == "sandbox" && (!filepath.IsAbs(c.SandboxExecutable) || strings.ContainsAny(c.SandboxExecutable, "\x00\r\n")) {
		return errors.New("terminal sandbox executable missing")
	}
	return nil
}

func (c Config) Command(script string) (b.LaunchCommand, error) {
	if err := c.Validate(); err != nil {
		return b.LaunchCommand{}, err
	}
	if !filepath.IsAbs(script) || filepath.Dir(script) != c.StateDirectory {
		return b.LaunchCommand{}, errors.New("tool script is outside original state directory")
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script}
	if c.AccessMode == "full_access" {
		return b.LaunchCommand{Executable: c.Shell, Args: args, WorkingDirectory: c.Workspace}, nil
	}
	// JSON string quoting is also valid TOML basic-string quoting. These are
	// argv values, not shell expressions. Explicit roots avoid ambient temp grants.
	quote := func(v string) string { raw, _ := json.Marshal(v); return string(raw) }
	entries := []string{`":root"="read"`}
	for _, root := range append([]string{c.Workspace}, c.WriteRoots...) {
		entries = append(entries, quote(root)+`="write"`)
	}
	policy := "permissions.steerrunner-terminal.filesystem={" + strings.Join(entries, ",") + "}"
	bound := []string{"sandbox", "--permission-profile", "steerrunner-terminal", "-c", "windows.sandbox=mxc", "-c", policy, "-c", "permissions.steerrunner-terminal.network.enabled=true", "-C", c.Workspace, "--", c.Shell}
	return b.LaunchCommand{Executable: c.SandboxExecutable, Args: append(bound, args...), WorkingDirectory: c.Workspace}, nil
}

// Run owns exactly one native tool tree. Even successful shell exit does not
// release surviving children; revoke the Job and prove absence before cleanup.
func Run(ctx context.Context, c Config, input io.Reader, output, diagnostics io.Writer) error {
	if err := c.Validate(); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(input, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return errors.New("terminal tool input exceeds limit")
	}
	info, err := os.Lstat(c.StateDirectory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("terminal tool state directory unavailable")
	}
	real, err := filepath.EvalSymlinks(c.StateDirectory)
	if err != nil || !strings.EqualFold(real, filepath.Clean(c.StateDirectory)) {
		return errors.New("terminal tool state directory redirected")
	}
	file, err := os.CreateTemp(c.StateDirectory, "tool-*.ps1")
	if err != nil {
		return err
	}
	name := file.Name()
	// Preserve PowerShell native-command semantics: stderr captured with 2>&1
	// is data, not a terminating exception. Trusted filesystem/preparation
	// scripts opt into Stop. LASTEXITCODE alone is stale after a later cmdlet.
	// PS5 also sets $? false for captured native stderr even at exit zero. Only
	// forgive that specific error at the final command's exact source location;
	// never borrow a previous native error to forgive a failing cmdlet.
	script := "$ErrorActionPreference='Continue'\n$ProgressPreference='SilentlyContinue'\n[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)\n$OutputEncoding=[Console]::OutputEncoding\n" + string(body) + `
if ($?) { exit 0 } else {
  $srError=$Error[0]; $srNativeExit=$LASTEXITCODE
  $srAst=[System.Management.Automation.Language.Parser]::ParseFile($PSCommandPath,[ref]$null,[ref]$null)
  $srLast=$srAst.EndBlock.Statements[-2]
  if ($srLast -is [System.Management.Automation.Language.PipelineAst]) {
    $srLast=$srLast.PipelineElements[-1]
    if ($srNativeExit -eq 0 -and $srError.FullyQualifiedErrorId -eq 'NativeCommandError' -and
        $srError.InvocationInfo.ScriptName -eq $PSCommandPath -and
        $srError.InvocationInfo.ScriptLineNumber -eq $srLast.Extent.StartLineNumber -and
        $srError.InvocationInfo.OffsetInLine -eq $srLast.Extent.StartColumnNumber) { exit 0 }
  }
  exit 1
}
`
	_, err = file.Write(append([]byte{0xef, 0xbb, 0xbf}, []byte(script)...))
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(name)
		return errors.New("terminal tool staging failed")
	}
	command, err := c.Command(name)
	if err != nil {
		_ = os.Remove(name)
		return err
	}
	cmd := exec.Command(command.Executable, command.Args...)
	cmd.Dir = command.WorkingDirectory
	cmd.Env = ToolEnvironment(os.Environ())
	cmd.Stdout, cmd.Stderr = output, diagnostics
	owned, err := process.New(time.Minute).StartManaged(cmd)
	if err != nil {
		_ = os.Remove(name)
		return err
	}
	defer owned.Close()
	tree, ok := owned.(b.ProcessTreeHandle)
	if !ok {
		owned.Kill()
		return errors.New("terminal tool process tree cannot be verified")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		owned.Kill()
		<-done
		// A revoked/deadline-bound command may already have had side effects.
		// Keep that uncertainty distinct from an ordinary native exit failure.
		err = ctx.Err()
	}
	owned.Kill()
	until := time.Now().Add(2 * time.Second)
	for !tree.WritersAbsent() && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if !tree.WritersAbsent() {
		return errors.New("terminal tool owned writers remain unverified")
	}
	if removeErr := os.Remove(name); removeErr != nil {
		return fmt.Errorf("terminal tool staging cleanup: %w", removeErr)
	}
	return err
}

// The trusted runtime supplies repository-scoped authentication before this
// boundary. Pi model-provider credentials and unrelated caller secrets never
// reach native tools, in either permission mode.
func ToolEnvironment(base []string) []string {
	allowed := map[string]bool{}
	for _, key := range strings.Fields("PATH PATHEXT SYSTEMROOT SYSTEMDRIVE COMSPEC WINDIR TEMP TMP USERPROFILE HOME APPDATA LOCALAPPDATA PROGRAMFILES PROGRAMFILES(X86) PROGRAMDATA GH_HOST GH_TOKEN GH_PROMPT_DISABLED GH_CONFIG_DIR GIT_TERMINAL_PROMPT HTTPS_PROXY HTTP_PROXY NO_PROXY SSL_CERT_FILE GIT_SSL_CAINFO CODEX_HOME STEERLINE_PROGRESS_FILE STEERLINE_RESULT_FILE") {
		allowed[key] = true
	}
	var result []string
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if allowed[upper] || strings.HasPrefix(upper, "GIT_CONFIG_") {
			result = append(result, entry)
		}
	}
	return result
}
