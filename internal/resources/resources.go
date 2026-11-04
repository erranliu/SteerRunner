// Package resources contains the resources shipped inside the executable.
package resources

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	model "github.com/erranliu/SteerRunner/internal/execution/model"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	parser "github.com/erranliu/SteerRunner/internal/textanalysis"
	"io/fs"
	"runtime"
	"sort"
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
		"recovery": map[string]any{"state_format": "steerrunner-state/v8", "same_build_restart": true, "resource_rebuild": false, "upgrade": "drain original resources and unresolved operations with the original executable; use a fresh state directory", "manual_reconciliation": "resume queries original operations for at most 30 seconds; no replacement resources"},
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
		"model_input_contract": ModelInputContract(),
		"stop_evidence":        map[string]any{"run_property": "stop_evidence_scope", "frozen": true, "schema": json.RawMessage(Read("stop-evidence.schema.json")), "orca_terminal": "original runtime/worktree/local-host terminal identity verified exited or absent in a complete inventory; native local and environment contracts; not all descendants or host credentials", "orca_local_codex_resources": "original Orca business/control terminals and registered local helper/app-server roots; explicit managed local Codex contract; not all descendants or host credentials", "orca_agent_session": "original native session close acknowledged at most once; settlement is session-scoped, not detached processes or host cleanup"},
		"orca_execution_contracts": map[string]any{
			"structured_execute": map[string]any{
				"agent": "codex", "workflow": contracts.Workflow{ID: "execute", Version: 1}, "target_kind": "local", "target_kinds": []string{"local", "orca_environment"}, "access_mode": "full_access",
				"input":               "atomic create.firstMessage; one exact text block; explicit model/effort selection; no terminal input or helper",
				"result":              "all complete assistant messages in the original turn, in journal order; public API does not identify provider final phase",
				"stop_evidence_scope": orca.AgentSessionStopScope, "session_close_settles_scope": true, "additional_resource_cleanup": "unverified; not a workflow gate", "stop": "original close at most once; successful acknowledgement settles the original session scope for new runs; previous paired profiles retain unverified stop",
				"usage":       "local retains its existing unreported contract; remote uses original StageInput/native turn and public structuredSession source with existing scoped log policy/reader/parser/journal; conflicts remain unverified",
				"result_file": map[string]string{"local": contracts.ResultFileContract, "orca_environment": contracts.ResultFileContract},
			},
			"structured_development": map[string]any{
				"agent": "codex", "workflow": contracts.Workflow{ID: "development", Version: 2}, "access_mode": "full_access",
				"target_kinds": []string{"local", "orca_environment"}, "selection": "default Codex launch, with no explicit executable, args, or runtime executable",
				"input":               "one original session with atomic first message and one structured text block per authorized follow-up",
				"result":              "last complete assistant message from the verified original turn",
				"stop_evidence_scope": orca.AgentSessionStopScope, "session_close_settles_scope": true, "stop": "original close at most once; successful acknowledgement settles the original session scope; additional resource cleanup unverified",
				"usage":       "each original command and native turn from the bound structured session",
				"result_file": contracts.ResultFileContract,
			},
			"structured_audited": map[string]any{
				"agent": "codex", "workflow": contracts.Workflow{ID: "development-audited", Version: 2}, "access_mode": "full_access",
				"target_kinds": []string{"local"}, "selection": "Orca's structured executor with explicit execution and audit models",
				"result":              "unique assistant message qualified by the original stage contract",
				"actual_model":        "original turn model and effort from the bound audited source",
				"workspace_audit":     "original managed Git checkpoint and existing publication validation",
				"stop_evidence_scope": orca.AgentSessionStopScope, "session_close_settles_scope": true,
				"result_file": contracts.ResultFileSnapshotContract,
			},
			"native": map[string]any{
				"agent": "codex", "workflow": contracts.Workflow{ID: "development", Version: 2}, "access_mode": "full_access",
				"target_kinds": []string{"local", "orca_environment"}, "selection": "explicit agent_launch executable or nonempty args, with no managed runtime executable; existing frozen terminal runs",
				"stop_evidence_scope": "orca_terminal", "actual_model": "unverified_launch_preference_only", "workspace_audit": "unreported",
				"working_start": "independent original unique-input query; no stream coverage; completion requires separate full original evidence",
			},
			"retained_managed_local": map[string]any{
				"selection":                 "explicit nonempty agent_launch.runtime_executable; existing agent/workflow gates still apply",
				"consumers":                 []string{"explicit ordinary Codex development", "Pi managed tools and evidence"},
				"codex_stop_evidence_scope": orca.LocalCodexStopScope, "qualification": "existing managed contract; does not inherit native qualification",
			},
			"historical_records": "original profile/state read, reconciliation and drain; no reinterpretation or automatic migration",
			"other_combinations": "existing admission and evidence gates",
		},
		"execution_wait":            map[string]any{"contract": b.ExecutionWaitContract, "same_snapshot": true, "run_property": "execution_wait", "schema": json.RawMessage(Read("execution-wait.schema.json"))},
		"run_observation":           map[string]any{"contract": contracts.RunObservationContract, "same_snapshot": true, "max_events": 500},
		"recovery":                  map[string]any{"state_format": "steerrunner-state/v8", "same_build_restart": true, "resource_rebuild": false, "upgrade": "drain original resources and unresolved operations with the original executable; use a fresh state directory", "manual_reconciliation": "resume queries original operations for at most 30 seconds; no replacement resources"},
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
		"offline_inspect":           map[string]any{"command": "inspect --offline", "read_only": true, "writer_absent": true, "starts_workers": false},
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
		"generate":              map[string]any{"file_materials": map[string]any{"contract": "frozen_private_files/v1", "input_property": "generate.materials", "kind": "file", "manifest": "run.materials/content_result.file_materials", "max_files": contracts.MaxGenerateFileMaterials, "max_bytes": contracts.MaxGenerateFileMaterialBytes}, "workflow": "generate/1", "repository": map[string]any{"input_property": "generate.repository", "source": "existing_local_git_objects", "commit": "full_commit_object_id", "snapshot": "private_regular_files_without_git", "selection": "explicit_file_paths_or_whole_tree", "max_manifest_bytes": contracts.MaxGenerateRepositoryManifestBytes, "max_files": contracts.MaxGenerateRepositoryFiles, "max_bytes": contracts.MaxGenerateRepositoryBytes, "unsupported": []string{"symlinks", "submodules", "unsafe_or_case_colliding_paths"}}, "result": "inspect/history: content_result.json", "source": "repository_source: commit/tree/files/blob/sha256/size_bytes", "max_result_utf8_bytes": contracts.MaxGenerateResultBytes, "output_schema": "caller_defined_json_schema_2020_12", "replay": "original_request_id_only_no_unknown_retry"},
		"capability_catalog":    Catalog(),
		"cli_schema":            json.RawMessage(Read("cli.schema.json")),
		"commands":              []string{"progress-file", "result-file", "run-usage", "run-usage-events", "estimate-run-cost", "version", "service start", "service status", "service stop", "service verify-stopped", "capabilities", "inspect", "submit", "last-message", "open-native-session", "observe", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"},
		"result_acquisition_configuration_schema": json.RawMessage(Read("result-acquisition.schema.json")),
		"parser_support": map[string]any{"supported": parserSupported, "profile": ra.ParserProfile, "reason": parserReason, "readiness": "CLI_existing_login; in_memory_task; internal_process_cleanup", "empty_tool_guarantee": false},
	}
}
