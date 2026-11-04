package resources

import "github.com/erranliu/SteerRunner/internal/contracts"

func ResultFileCapabilities() any {
	capabilities := map[string]any{"initialization": "optional best effort, one durable attempt per channel before first business input, shared 4-second total window; failure never gates business and initialization is never replayed", "contract": contracts.ResultFileContract, "command": "result-file --run-id R --json", "submit_property": "result_file", "environment_variable": contracts.ResultFileEnvironment, "max_bytes": contracts.MaxResultBytes, "encoding": "base64", "digest": "sha256", "collection": "end_only", "query_reads_file": false, "version": "0 until successful seal, then 1 forever; does not change Run revision", "delivery": "final-only: at most one snapshot after original execution ended and producers stopped; no running collection or query reads; bounded by 2 seconds and lifecycle deadline; failed seal is final; resume never refreshes", "supported": map[string]any{"adapters": []string{"codex-app-server", "orca-terminal"}, "target_kinds": []string{"local", "orca_environment"}, "workflows": []string{"development/2", "development-audited/2", "execute/1"}}, "path_boundary": "fixed original host/worktree/relative file; no arbitrary caller paths or directory scan; adversarial link replacement races are outside this contract", "retention": "sealed success or failure retained after stop and restart; state-dir required", "statuses": []string{"disabled", "pending", "closed"}, "diagnostics": []string{"binding_invalid", "workspace_unavailable", "directory_unavailable", "file_missing", "read_failed", "size_exceeded", "file_changing", "initialization_unverified", "transport_unavailable"}}
	capabilities["contracts"] = map[string]any{
		contracts.ResultFileContract: map[string]any{"delivery": capabilities["delivery"]},
		contracts.ResultFileSnapshotContract: map[string]any{
			"scope":       "previous remote Orca Codex execute/1 profiles only; frozen at admission",
			"delivery":    "one bounded snapshot after durable original business result or first original stop processing, including stop unknown; no guarantee producers stopped or future writes cease",
			"window":      "original first-end identity and 2-second deadline survive restart; a failed or expired attempt closes collection at version 0; query never reads files",
			"observed_at": "actual collection time, not an atomic business-end instant",
		},
	}
	capabilities["contract_selection"] = "response contract is frozen per Run; new local and paired remote Orca Codex execute/1 use producer-stop v1 within the original session scope"
	return capabilities
}
