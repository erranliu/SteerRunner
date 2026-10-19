package resources

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRValidationPreservesGateAndDiagnostics(t *testing.T) {
	examples := []string{
		`{"execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}},"workflow":{"id":"development","version":1}}`,
		`{"execution":{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"future-agent"}},"workflow":{"id":"development","version":1}}`,
		`{"execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}},"workflow":{"id":"development-audited","version":1,"audit_model":{"model":"audit"}}}`,
		`{"execution":{"adapter":"codex-app-server","target":{"kind":"orca_environment","environment":"e"},"settings":{}},"workflow":{"id":"development-audited","version":1,"audit_model":{"model":"audit"}}}`,
		`{}`, `null`, `{"workflow":null}`, `{"workflow":{"id":"unknown","version":1}}`,
	}
	for _, input := range examples {
		original := contracts.ValidateRunConfiguration([]byte(input))
		result, fault := Validate("run-configuration", []byte(input))
		if !reflect.DeepEqual(original, fault) || result.Valid != (original == nil) {
			t.Fatalf("gate changed for %s: %v vs %v", input, original, fault)
		}
		if original != nil && len(result.Issues) == 0 {
			t.Fatal("missing diagnostics")
		}
		if strings.Contains(input, `"model":"audit"`) && strings.Contains(input, `"target":{"kind":"local"}`) {
			found := false
			for _, issue := range result.Issues {
				if issue.Path == "/execution/settings/model" && issue.Code == "required" {
					found = true
				}
			}
			if !found {
				t.Fatalf("lost field diagnostic: %+v", result.Issues)
			}
		}
	}
	for _, name := range []string{"run-configuration", "run", "environment-check"} {
		if diagnosticSchema(name) == nil {
			t.Fatal("published schema did not compile", name)
		}
	}
}

func TestSRCapabilityCatalogIsStaticAndResolvable(t *testing.T) {
	catalog := Catalog()
	before := Digest()
	changed := Catalog()
	changed.Version++
	if resourceDigest(changed) == before {
		t.Fatal("published catalog changes must invalidate the resource identity")
	}
	source := CapabilitySource{Kind: "service", InstanceID: "instance-one", Version: Version, ExecutableSHA256: strings.Repeat("a", 64), ResourceDigest: before}
	online := CapabilitiesFrom(source)
	source.InstanceID = "instance-two"
	source.ExecutableSHA256 = strings.Repeat("b", 64)
	offline := CapabilitiesFrom(source)
	if !reflect.DeepEqual(online["capability_catalog"], offline["capability_catalog"]) || Digest() != before {
		t.Fatal("source identity entered static resources")
	}
	for name, pointer := range catalog.ValidationKinds {
		var schemas map[string]json.RawMessage
		_ = json.Unmarshal(ConfigurationSchemas(), &schemas)
		if name == "environment-check" {
			if pointer != "/environment_check_schema/$defs/input" {
				t.Fatal(pointer)
			}
			continue
		}
		key := strings.TrimPrefix(pointer, "/configuration_schemas/")
		if len(schemas[key]) == 0 {
			t.Fatal("unresolvable schema", name, pointer)
		}
	}
	for _, entry := range catalog.Combinations {
		settings := map[string]any{"access_mode": entry.AccessMode}
		if entry.Adapter == "orca-terminal" {
			settings["agent"] = entry.Agent
		}
		execution := contracts.Execution{Adapter: entry.Adapter, Target: contracts.Target{Kind: entry.TargetKind}, Settings: marshal(settings)}
		if entry.TargetKind == "orca_environment" {
			execution.Target.Environment = "selected"
		}
		if contracts.ValidateExecution(execution) != nil || !catalog.Declares(execution, entry.Workflow, entry.Operations[0]) {
			t.Fatal("catalog disagrees with gate", entry)
		}
	}
	// Legacy extension values remain accepted without being advertised as implemented.
	raw := `{"execution":{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"future-agent"}},"workflow":{"id":"development","version":1}}`
	var input contracts.RunConfiguration
	_ = json.Unmarshal([]byte(raw), &input)
	result, fault := Validate("run-configuration", []byte(raw))
	if fault != nil || !result.Valid || catalog.Declares(input.Execution, &input.Workflow, "submit") {
		t.Fatal("catalog became an admission whitelist")
	}
}
