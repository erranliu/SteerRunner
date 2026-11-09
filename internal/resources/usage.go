package resources

func UsageCapabilities() any {
	matrix := []map[string]any{
		{"adapter": "codex-app-server", "target": "local", "agent": "codex", "parser": true, "file_transport": false, "collection": true, "production_run_binding": true, "reason": ""},
	}
	for _, target := range []string{"local", "orca_environment"} {
		for _, agent := range []string{"codex", "pi"} {
			supported := agent == "codex"
			reason := ""
			if !supported {
				reason = "PRODUCTION_BINDING_UNSUPPORTED"
			}
			matrix = append(matrix, map[string]any{"adapter": "orca-terminal", "target": target, "agent": agent, "parser": true, "file_transport": true, "collection": supported, "production_run_binding": supported, "reason": reason})
		}
	}
	return map[string]any{"schema": "run-usage/v1", "same_snapshot": true, "events_schema": "run-usage-events/v1", "max_events": 500, "scope": "main_session", "query_only": true, "matrix": matrix, "platforms": []string{"windows/amd64"}, "support_scope": "implementation; live qualification is recorded separately", "remote_log_access_default": "verified_session", "directory_registration_read_only_acl": false, "automatic_remote_ttl": false, "max_source_bytes": 64 << 20, "max_decoded_bytes": 256 << 20, "max_record_bytes": 4 << 20, "max_read_chunk_bytes": 512 << 10, "collection_pass_seconds": 30}
}
