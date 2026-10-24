package resources

import "github.com/erranliu/SteerRunner/internal/contracts"

func progressInputSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"initial_content_base64"}, "properties": map[string]any{"initial_content_base64": map[string]any{"type": "string", "contentEncoding": "base64", "maxLength": 87384}}, "description": "Canonical base64, decoded length <= 65536 bytes. Writable local Codex and local/remote Orca development workspaces; no caller path."}
}
func ProgressCapabilities() any {
	return map[string]any{"contract": contracts.ProgressContract, "command": "progress-file --run-id R --json", "submit_property": "progress_file", "environment_variable": contracts.ProgressEnvironment, "max_bytes": contracts.MaxProgressBytes, "encoding": "base64", "digest": "sha256", "version": "persistent independent monotonic content version; does not change Run revision", "delivery": "latest stable complete snapshot; intermediate writes may coalesce", "supported": map[string]any{"adapters": []string{"codex-app-server", "orca-terminal"}, "target_kinds": []string{"local", "orca_environment"}, "workflows": []string{"development/2", "development-audited/2"}}, "path_boundary": "fixed original host/worktree/relative file; no arbitrary caller paths or directory scan; adversarial link replacement races are outside this contract", "retention": "last snapshot retained after stop; state-dir required", "statuses": []string{"disabled", "pending", "initializing", "observing", "unavailable", "closed", "detached"}, "diagnostics": []string{"binding_invalid", "workspace_unavailable", "directory_unavailable", "file_missing", "read_failed", "size_exceeded", "file_changing", "initialization_unverified", "transport_unavailable"}}
}
