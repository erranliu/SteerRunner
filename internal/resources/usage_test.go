package resources

import (
	"encoding/json"
	"testing"

	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestUsageCapabilitiesBoundariesAndSeparation(t *testing.T) {
	caps := UsageCapabilities().(map[string]any)
	for _, key := range []string{"pricing_policy", "cost_basis", "prices", "currency"} {
		if _, ok := caps[key]; ok {
			t.Fatal("pricing leaked into usage", key)
		}
	}
	matrix := caps["matrix"].([]map[string]any)
	if len(matrix) != 5 {
		t.Fatal(matrix)
	}
	seen := map[string]bool{}
	for _, row := range matrix {
		target, agent := row["target"].(string), row["agent"].(string)
		adapter := row["adapter"].(string)
		key := adapter + "/" + target + "/" + agent
		if seen[key] || (target != "local" && target != "orca_environment") {
			t.Fatal(row)
		}
		seen[key] = true
		want := agent == "codex"
		if row["collection"] != want || row["production_run_binding"] != want || row["parser"] != true || row["file_transport"] != (adapter == "orca-terminal") {
			t.Fatal(row)
		}
		if !want && row["reason"] != "PRODUCTION_BINDING_UNSUPPORTED" {
			t.Fatal(row)
		}
	}
	for _, key := range []string{"run_cost", "run_cost_estimate_schema", "run_pricing_policy_schema"} {
		if _, ok := Capabilities().(map[string]any)[key]; ok {
			t.Fatal("retired cost capability", key)
		}
	}

}

func TestRunUsageContractSchemas(t *testing.T) {
	for _, name := range []string{"run-usage.schema.json", "run-usage-events.schema.json", "service-start.schema.json"} {
		t.Run(name, func(t *testing.T) {
			var document any
			if err := json.Unmarshal(Read(name), &document); err != nil {
				t.Fatal(err)
			}
			compiler := jsonschema.NewCompiler()
			if err := compiler.AddResource("https://fixture.invalid/"+name, document); err != nil {
				t.Fatal(err)
			}
			schema, err := compiler.Compile("https://fixture.invalid/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if name == "run-usage.schema.json" {
				snapshot := usage.Project(api.Snapshot{RunID: "fixture", AccessCleanupStatus: "not_required"}, nil)
				raw, _ := json.Marshal(snapshot)
				var value any
				_ = json.Unmarshal(raw, &value)
				if err = schema.Validate(value); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
