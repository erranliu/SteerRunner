// Package resources contains the resources shipped inside the executable.
package resources

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"runtime"
	"sort"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	model "github.com/erranliu/SteerRunner/internal/execution/model"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

var Version = "0.2.0-dev.4"
var SourceCommit = "development"

//go:embed assets/*
var assets embed.FS

func Read(name string) []byte {
	value, err := assets.ReadFile("assets/" + name)
	if err != nil {
		panic("missing embedded resource")
	}
	return value
}

func Digest() string {
	return resourceDigest(Catalog())
}

func resourceDigest(catalog CapabilityCatalog) string {
	names, err := fs.Glob(assets, "assets/*")
	if err != nil {
		panic("invalid embedded resource glob")
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		body, err := assets.ReadFile(name)
		if err != nil {
			panic("unreadable embedded resource")
		}
		digest.Write([]byte(name + "\x00"))
		digest.Write(body)
	}
	definition, _ := json.Marshal(workflow.Catalog())
	digest.Write([]byte("compiled-workflow-definitions\x00"))
	digest.Write(definition)
	digest.Write([]byte("model-input-contract\x00"))
	digest.Write(marshal(ModelInputContract()))
	digest.Write([]byte("published-configuration\x00"))
	digest.Write(ConfigurationSchemas())
	digest.Write([]byte("capability-catalog\x00"))
	digest.Write(marshal(catalog))
	return hex.EncodeToString(digest.Sum(nil))
}

func VersionInfo() any {
	return map[string]any{
		"version": Version, "source_commit": SourceCommit,
		"protocols": []string{contracts.Protocol}, "resource_version": 3,
		"resource_digest": Digest(), "platform": "windows/amd64",
		"recovery": RecoveryCapabilities(),
	}
}

func ModelInputContract() any {
	return map[string]any{
		"contract":          model.InputContract,
		"utf8_bytes":        map[string]any{"caller": nil, "runner": model.RunnerInputBytes, "total": nil},
		"measurement":       "all new model-visible text in one native dispatch after framing/newline handling; null byte limits mean no text budget; only Runner-generated text is capped; no truncation, summarization or automatic splitting; checked text is frozen and verified before send",
		"caller":            "body override, instruction, inline materials, schema, answer, feedback; escaped data remains caller",
		"runner":            "default body, labels/separators/newlines, dynamic facts/references, result format, completion, native model/header framing, developer and Runner tool descriptions, parser source text",
		"tool_file_results": "not new input text unless copied into it; existing file channels only, references must resolve in execution workspace",
		"workflow":          workflow.InputContract(), "native": codex.InputContract(), "terminal": orca.InputContract(),
		"parser":    map[string]any{"boundary": parser.ParserBoundary, "format": parser.ParserFormat, "instruction": "original frozen parse state.prompt", "evidence": "complete JSON-encoded original final; unattempted oversized parser uses existing report fallback"},
		"transport": map[string]any{"codex_websocket_max_bytes": 16 << 20, "windows_command_line_max_utf16_with_nul": 32767, "orca": "actual CLI argv protected independently; remote RPC envelope not exposed by CLI"},
	}
}

