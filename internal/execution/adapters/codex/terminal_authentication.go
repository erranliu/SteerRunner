package codexworker

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
)

// TerminalAuthentication exposes only the existing repository-scoped broker
// to native terminal tools. It carries no Codex session or completion proof.
type TerminalAuthentication struct {
	auth        *AuthenticationRuntime
	Environment []string
	Probe       string
	ExpiresAt   time.Time
	WriteRoots  []string
}

func PrepareTerminalAuthentication(ctx context.Context, workspace, remote, state string, identity PolicyResourceIdentity, roots, base []string, mode string) (prepared *TerminalAuthentication, result error) {
	if mode != "sandbox" && mode != "full_access" {
		return nil, errors.New("terminal authentication access mode invalid")
	}
	args, err := LinkedWorktreeWriteArgs(rootsAt(roots, 0), rootsAt(roots, 1), rootsAt(roots, 2), rootsAt(roots, 3))
	if err != nil || len(roots) != 4 {
		return nil, errors.New("terminal authentication requires four verified Git roots")
	}
	worker, err := prepareWorkerRuntime(workspace, args)
	if err != nil {
		return nil, err
	}
	auth, err := newAuthentication(nil, remote)
	if err != nil {
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			auth.Close()
			// Preserve cleanup truth even when preparation fails part way through.
			prepared = &TerminalAuthentication{auth: auth}
		}
	}()
	if err = auth.prepare(ctx); err != nil {
		return nil, err
	}
	worker.Auth = auth
	if err = auth.broker.installPublicCA(worker.Temp); err != nil {
		return nil, err
	}
	plan, err := PlanAuthenticationTransport(state, workspace, remote, identity)
	if err != nil {
		return nil, err
	}
	if err = auth.broker.installAPITransport(worker.Temp, plan); err != nil {
		return nil, err
	}
	env, err := worker.environment(base)
	if err != nil {
		return nil, err
	}
	// The same values previously compiled into Codex tool policy now apply to
	// the actual Pi native process environment. No proxy/trust setting is global.
	env = setEnvironmentValues(env, map[string]string{"HTTPS_PROXY": auth.broker.address, "HTTP_PROXY": auth.broker.address, "NO_PROXY": "", "SSL_CERT_FILE": auth.broker.caPath, "GIT_SSL_CAINFO": auth.broker.caPath, "GH_CONFIG_DIR": auth.broker.configDir})
	sandbox := "workspace-write"
	if mode == "full_access" {
		sandbox = "danger-full-access"
	}
	probe, expires, err := auth.readinessScript(roots, "origin", sandbox)
	if err != nil {
		return nil, err
	}
	complete = true
	return &TerminalAuthentication{auth: auth, Environment: terminaltools.ToolEnvironment(env), Probe: probe, ExpiresAt: expires, WriteRoots: append(append([]string{}, roots...), filepath.Clean(plan.Directory))}, nil
}

func rootsAt(roots []string, index int) string {
	if index >= len(roots) {
		return ""
	}
	return roots[index]
}
func (a *TerminalAuthentication) Close() bool { return a != nil && a.auth.Close() }
