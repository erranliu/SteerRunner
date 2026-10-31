package codexworker

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type LaunchConfig struct {
	AuthenticationTransport *AuthenticationTransport
	WritableRoots           []string
	Model                   string
	ReasoningEffort         string
	Executable              string
	Args                    []string
	Profile                 string
	Workspace               string
	ServerDir               string
	Sandbox                 string
	WindowsSandbox          string
	Runtime                 *workerRuntime
}

// LaunchServer starts the fixed native app-server. The caller supplies its
// platform process starter and persists the returned identity before a turn.
func LaunchServer[T any](config LaunchConfig, start func(*exec.Cmd) (T, error), environment func([]string) []string) (identity T, process *os.Process, endpoint string, err error) {
	if err = (Config{Sandbox: config.Sandbox, WindowsSandbox: config.WindowsSandbox, Args: config.Args, Profile: config.Profile}).Validate(runtime.GOOS); err != nil {
		return identity, nil, "", err
	}
	permissions, err := NativePermissions(runtime.GOOS, "local", "codex", accessMode(config.Sandbox))
	if err != nil {
		return identity, nil, "", err
	}
	// Each Runner session owns its app-server and environment. Do not route a
	// new session through a pre-existing user daemon with another environment.
	args := append([]string{"--no-daemon", "--strict-config"}, config.Args...)
	args = append(args, permissions.Args()...)
	roots := append([]string(nil), config.WritableRoots...)
	if len(roots) > 0 {
		if len(config.WritableRoots) != 4 {
			return identity, nil, "", fmt.Errorf("remote native server needs four verified Git roots")
		}
	}
	if plan := config.AuthenticationTransport; plan != nil {
		if err = plan.validate(); err != nil {
			return identity, nil, "", err
		}
		if len(roots) != 4 || !sameWorkspace(plan.Workspace, config.Workspace) {
			return identity, nil, "", fmt.Errorf("authentication_api_binding_mismatch")
		}
		roots = append(roots, plan.Directory)
	}
	if config.Model != "" {
		args = append(args, "--config", fmt.Sprintf("model=%q", config.Model))
	}
	if config.ReasoningEffort != "" {
		if !ValidReasoningEffort(config.ReasoningEffort) {
			return identity, nil, "", fmt.Errorf("invalid server reasoning effort")
		}
		args = append(args, "--config", fmt.Sprintf("model_reasoning_effort=%q", config.ReasoningEffort))
	}
	err = ValidateManagedArgs(config.Args)
	if err != nil {
		return identity, nil, "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return identity, nil, "", err
	}
	endpoint = "ws://" + listener.Addr().String()
	_ = listener.Close()
	launch, err := AppServerLaunch(config.Executable, args, config.Profile, endpoint)
	if err != nil {
		return identity, nil, "", err
	}
	output, err := os.OpenFile(filepath.Join(config.ServerDir, "appserver.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return identity, nil, "", err
	}
	launch.WorkingDirectory = config.Workspace
	cmd := exec.Command(launch.Executable, launch.Args...)
	cmd.Dir = launch.WorkingDirectory
	cmd.Env = environment(cmd.Environ())
	if config.Runtime != nil {
		cmd.Env, err = config.Runtime.environment(cmd.Env)
		if err != nil {
			_ = output.Close()
			return identity, nil, "", err
		}
		// Native execution servers may rebuild their tool environment. Pin only
		// this task's public runtime values through the native shell policy too.
		runtimeArgs, runtimeErr := config.Runtime.configArgs(cmd.Env)
		if runtimeErr != nil {
			_ = output.Close()
			return identity, nil, "", runtimeErr
		}
		cmd.Args = append([]string{launch.Executable}, append(runtimeArgs, launch.Args...)...)
	}
	cmd.Stdout, cmd.Stderr = output, output
	identity, err = start(cmd)
	_ = output.Close()
	if err != nil {
		return identity, nil, "", err
	}
	go func() {
		_ = cmd.Wait()
		if config.Runtime != nil && config.Runtime.Auth != nil {
			config.Runtime.Auth.Close()
		}
	}()
	return identity, cmd.Process, endpoint, nil
}
