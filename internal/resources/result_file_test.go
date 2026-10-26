package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"testing"
)

func TestResultFilePublishedContract(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(Read("result-file.schema.json"), &schema); err != nil {
		t.Fatal(err)
	}
	caps := ResultFileCapabilities().(map[string]any)
	if schema["$id"] != contracts.ResultFileContract || caps["contract"] != schema["$id"] || caps["submit_property"] != "result_file" || caps["environment_variable"] != contracts.ResultFileEnvironment || caps["command"] != "result-file --run-id R --json" {
		t.Fatal(caps, schema)
	}
	var schemas map[string]any
	if err := json.Unmarshal(ConfigurationSchemas(), &schemas); err != nil {
		t.Fatal(err)
	}
	input := schemas["run_input"].(map[string]any)["oneOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)
	if input["result_file"] == nil || input["progress_file"] == nil {
		t.Fatal("missing input schema")
	}
	if caps["collection"] != "end_only" || caps["query_reads_file"] != false || caps["delivery"] == nil || caps["version"] == nil {
		t.Fatal("missing final-only semantics")
	}
	if ProgressCapabilities().(map[string]any)["contract"] != contracts.ProgressContract {
		t.Fatal("progress contract replaced")
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://runner.invalid/result-file", schema); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("https://runner.invalid/result-file")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		version int
		content bool
		valid   bool
	}{{0, false, true}, {0, true, false}, {1, false, false}, {1, true, true}, {2, true, false}} {
		view := map[string]any{"contract": contracts.ResultFileContract, "run_id": "R", "version": tc.version, "encoding": "base64", "size_bytes": 0, "collection_status": "closed"}
		if tc.version == 0 && !tc.content {
			view["diagnostic"] = "file_missing"
		}
		if tc.content {
			view["content_base64"] = ""
			view["observed_at"] = "2026-10-26T00:00:00Z"
			view["sha256"] = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		}
		// JSON round trip uses the same numeric representation as CLI responses.
		raw, _ := json.Marshal(view)
		var value any
		_ = json.Unmarshal(raw, &value)
		if err := compiled.Validate(value); (err == nil) != tc.valid {
			t.Fatal(tc, err)
		}
	}
}
