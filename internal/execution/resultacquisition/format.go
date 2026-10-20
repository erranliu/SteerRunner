package resultacquisition

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type noExternalSchemas struct{}

func (noExternalSchemas) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema loading is disabled: %s", url)
}

// CompileOutputSchema has no filesystem/network loader. Only fragment refs
// within this one resource are accepted, including in unused definitions.
func CompileOutputSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	var check func(any) error
	check = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "$id" || k == "$schema" { // Fixed dialect/base; no nested resource identities.
					if k == "$id" || v != "https://json-schema.org/draft/2020-12/schema" {
						return fmt.Errorf("unsupported schema keyword %s", k)
					}
				}
				if k == "$ref" || k == "$dynamicRef" || k == "$recursiveRef" {
					s, ok := v.(string)
					if !ok || !strings.HasPrefix(s, "#") {
						return fmt.Errorf("only local fragment references are supported")
					}
				}
				switch k {
				case "$defs", "definitions", "properties", "patternProperties", "dependentSchemas":
					if schemas, ok := v.(map[string]any); ok {
						for _, schema := range schemas {
							if err := check(schema); err != nil {
								return err
							}
						}
					}
				case "additionalProperties", "unevaluatedProperties", "propertyNames", "items", "prefixItems", "additionalItems", "unevaluatedItems", "contains", "not", "if", "then", "else", "allOf", "anyOf", "oneOf":
					if err := check(v); err != nil {
						return err
					}
				}
			}
		case []any:
			for _, v := range x {
				if err := check(v); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := check(doc); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(noExternalSchemas{})
	const url = "https://runner.invalid/output"
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

func ValidateOutput(raw json.RawMessage, text string) (json.RawMessage, error) {
	s, err := CompileOutputSchema(raw)
	if err != nil {
		return nil, err
	}
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	if err := s.Validate(v); err != nil {
		return nil, fmt.Errorf("schema_mismatch: %w", err)
	}
	return json.RawMessage(strings.TrimSpace(text)), nil
}
