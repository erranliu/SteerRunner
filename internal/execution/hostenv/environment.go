// Package hostenv prepares trusted execution-host environments without resolving credentials.
package hostenv

import "strings"

const ToolNames = "PATH PATHEXT SYSTEMROOT SYSTEMDRIVE COMSPEC WINDIR TEMP TMP USERPROFILE HOME APPDATA LOCALAPPDATA PROGRAMFILES PROGRAMFILES(X86) PROGRAMDATA CODEX_HOME GH_HOST GH_CONFIG_DIR GH_TOKEN GITHUB_TOKEN GH_ENTERPRISE_TOKEN GITHUB_ENTERPRISE_TOKEN GH_PROMPT_DISABLED GIT_TERMINAL_PROMPT GCM_INTERACTIVE GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_NOSYSTEM GIT_SSH GIT_SSH_COMMAND SSH_AUTH_SOCK SSH_AGENT_PID HTTPS_PROXY HTTP_PROXY ALL_PROXY NO_PROXY SSL_CERT_FILE SSL_CERT_DIR GIT_SSL_CAINFO GIT_SSL_CAPATH STEERLINE_PROGRESS_FILE STEERLINE_RESULT_FILE"

func blocked(name string) bool {
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING", "ORCA_ENVIRONMENT", "STEERRUNNER_REMOTE_MODEL_BRIDGE_TOKEN"} {
		if name == key {
			return true
		}
	}
	return false
}

// Development preserves host authentication and model configuration in memory.
func Development(base []string) []string {
	values := map[string]string{}
	order := []string{}
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		name := strings.ToUpper(key)
		if !ok || key == "" || blocked(name) {
			continue
		}
		if _, seen := values[name]; !seen {
			order = append(order, name)
		}
		values[name] = value
	}
	for _, entry := range []string{"GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1", "GCM_INTERACTIVE=Never"} {
		key, value, _ := strings.Cut(entry, "=")
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = value
	}
	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, key+"="+values[key])
	}
	return out
}

// Tools excludes model-provider secrets and control credentials from generic tools.
func Tools(base []string) []string {
	allowed := map[string]bool{}
	for _, name := range strings.Fields(ToolNames) {
		allowed[name] = true
	}
	out := []string{}
	for _, entry := range Development(base) {
		key, _, _ := strings.Cut(entry, "=")
		if allowed[key] || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") || key == "GIT_CONFIG_COUNT" {
			out = append(out, entry)
		}
	}
	return out
}