func Capabilities() any {
	// Repository generation shares generate/1 and the original content result.
	parserSupported, parserReason := ra.ParserSupport()
	viewerAdapters := []string{}
	if runtime.GOOS == "windows" {
		viewerAdapters = append(viewerAdapters, "codex-app-server")
	}
	return map[string]any{
		"model_input_contract":      ModelInputContract(),
		"execution_wait":            map[string]any{"contract": b.ExecutionWaitContract, "same_snapshot": true, "run_property": "execution_wait", "schema": json.RawMessage(Read("execution-wait.schema.json"))},
		"run_observation":           map[string]any{"contract": contracts.RunObservationContract, "same_snapshot": true, "max_events": 500},
		"recovery":                  RecoveryCapabilities(),
		"run_usage":                 UsageCapabilities(),
		"run_cost":                  CostCapabilities(),
		"run_usage_schema":          json.RawMessage(Read("run-usage.schema.json")),
		"run_usage_events_schema":   json.RawMessage(Read("run-usage-events.schema.json")),
		"run_cost_estimate_schema":  json.RawMessage(Read("run-cost-estimate.schema.json")),
		"run_pricing_policy_schema": json.RawMessage(Read("run-pricing-policy.schema.json")),
		"service_start_schema":      json.RawMessage(Read("service-start.schema.json")),
		"progress_file":             ProgressCapabilities(),
		"progress_file_schema":      json.RawMessage(Read("progress-file.schema.json")),
		"result_file":               ResultFileCapabilities(),
		"result_file_schema":        json.RawMessage(Read("result-file.schema.json")),
		"run_evidence":              json.RawMessage(Read("run-evidence.capabilities.json")),
		"run_evidence_schema":       json.RawMessage(Read("run-evidence.schema.json")),
		"native_session_viewer":     map[string]any{"adapters": viewerAdapters},
		"instance_binding": map[string]any{"expected_instance_id": true, "full_binding": map[string]any{
			"contract": contracts.ServiceBindingContract, "before_effects": true, "no_implicit_start": true, "commands": contracts.ServiceBindingCommands(),
		}},
		"protocol": contracts.Protocol, "version": Version,
		"resource_version": 3, "resource_digest": Digest(),
		"execution":             map[string]any{"status": "implemented", "adapters": []string{"codex-app-server", "orca-terminal"}, "readiness": "requires_per_run_native_verification"},
		"workflow":              json.RawMessage(Read("development.json")),
		"workflows":             workflow.Catalog(),
		"configuration_schemas": ConfigurationSchemas(),
		"generate":              map[string]any{"workflow": "generate/1", "repository": map[string]any{"input_property": "generate.repository", "source": "existing_local_git_objects", "commit": "full_commit_object_id", "snapshot": "private_regular_files_without_git", "selection": "explicit_file_paths_or_whole_tree", "max_manifest_bytes": contracts.MaxGenerateRepositoryManifestBytes, "max_files": contracts.MaxGenerateRepositoryFiles, "max_bytes": contracts.MaxGenerateRepositoryBytes, "unsupported": []string{"symlinks", "submodules", "unsafe_or_case_colliding_paths"}}, "result": "inspect/history: content_result.json", "source": "repository_source: commit/tree/files/blob/sha256/size_bytes", "max_result_utf8_bytes": contracts.MaxGenerateResultBytes, "output_schema": "caller_defined_json_schema_2020_12", "replay": "original_request_id_only_no_unknown_retry"},
		"capability_catalog":    Catalog(),
		"cli_schema":            json.RawMessage(Read("cli.schema.json")),
		"commands":              []string{"progress-file", "result-file", "run-usage", "run-usage-events", "estimate-run-cost", "version", "service start", "service status", "service stop", "service verify-stopped", "service verify-takeover", "capabilities", "inspect", "submit", "last-message", "open-native-session", "observe", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"},
		"result_acquisition_configuration_schema": json.RawMessage(Read("result-acquisition.schema.json")),
		"parser_support": map[string]any{"supported": parserSupported, "profile": ra.ParserProfile, "reason": parserReason, "readiness": "requires_per_acquisition_native_verification", "empty_tool_guarantee": false},
	}
}

func RecoveryCapabilities() contracts.RecoverySupport {
	return contracts.RecoverySupport{ReadableStateFormats: []string{"steerrunner-state/v5"}, WritableStateFormat: "steerrunner-state/v5", OperationVersions: []int{1}, ExecutionRecordVersions: []int{1}, OwnerProtocol: "steerrunner-execution-owner/v1", PendingStops: true,
		ImplementationCodecs: map[string][]int{"codex.worker-profile": {3}, "codex.app-server-binding": {1, 2}, "codex.policy-resources": {3, 4}, "orca.terminal-profile": {3, 4}, "orca.terminal-state": {6, 7}}}
}
