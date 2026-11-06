package orcatransport

import (
	"context"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const LocalCodexStopScope = "orca_local_codex_resources"
const LocalCodexStopDiagnostic = "Stopped the registered original Orca terminals and local Codex helper/root processes; all descendant processes and execution-host credentials were not verified or revoked."

// StopScope is fixed by accepted configuration, including before resources exist.
func StopScope(p Profile, agent string) string {
	if p.nativeTerminalContract() {
		return "orca_terminal"
	}
	if agent == "codex" && p.Terminal == nil && p.RuntimeExecutable != "" {
		return LocalCodexStopScope
	}
	return ""
}

func (l *Lifecycle) localCodexScope() bool { return l.frozenRecord().localCodexScope() }

func (l *Lifecycle) localStopObjects(s b.LifecycleSnapshot, state State) ([]b.ProcessIdentity, []b.ProcessIdentity, error) {
	return l.frozenRecord().localStopObjects(s, state)
}

func (l *Lifecycle) verifyLocalCodexStopped(ctx context.Context, s b.LifecycleSnapshot) error {
	return l.frozenRecord().verifyLocalCodexStopped(ctx, s)
}
