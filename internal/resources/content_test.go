package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"testing"
)

func TestContentPublishedSchemas(t *testing.T) {
	var schemas map[string]json.RawMessage
	_ = json.Unmarshal(ConfigurationSchemas(), &schemas)
	for _, id := range []string{"generate", "conversation"} {
		input := map[string]any{"instruction": "Reply", "materials": []any{}}
		if id == "conversation" {
			input["directory"] = map[string]any{"path": "C:/test", "access": "read_only"}
		}
		v := map[string]any{"title": "test", "workflow": map[string]any{"id": id, "version": 1}, "execution": map[string]any{"adapter": "codex-app-server", "target": map[string]any{"kind": "local"}, "settings": map[string]any{"access_mode": "read_only"}}, id: input}
		for _, name := range []string{"run_input", id + "_input"} {
			c := jsonschema.NewCompiler()
			doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemas[name])))
			if err != nil {
				t.Fatal(err)
			}
			if err = c.AddResource("https://runner.invalid/test", doc); err != nil {
				t.Fatal(err)
			}
			s, err := c.Compile("https://runner.invalid/test")
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if err = s.Validate(v); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		raw, _ := json.Marshal(v)
		if result, err := Validate("run", raw); err != nil || !result.Valid {
			t.Fatal(err, result)
		}
		selection := &contracts.Workflow{ID: id, Version: 1}
		execution := contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"read_only"}`)}
		if !Catalog().Declares(execution, selection, "submit") {
			t.Fatal("missing catalog route")
		}
	}
}
