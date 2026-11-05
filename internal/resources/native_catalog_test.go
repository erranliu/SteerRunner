package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"testing"
)

func TestNativeCatalogSchemaMatchesGate(t *testing.T) {
	var schemas struct {
		Request any `json:"request"`
	}
	if err := json.Unmarshal(Read("native-catalog.schema.json"), &schemas); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://runner.invalid/native", schemas.Request); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://runner.invalid/native")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"kind":"environments"}`, `{"kind":"agents","adapter":"orca-terminal","target":{"kind":"local","environment":""}}`, `{"kind":"agents","target":{"kind":"local"}}`, `{"kind":"models","adapter":"orca-terminal","target":{"kind":"orca_environment","environment":" "},"agent":"pi"}`, `{"kind":"models","adapter":"codex-app-server","target":{"kind":"local"},"agent":"codex","executable":" "}`} {
		var value any
		_ = json.Unmarshal([]byte(raw), &value)
		_, fault := contracts.ReadNativeCatalog([]byte(raw))
		if (schema.Validate(value) == nil) != (fault == nil) {
			t.Fatal("schema/gate differ", raw, fault)
		}
	}
}
