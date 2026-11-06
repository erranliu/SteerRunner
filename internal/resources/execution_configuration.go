package resources

import (
	"bytes"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ExecutionConfiguration publishes the same configuration shapes used by the
// production gate. Combinations remain owned by the existing capability catalog.
func ExecutionConfiguration() any {
	var schemas map[string]any
	_ = json.Unmarshal(ConfigurationSchemas(), &schemas)
	run := schemas["run_configuration"].(map[string]any)
	execution := run["properties"].(map[string]any)["execution"]
	stripSchemaIDs(execution)
	selector := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "version"}, "properties": map[string]any{"id": map[string]any{"type": "string", "minLength": 1}, "version": map[string]any{"type": "integer", "minimum": 1}}}
	text := map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}
	target := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"kind"}, "properties": map[string]any{"kind": map[string]any{"enum": []string{"local", "orca_environment"}}, "environment": map[string]any{"type": "string"}}}
	view := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"adapter", "target", "agent", "access_mode"}, "properties": map[string]any{"adapter": text, "target": target, "agent": text, "access_mode": map[string]any{"enum": []string{"full_access", "read_only"}}, "model": text, "reasoning_effort": text}}
	executionRef := map[string]any{"$ref": "#/$defs/execution"}
	variants := []any{}
	for _, operation := range []string{"describe", "derive", "compose"} {
		props := map[string]any{"operation": map[string]any{"const": operation}, "workflow": selector}
		required := []string{"operation", "workflow"}
		if operation == "compose" {
			props["workflow"] = schemas["workflow"]
			stripSchemaIDs(props["workflow"])
			props["selection"] = view
			props["base_execution"] = executionRef
			props["unset"] = map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{"enum": []string{"model", "reasoning_effort"}}}
			required = append(required, "selection")
		} else {
			props["execution"] = executionRef
			required = append(required, "execution")
			if operation == "derive" {
				props["access_mode"] = map[string]any{"enum": []string{"full_access", "read_only"}}
				required = append(required, "access_mode")
			}
		}
		variants = append(variants, map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": props})
	}
	request := map[string]any{"$defs": map[string]any{"execution": execution}, "oneOf": variants}
	digest := map[string]any{"type": "string", "pattern": "^[a-f0-9]{64}$"}
	identity := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"contract", "source_execution_digest", "context"}, "properties": map[string]any{"contract": map[string]any{"const": contracts.ExecutionIdentityContract}, "source_execution_digest": digest, "context": executionRef}}
	source := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"version", "executable_sha256", "resource_digest"}, "properties": map[string]any{"version": text, "executable_sha256": digest, "resource_digest": digest}}
	response := map[string]any{"$defs": map[string]any{"execution": execution}, "type": "object", "additionalProperties": false, "required": []string{"contract", "operation", "input_digest", "source", "view"}, "properties": map[string]any{"contract": map[string]any{"const": contracts.ExecutionConfigurationContract}, "operation": map[string]any{"enum": []string{"describe", "derive", "compose"}}, "input_digest": digest, "source": source, "view": view, "execution": executionRef, "identity": identity, "source_identity": identity}}
	responseCases := []any{}
	for _, operation := range []string{"describe", "derive", "compose"} {
		required := []string{"identity"}
		forbidden := []string{"execution", "source_identity"}
		if operation == "derive" {
			required = []string{"execution", "source_identity"}
			forbidden = []string{"identity"}
		}
		if operation == "compose" {
			required = []string{"execution", "identity"}
			forbidden = []string{"source_identity"}
		}
		props := map[string]any{"operation": map[string]any{"const": operation}}
		for _, field := range forbidden {
			props[field] = false
		}
		responseCases = append(responseCases, map[string]any{"properties": props, "required": required})
	}
	response["oneOf"] = responseCases
	return map[string]any{"contract": contracts.ExecutionConfigurationContract, "command": "execution-config", "query_only": true, "requires_service": false, "operations": []string{"describe", "derive", "compose"}, "request_schema": request, "response_schema": response, "combination_source": "/capability_catalog/combinations", "reasoning_efforts": map[string][]string{"codex": contracts.ReasoningEfforts("codex"), "pi": contracts.ReasoningEfforts("pi")}, "editable_fields": []string{"adapter", "target", "agent", "model", "reasoning_effort", "access_mode"}, "derive_workflows": []contracts.Workflow{{ID: "execute", Version: 1}, {ID: "generate", Version: 1}}, "limits": map[string]int{"input_bytes": contracts.ExecutionConfigurationLimit, "output_bytes": contracts.ExecutionConfigurationLimit, "timeout_ms": 30000}}
}

// ResolveExecutionConfiguration adds the published shape checks to the shared
// semantic gate, so standalone CLI clients and consumers see one strict contract.
func ResolveExecutionConfiguration(raw []byte, source contracts.ExecutionConfigurationSource) (contracts.ExecutionConfigurationResult, *contracts.Fault) {
	result, fault := contracts.ConfigureExecution(raw, source)
	if fault != nil {
		return contracts.ExecutionConfigurationResult{}, fault
	}
	capability := ExecutionConfiguration().(map[string]any)
	if err := validateConfigurationObject(capability["request_schema"], raw); err != nil {
		return contracts.ExecutionConfigurationResult{}, contracts.Fail(2, "invalid_input", "Execution configuration does not satisfy the published schema.")
	}
	return result, nil
}

func validateConfigurationObject(schema any, raw []byte) error {
	encoded, _ := json.Marshal(schema)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("https://runner.invalid/execution-configuration", doc); err != nil {
		return err
	}
	compiled, err := compiler.Compile("https://runner.invalid/execution-configuration")
	if err != nil {
		return err
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		return err
	}
	return compiled.Validate(value)
}
