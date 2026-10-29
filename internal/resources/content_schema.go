package resources

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"reflect"
)

func contentInputSchema(schemas map[string]any, id string) map[string]any {
	shape := any(shapeSchema(reflect.TypeFor[contracts.GenerateInput]()))
	if id == "conversation" {
		shape = shapeSchema(reflect.TypeFor[contracts.ConversationInput]())
	}
	input := shape.(map[string]any)
	input["required"] = []string{"instruction", "materials"}
	if id == "conversation" {
		input["required"] = []string{"instruction", "materials", "directory"}
	}
	p := input["properties"].(map[string]any)
	if id == "generate" {
		p["repository"] = map[string]any{"type": "object", "required": []string{"local_path", "commit"}, "additionalProperties": false, "properties": map[string]any{"paths": map[string]any{"type": "array", "minItems": 1, "maxItems": contracts.MaxGenerateRepositoryFiles, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1}}, "local_path": map[string]any{"type": "string", "minLength": 1}, "commit": map[string]any{"type": "string", "pattern": `^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`}}}
		input["allOf"] = []any{map[string]any{"if": map[string]any{"required": []string{"repository"}}, "then": map[string]any{"required": []string{"output_schema"}, "not": map[string]any{"required": []string{"directory"}}}}}
	}
	p["instruction"] = map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}
	p["directory"] = map[string]any{"type": "object", "required": []string{"path", "access"}, "additionalProperties": false, "properties": map[string]any{"path": map[string]any{"type": "string", "minLength": 1}, "access": map[string]any{"const": "read_only"}}}
	p["materials"] = map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"kind", "path", "content"}, "additionalProperties": false, "properties": map[string]any{"kind": map[string]any{"const": "inline"}, "path": map[string]any{"type": "string", "minLength": 1}, "content": map[string]any{"type": "string"}, "commit": map[string]any{"const": ""}}}}
	return map[string]any{"type": "object", "required": []string{"title", "workflow", "execution", id}, "additionalProperties": false, "allOf": schemas["run_configuration"].(map[string]any)["allOf"], "properties": map[string]any{
		"title": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}, "correlation": shapeSchema(reflect.TypeFor[map[string]string]()), id: input,
		"name_prefix": namePrefixSchema(),
		"workflow":    map[string]any{"type": "object", "required": []string{"id", "version"}, "additionalProperties": false, "properties": map[string]any{"id": map[string]any{"const": id}, "version": map[string]any{"const": 1}}},
		"execution":   schemas["run_configuration"].(map[string]any)["properties"].(map[string]any)["execution"],
	}}
}
