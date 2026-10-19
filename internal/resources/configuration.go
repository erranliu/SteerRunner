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
	schemas["message_input"] = map[string]any{"type": "object", "required": []string{"expected_revision", "text"}, "additionalProperties": false, "properties": map[string]any{"expected_revision": map[string]any{"type": "integer", "minimum": 1}, "text": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}}}
	schemas["revision_input"] = map[string]any{"type": "object", "required": []string{"expected_revision"}, "additionalProperties": false, "properties": map[string]any{"expected_revision": map[string]any{"type": "integer", "minimum": 1}}}
	// Match existing admission: an explicit empty mode uses the original
	// sandbox default, and a null optional App reference is equivalent to absent.
	// This only corrects publication; it does not tighten legacy write gates.
	for _, name := range []string{"codex-app-server", "orca-terminal"} {
		properties := adapters[name].(map[string]any)["properties"].(map[string]any)
		properties["access_mode"].(map[string]any)["enum"] = []string{"", "sandbox", "full_access"}
	}
	nativeProperties := adapters["codex-app-server"].(map[string]any)["properties"].(map[string]any)
	nativeProperties["access_mode"].(map[string]any)["enum"] = []string{"", "sandbox", "full_access", "read_only"}
	nativeProperties["github_auth"].(map[string]any)["type"] = []string{"object", "null"}
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
				"then": map[string]any{"properties": map[string]any{"settings": map[string]any{"properties": map[string]any{"github_auth": map[string]any{"type": "null"}, "agent_launch": map[string]any{"not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"runtime_executable"}}, map[string]any{"required": []string{"runtime_sha256"}}}}}}}}},
			}}
		}
		choices = append(choices, choice)
	}
	rules := []any{}
	for _, profile := range workflow.Catalog() {
		selection := map[string]any{"properties": map[string]any{"workflow": map[string]any{"properties": map[string]any{"id": map[string]any{"const": profile.ID}, "version": map[string]any{"const": profile.Version}}}}}
		execution := map[string]any{"properties": map[string]any{"adapter": map[string]any{"enum": profile.SupportedAdapters}}}
		if profile.AuditModelRequired {
			execution["properties"].(map[string]any)["target"] = local
			execution["properties"].(map[string]any)["settings"] = map[string]any{"required": []string{"model"}, "properties": map[string]any{"model": map[string]any{"type": "string", "minLength": 1, "pattern": `^\S(?:.*\S)?$`, "not": map[string]any{"pattern": "[\u0000-\u001f\u007f-\u009f]"}}}}
		}
		promptNames := any(map[string]any{"enum": profile.PromptSteps})
		if len(profile.PromptSteps) == 0 {
			promptNames = false
		}
		overrides := any(map[string]any{"propertyNames": promptNames})
		if workflow.Content(profile.ID) {
			overrides = false
			execution["properties"].(map[string]any)["settings"] = map[string]any{"required": []string{"access_mode"}, "properties": map[string]any{"access_mode": map[string]any{"const": "read_only"}, "github_auth": false}}
		} else {
			execution["properties"].(map[string]any)["settings"] = mergeSettingsRestriction(execution["properties"].(map[string]any)["settings"])
		}
		rules = append(rules, map[string]any{"if": selection, "then": map[string]any{"properties": map[string]any{
			"execution": execution,
			"workflow":  map[string]any{"properties": map[string]any{"prompt_overrides": overrides}},
		}}})
	}
	schemas["run_configuration"] = map[string]any{"type": "object", "required": []string{"execution", "workflow"}, "additionalProperties": false,
		"properties": map[string]any{"execution": map[string]any{"oneOf": choices}, "workflow": schemas["workflow"]}, "allOf": rules}
	development := runInputSchema(schemas)
	delete(development, "$id")
	development["properties"].(map[string]any)["conversation"] = false
	development["properties"].(map[string]any)["generate"] = false
	development["properties"].(map[string]any)["workflow"] = map[string]any{"allOf": []any{schemas["workflow"], map[string]any{"properties": map[string]any{"id": map[string]any{"enum": []string{"development", "development-audited"}}}}}}
	schemas["conversation_input"] = contentInputSchema(schemas, "conversation")
	schemas["generate_input"] = contentInputSchema(schemas, "generate")
	schemas["run_input"] = map[string]any{"$id": "steerrunner/run-input/v1", "oneOf": []any{development, schemas["conversation_input"], schemas["generate_input"]}}
	// Embedded adapter schemas occur in every input variant. They are one
	// resource here; repeated nested $id values would create ambiguous resources.
	var combined any
	_ = json.Unmarshal(marshal(schemas["run_input"]), &combined)
	stripSchemaIDs(combined)
	schemas["run_input"] = combined
	raw, err := json.Marshal(schemas)
	if err != nil {
		panic(err)
	}
	return raw
}

func stripSchemaIDs(v any) {
	switch x := v.(type) {
	case map[string]any:
		delete(x, "$id")
		for _, child := range x {
			stripSchemaIDs(child)
		}
	case []any:
		for _, child := range x {
			stripSchemaIDs(child)
		}
	}
}

func mergeSettingsRestriction(existing any) map[string]any {
	return map[string]any{"allOf": []any{existingOrEmpty(existing), map[string]any{"properties": map[string]any{"access_mode": map[string]any{"enum": []string{"", "sandbox", "full_access"}}}}}}
}
func existingOrEmpty(value any) any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
