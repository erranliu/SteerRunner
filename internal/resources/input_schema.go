package resources

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// shapeSchema reflects public field names/types. Admission remains the pure gate;
// repository identity and other cross-field rules are not JSON Schema rules.
func shapeSchema(t reflect.Type) map[string]any {
	if t == reflect.TypeFor[json.RawMessage]() {
		return map[string]any{}
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{shapeSchema(t.Elem()), map[string]any{"type": "null"}}}
	}
	switch t.Kind() {
	case reflect.Struct:
		properties := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			tag := strings.Split(field.Tag.Get("json"), ",")[0]
			if field.PkgPath != "" || tag == "-" {
				continue
			}
			if tag == "" {
				tag = field.Name
			}
			properties[tag] = shapeSchema(field.Type)
		}
		return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": shapeSchema(t.Elem())}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": shapeSchema(t.Elem())}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int64, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	}
	return map[string]any{}
}

func runInputSchema(schemas map[string]any) map[string]any {
	result := shapeSchema(reflect.TypeFor[contracts.RunRequest]())
	result["$id"] = "steerrunner/run-input/v1"
	result["required"] = []string{"title", "repository", "documents", "execution", "workflow", "publication"}
	properties := result["properties"].(map[string]any)
	properties["progress_file"] = progressInputSchema()
	properties["title"] = map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}
	properties["name_prefix"] = namePrefixSchema()
	properties["execution"] = schemas["run_configuration"].(map[string]any)["properties"].(map[string]any)["execution"]
	properties["workflow"] = schemas["workflow"]
	result["allOf"] = schemas["run_configuration"].(map[string]any)["allOf"]
	repo := properties["repository"].(map[string]any)
	repo["required"] = []string{"url", "base_revision", "remote", "base_branch"}
	repo["properties"].(map[string]any)["base_revision"] = map[string]any{"type": "string", "pattern": `^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`}
	docs := properties["documents"].(map[string]any)
	docs["required"] = []string{"spec", "plan"}
	doc := shapeSchema(reflect.TypeFor[contracts.Document]())
	doc["required"] = []string{"path"}
	doc["description"] = "Repository-relative path in the execution worktree. The caller owns document availability; Runner does not read, copy or validate its contents. Legacy kind/content/commit fields do not select an execution material source."
	doc["oneOf"] = []any{
		map[string]any{"properties": map[string]any{"kind": map[string]any{"const": ""}, "content": map[string]any{"type": "null"}, "commit": map[string]any{"const": ""}}},
		map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "inline"}, "content": map[string]any{"type": "string"}, "commit": map[string]any{"const": ""}}, "required": []string{"kind", "content"}},
		map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "git"}, "content": map[string]any{"type": "null"}, "commit": map[string]any{"type": "string", "pattern": `^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`}}, "required": []string{"kind", "commit"}},
	}
	dp := docs["properties"].(map[string]any)
	dp["spec"] = doc
	dp["plan"] = doc
	dp["references"] = map[string]any{"type": "array", "items": doc}
	publication := properties["publication"].(map[string]any)
	publication["required"] = []string{"provider", "repository", "base_branch", "settings"}
	choices := []any{}
	for _, provider := range []string{"command", "gitea"} {
		choices = append(choices, map[string]any{"properties": map[string]any{"provider": map[string]any{"const": provider}, "settings": schemas["providers"].(map[string]any)[provider]}})
	}
	publication["oneOf"] = choices
	return result
}

func namePrefixSchema() map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": contracts.MaxNamePrefixLength, "pattern": contracts.NamePrefixPattern}
}
