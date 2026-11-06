package orcatransport

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
