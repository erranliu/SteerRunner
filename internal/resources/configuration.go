package resources

import (
	"encoding/json"

	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

// ConfigurationSchemas publishes a self-contained combination contract. The
// workflow restrictions come from the same catalog used by production admission.
func ConfigurationSchemas() json.RawMessage {
	var schemas map[string]any
	if err := json.Unmarshal(Read("settings.schema.json"), &schemas); err != nil {
		panic(err)
	}
	adapters := schemas["adapters"].(map[string]any)
	// An explicit empty mode retains the sandbox default. Retired App fields
	// remain absent from the strict adapter schemas, including null values.
	for _, name := range []string{"codex-app-server", "orca-terminal"} {
		properties := adapters[name].(map[string]any)["properties"].(map[string]any)
		properties["access_mode"].(map[string]any)["enum"] = []string{"", "sandbox", "full_access"}
	}
	local := map[string]any{"type": "object", "required": []string{"kind"}, "additionalProperties": false,
		"properties": map[string]any{"kind": map[string]any{"const": "local"}, "environment": map[string]any{"const": ""}}}
	remote := map[string]any{"type": "object", "required": []string{"kind", "environment"}, "additionalProperties": false,
		"properties": map[string]any{"kind": map[string]any{"const": "orca_environment"}, "environment": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}}}
	choices := []any{}
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		target := any(local)
		if adapter == "orca-terminal" {
			target = map[string]any{"oneOf": []any{local, remote}}
		}
		choice := map[string]any{"type": "object", "required": []string{"adapter", "target", "settings"}, "additionalProperties": false,
			"properties": map[string]any{"adapter": map[string]any{"const": adapter}, "target": target, "settings": adapters[adapter]}}
		if adapter == "orca-terminal" {
			choice["allOf"] = []any{map[string]any{
				"if":   map[string]any{"properties": map[string]any{"target": remote}},
				"then": map[string]any{"properties": map[string]any{"settings": map[string]any{"properties": map[string]any{"agent_launch": map[string]any{"not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"runtime_executable"}}, map[string]any{"required": []string{"runtime_sha256"}}}}}}}}},
			}}
		}
		choices = append(choices, choice)
	}
	rules := []any{}
	for _, profile := range workflow.Catalog() {
		selection := map[string]any{"properties": map[string]any{"workflow": map[string]any{"properties": map[string]any{"id": map[string]any{"const": profile.ID}, "version": map[string]any{"const": profile.Version}}}}}
		execution := map[string]any{"properties": map[string]any{"adapter": map[string]any{"enum": profile.SupportedAdapters}}}
		if profile.AuditModelRequired {
			model := map[string]any{"type": "string", "minLength": 1, "pattern": `^\S(?:.*\S)?$`, "not": map[string]any{"pattern": "[\u0000-\u001f\u007f-\u009f]"}}
			execution["allOf"] = []any{
				map[string]any{"if": map[string]any{"properties": map[string]any{"adapter": map[string]any{"const": "codex-app-server"}}}, "then": map[string]any{"properties": map[string]any{"settings": map[string]any{"required": []string{"model"}, "properties": map[string]any{"model": model}}}}},
				map[string]any{"if": map[string]any{"properties": map[string]any{"adapter": map[string]any{"const": "orca-terminal"}}}, "then": map[string]any{"properties": map[string]any{"target": local, "settings": map[string]any{"required": []string{"agent_launch"}, "properties": map[string]any{"agent": map[string]any{"const": "codex"}, "agent_launch": map[string]any{"required": []string{"model", "runtime_executable"}, "properties": map[string]any{"model": model, "runtime_executable": map[string]any{"type": "string", "minLength": 1}}}}}}}},
			}
		}
		rules = append(rules, map[string]any{"if": selection, "then": map[string]any{"properties": map[string]any{
			"execution": execution,
			"workflow":  map[string]any{"properties": map[string]any{"prompt_overrides": map[string]any{"propertyNames": map[string]any{"enum": profile.PromptSteps}}}},
		}}})
	}
	schemas["run_configuration"] = map[string]any{"type": "object", "required": []string{"execution", "workflow"}, "additionalProperties": false,
		"properties": map[string]any{"execution": map[string]any{"oneOf": choices}, "workflow": schemas["workflow"]}, "allOf": rules}
	schemas["run_input"] = runInputSchema(schemas)
	raw, err := json.Marshal(schemas)
	if err != nil {
		panic(err)
	}
	return raw
}
