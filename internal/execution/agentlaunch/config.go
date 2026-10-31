// Package agentlaunch compiles agent settings into an opaque terminal command.
// Transport adapters do not interpret model or engine flags.
package agentlaunch

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"strings"
)

// Config targets a PowerShell terminal on the execution host.
type Config struct {
	AuditModel        *workflow.ModelSettings `json:"audit_model,omitempty"`
	RuntimeExecutable string                  `json:"runtime_executable,omitempty"`
	RepositoryURL     string                  `json:"repository_url,omitempty"`
	RepositoryRemote  string                  `json:"-"`
	AccessMode        string                  `json:"access_mode,omitempty"`
	Executable        string                  `json:"executable"`
	Args              []string                `json:"args,omitempty"`
	Model             string                  `json:"model,omitempty"`
	ReasoningEffort   string                  `json:"reasoning_effort,omitempty"`
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	copy := *c
	copy.Args = append([]string(nil), c.Args...)
	if c.AuditModel != nil {
		model := *c.AuditModel
		copy.AuditModel = &model
	}
	return &copy
}
func quote(text string) string { return "'" + strings.ReplaceAll(text, "'", "''") + "'" }

func (c *Config) InitializationParts() []m.InputPart {
	parts := []m.InputPart{{Source: "native.initialization", Owner: m.RunnerInput, Text: codex.NativeReadinessPrompt}}
	return parts
}

func (c *Config) Command(agent string) (string, error) {
	if c == nil {
		return "", nil
	}
	if c.RuntimeExecutable != "" {
		copy := *c
		copy.RuntimeExecutable = ""
		if _, err := copy.Command(agent); err != nil {
			return "", err
		}
		if strings.ContainsAny(c.RuntimeExecutable, "\x00\r\n") || agent != "codex" || c.RepositoryURL == "" {
			return "", errors.New("invalid remote runtime configuration")
		}
		payload, _ := json.Marshal(c.RemoteConfig())
		return "& " + quote(c.RuntimeExecutable) + " '--configuration' " + quote(base64.RawURLEncoding.EncodeToString(payload)), nil
	}
	if strings.TrimSpace(c.Executable) == "" || strings.ContainsAny(c.Executable+c.Model+c.ReasoningEffort, "\x00\r\n") {
		return "", errors.New("invalid agent launch executable or preferences")
	}
	args := append([]string(nil), c.Args...)
	if agent == "codex" {
		if err := codex.ValidateManagedArgs(args); err != nil {
			return "", err
		}
		permissions, err := codex.NativePermissions("windows", "orca", "codex", c.AccessMode)
		if err != nil {
			return "", err
		}
		args = append(args, permissions.Args()...)
	}
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return "", errors.New("invalid agent launch argument")
		}
	}
	if c.Model != "" || c.ReasoningEffort != "" {
		if agent != "codex" {
			return "", errors.New("model preference compiler is unavailable for this agent")
		}
		if c.Model == "" {
			return "", errors.New("reasoning effort requires a model")
		}
		switch c.ReasoningEffort {
		case "", "minimal", "low", "medium", "high", "xhigh", "max", "ultra":
		default:
			return "", errors.New("invalid reasoning effort")
		}
		for _, arg := range c.Args {
			if arg == "--" || strings.HasPrefix(arg, "-m") && !strings.HasPrefix(arg, "--") || arg == "--model" || strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-c") && !strings.HasPrefix(arg, "--") || arg == "--config" || strings.HasPrefix(arg, "--config=") {
				return "", errors.New("agent arguments cannot override declared model preferences")
			}
		}
		args = append(args, "--model", c.Model)
		if c.ReasoningEffort != "" {
			args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", c.ReasoningEffort))
		}
	}
	if agent == "codex" {
		// Animated repaints prevent the host terminal's generic quiet/idle
		// condition from settling. This is an agent-owned, session-only option.
		args = append(args, "-c", "tui.animations=false")
		// Establish the native session before transport waits for its identity.
		// This readiness input authorizes no business work, tools or file changes.
		text, _, err := m.FreezeInput(c.InitializationParts()...)
		if err != nil {
			return "", err
		}
		args = append(args, text)
	}
	parts := []string{"&", quote(c.Executable)}
	if agent == "codex" {
		if _, _, err := m.FreezeInput(m.InputPart{Source: "native.initialization", Owner: m.RunnerInput, Text: args[len(args)-1]}); err != nil {
			return "", err
		}
	}
	for _, arg := range args {
		parts = append(parts, quote(arg))
	}
	command := strings.Join(parts, " ")
	if agent == "codex" && c.RepositoryURL != "" && !gitHubRepository(c.RepositoryURL) {
		command = teaPathLaunch(command)
	}
	return command, nil
}

func (c *Config) RemoteConfig() codex.RemoteAgentConfig {
	return codex.RemoteAgentConfig{AuditModel: c.AuditModel, Executable: c.Executable, Args: c.Args, Model: c.Model, ReasoningEffort: c.ReasoningEffort, AccessMode: c.AccessMode, RepositoryURL: c.RepositoryURL}
}
func (c *Config) Display() []string {
	if c == nil {
		return nil
	}
	lines := []string{"代理启动：通用命令（覆盖原生默认启动）· 执行主机 PowerShell · 权限由启动参数与主机设置决定"}
	if c.Model != "" {
		lines = append(lines, "Model："+c.Model)
	}
	if c.ReasoningEffort != "" {
		lines = append(lines, "Reasoning effort："+c.ReasoningEffort)
	}
	return lines
}
