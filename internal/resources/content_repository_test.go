package resources

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestContentRepositoryPublishedSchema(t *testing.T) {
	raw, err := os.ReadFile("../../docs/workflows/generate-repository.example.json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope contracts.Request
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if fault := contracts.ValidateRun(envelope.Input); fault != nil {
		t.Fatal(fault)
	}
	var schemas map[string]json.RawMessage
	_ = json.Unmarshal(ConfigurationSchemas(), &schemas)
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemas["generate_input"])))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("https://runner.invalid/generate", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("https://runner.invalid/generate")
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	_ = json.Unmarshal(envelope.Input, &input)
	if err := schema.Validate(input); err != nil {
		t.Fatal(err)
	}
	generate := input["generate"].(map[string]any)
	generate["directory"] = map[string]any{"path": "C:/Other", "access": "read_only"}
	if schema.Validate(input) == nil {
		t.Fatal("schema permits mixed directory/repository")
	}
	delete(generate, "directory")
	delete(generate, "output_schema")
	if schema.Validate(input) == nil {
		t.Fatal("schema permits repository without output_schema")
	}
}
