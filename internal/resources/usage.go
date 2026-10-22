package resources

func UsageCapabilities() any {
	matrix := []map[string]any{}
	for _, target := range []string{"local", "orca_environment"} {
		for _, agent := range []string{"codex", "pi", "dsh"} {
			supported := target == "local" || agent == "codex"
			reason := ""
			if !supported {
				reason = "PRODUCTION_BINDING_UNSUPPORTED"
			}
			matrix = append(matrix, map[string]any{"adapter": "orca-terminal", "target": target, "agent": agent, "parser": true, "file_transport": true, "collection": supported, "production_run_binding": supported, "reason": reason})
		}
	}
	return map[string]any{"schema": "run-usage/v1", "scope": "main_session", "query_only": true, "matrix": matrix, "platforms": []string{"windows/amd64"}, "support_scope": "implementation; live qualification is recorded separately", "remote_log_access_default": "disabled", "directory_registration_read_only_acl": false, "automatic_remote_ttl": false, "max_source_bytes": 64 << 20, "max_decoded_bytes": 256 << 20, "max_record_bytes": 4 << 20, "max_read_chunk_bytes": 512 << 10, "collection_pass_seconds": 30}
}

func CostCapabilities() any {
	return map[string]any{"schema": "run-cost-estimate/v1", "usage_contract": "run-usage/v1", "query_only": true, "independent": true, "basis": "api_equivalent", "pricing_policy": "explicit_absolute_json_file; no built-in prices or network fetch"}
}
