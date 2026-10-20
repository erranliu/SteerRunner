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
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

var Version = "0.2.0-dev.3-r2"
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
	digest.Write([]byte("published-configuration\x00"))
	digest.Write(ConfigurationSchemas())
	digest.Write([]byte("capability-catalog\x00"))
	digest.Write(marshal(catalog))
	return hex.EncodeToString(digest.Sum(nil))
}

func VersionInfo() any {
	return map[string]any{
		"version": Version, "source_commit": SourceCommit,
		"protocols": []string{contracts.Protocol}, "resource_version": 2,
		"resource_digest": Digest(), "platform": "windows/amd64",
		"recovery": RecoveryCapabilities(),
	}
}

func Capabilities() any {
	parserSupported, parserReason := ra.ParserSupport()
	viewerAdapters := []string{}
	if runtime.GOOS == "windows" {
		viewerAdapters = append(viewerAdapters, "codex-app-server")
	}
	return map[string]any{
		"recovery":                   RecoveryCapabilities(),
		"service_start_schema":       json.RawMessage(Read("service-start.schema.json")),
		"run_evidence":               json.RawMessage(Read("run-evidence.capabilities.json")),
		"run_evidence_schema":        json.RawMessage(Read("run-evidence.schema.json")),
		"environment_check":          json.RawMessage(Read("environment-check.capabilities.json")),
		"environment_check_fixtures": json.RawMessage(Read("environment-check.fixtures.json")),
		"environment_check_schema":   json.RawMessage(Read("environment-check.schema.json")),
		"native_session_viewer":      map[string]any{"adapters": viewerAdapters},
		"instance_binding":           map[string]bool{"expected_instance_id": true},
		"protocol":                   contracts.Protocol, "version": Version,
		"resource_version": 2, "resource_digest": Digest(),
		"execution":             map[string]any{"status": "implemented", "adapters": []string{"codex-app-server", "orca-terminal"}, "readiness": "requires_per_run_native_verification"},
		"workflow":              json.RawMessage(Read("development.json")),
		"workflows":             workflow.Catalog(),
		"configuration_schemas": ConfigurationSchemas(),
		"capability_catalog":    Catalog(),
		"cli_schema":            json.RawMessage(Read("cli.schema.json")),
		"commands":              []string{"version", "service start", "service status", "service stop", "service verify-stopped", "service verify-takeover", "capabilities", "environment-check start", "environment-check inspect", "environment-check stop", "inspect", "submit", "last-message", "open-native-session", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"},
		"result_acquisition_configuration_schema": json.RawMessage(Read("result-acquisition.schema.json")),
		"parser_support": map[string]any{"supported": parserSupported, "profile": ra.ParserProfile, "reason": parserReason, "readiness": "requires_per_acquisition_native_verification", "empty_tool_guarantee": false},
	}
}

func RecoveryCapabilities() contracts.RecoverySupport {
	return contracts.RecoverySupport{ReadableStateFormats: []string{"steerrunner-state/v3", "steerrunner-state/v4"}, WritableStateFormat: "steerrunner-state/v4", OperationVersions: []int{1}, ExecutionRecordVersions: []int{1}, OwnerProtocol: "steerrunner-execution-owner/v1", PendingStops: true,
		ImplementationCodecs: map[string][]int{"codex.worker-profile": {3}, "codex.app-server-binding": {1, 2}, "codex.policy-resources": {3, 4}, "orca.terminal-profile": {3, 4}, "orca.terminal-state": {6}}}
}
