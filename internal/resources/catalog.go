package resources

import (
	"encoding/json"
	"slices"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

// Combination describes implemented routes, never an inventory of a host.
type Combination struct {
	Adapter      string             `json:"adapter"`
	TargetKind   string             `json:"target_kind"`
	Agent        string             `json:"agent"`
	Workflow     contracts.Workflow `json:"workflow"`
	AccessMode   string             `json:"access_mode"`
	Operations   []string           `json:"operations"`
	Schema       string             `json:"schema"`
	Requirements []string           `json:"requirements"`
}
type CapabilityCatalog struct {
	Version         int               `json:"version"`
	Combinations    []Combination     `json:"combinations"`
	ValidationKinds map[string]string `json:"validation_kinds"`
}

func Catalog() CapabilityCatalog {
	result := CapabilityCatalog{Version: 1, Combinations: []Combination{}, ValidationKinds: map[string]string{
		"run-configuration": "/configuration_schemas/run_configuration", "run": "/configuration_schemas/run_input",
	}}
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		for _, target := range []string{"local", "orca_environment"} {
			for _, agent := range []string{"codex", "pi", "dsh"} {
				if adapter == "codex-app-server" && (target != "local" || agent != "codex") {
					continue
				}
				for _, mode := range []string{"sandbox", "full_access"} {
					settings := map[string]any{"access_mode": mode}
					if adapter == "orca-terminal" {
						settings["agent"] = agent
					}
					execution := contracts.Execution{Adapter: adapter, Target: contracts.Target{Kind: target}, Settings: marshal(settings)}
					if target == "orca_environment" {
						execution.Target.Environment = "configured-environment"
					}
					if contracts.ValidateExecution(execution) != nil {
						continue
					}
					requirements := []string{"per_execution_native_verification", "execution_host_authentication", "native_codex_0.159.2_or_newer_permissions"}
					if target == "orca_environment" {
						requirements = []string{"configured_orca_environment", "execution_host_native_login", "native_launch_parameters_requested_only"}
					}
					for _, profile := range workflow.Catalog() {
						if !slices.Contains(profile.SupportedAdapters, adapter) {
							continue
						}
						selection := contracts.Workflow{ID: profile.ID, Version: profile.Version}
						candidate := execution
						needed := slices.Clone(requirements)
						if profile.AuditModelRequired {
							selection.AuditModel = &workflow.ModelSettings{Model: "audit-model"}
							values := map[string]any{"access_mode": mode, "model": "execution-model"}
							if adapter == "orca-terminal" {
								values = map[string]any{"agent": agent, "access_mode": mode, "agent_launch": map[string]any{"runtime_executable": "configured-helper.exe", "model": "execution-model"}}
							}
							candidate.Settings = marshal(values)
							needed = append(needed, "explicit_execution_and_audit_models")
						}
						if contracts.ValidateRunConfiguration(marshal(contracts.RunConfiguration{Execution: candidate, Workflow: selection})) != nil {
							continue
						}
						// Model examples are only used to evaluate the production route gate.
						selection.AuditModel = nil
						result.Combinations = append(result.Combinations, Combination{adapter, target, agent, selection, mode, []string{"submit", "reply", "resume", "stop"}, "/configuration_schemas/run_configuration", needed})
					}

				}
			}
		}
	}
	for _, id := range []string{"conversation", "generate"} {
		selection := contracts.Workflow{ID: id, Version: 1}
		operations := []string{"submit", "inspect", "history", "stop", "resume"}
		if id == "conversation" {
			operations = append(operations, "message", "reply", "interrupt", "close")
		}
		result.Combinations = append(result.Combinations, Combination{"codex-app-server", "local", "codex", selection, "read_only", operations, "/configuration_schemas/" + id + "_input", []string{"per_execution_native_verification", "native_codex_0.159.2_or_newer_permissions", "self_contained_schema_only"}})
	}
	return result
}

func marshal(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

// Declares is for caller/test selection only, never for production admission.
func (c CapabilityCatalog) Declares(execution contracts.Execution, selection *contracts.Workflow, operation string) bool {
	if selection == nil {
		return false
	}
	agent, mode := "codex", "sandbox"
	var values struct {
		Agent      string `json:"agent"`
		AccessMode string `json:"access_mode"`
	}
	if json.Unmarshal(execution.Settings, &values) != nil {
		return false
	}
	if execution.Adapter == "orca-terminal" && values.Agent != "" {
		agent = values.Agent
	}
	if values.AccessMode != "" {
		mode = values.AccessMode
	}
	for _, entry := range c.Combinations {
		if entry.Adapter != execution.Adapter || entry.TargetKind != execution.Target.Kind || entry.Agent != agent || entry.AccessMode != mode || !slices.Contains(entry.Operations, operation) {
			continue
		}
		if entry.Workflow.ID == selection.ID && entry.Workflow.Version == selection.Version {
			return true
		}
	}
	return false
}

type CapabilitySource struct {
	Kind             string `json:"kind"`
	Version          string `json:"version"`
	Protocol         string `json:"protocol"`
	ExecutableSHA256 string `json:"executable_sha256"`
	ResourceDigest   string `json:"resource_digest"`
	InstanceID       string `json:"instance_id,omitempty"`
}

func CapabilitiesFrom(source CapabilitySource) map[string]any {
	result := Capabilities().(map[string]any)
	result["capability_source"] = source
	return result
}
