package resources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestSRPublishedRunConfiguration(t *testing.T) {
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(ConfigurationSchemas(), &schemas); err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemas["run_configuration"])))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://runner.invalid/config", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://runner.invalid/config")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, execution, workflow string
		valid                     bool
	}{
		{"plain", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":1}`, true},
		{"orca_legacy", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":1}`, true},
		{"orca_pi", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi"}}`, `{"id":"development","version":1}`, true},
		{"orca_dsh", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"dsh","access_mode":"full_access"}}`, `{"id":"development","version":1}`, true},
		{"orca_empty_agent", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":""}}`, `{"id":"development","version":1}`, false},
		{"orca_null_agent", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":null}}`, `{"id":"development","version":1}`, false},
		{"orca_agent_shell", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi --unsafe"}}`, `{"id":"development","version":1}`, false},
		{"orca_pi_model", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi","agent_launch":{"model":"m"}}}`, `{"id":"development","version":1}`, true},
		{"orca_pi_app", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi","agent_launch":{"runtime_executable":"helper.exe"},"github_auth":{"app_id":3,"installation_id":7,"private_key_env":"APP_KEY"}}}`, `{"id":"development","version":1}`, false},
		{"orca_dsh_app", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"dsh","agent_launch":{"runtime_executable":"helper.exe"},"github_auth":{"app_id":3,"installation_id":7,"private_key_env":"APP_KEY"}}}`, `{"id":"development","version":1}`, false},
		{"orca_dsh_args", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"dsh","agent_launch":{"args":["--no-daemon"]}}}`, `{"id":"development","version":1}`, false},
		{"audited_inherit_effort", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute-model"}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit-model"}}`, true},
		{"audited_explicit", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute-model","reasoning_effort":"medium"}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit-model","reasoning_effort":"high"},"prompt_overrides":{"audit.spec":"Review spec"}}`, true},

		{"plain_trimmed_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"x"}}`, `{"id":"development","version":1}`, true},
		{"plain_padded_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":" x "}}`, `{"id":"development","version":1}`, false},
		{"plain_blank_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"   "}}`, `{"id":"development","version":1}`, false},
		{"plain_newline_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"x\nx"}}`, `{"id":"development","version":1}`, false},
		{"plain_audit_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":1,"audit_model":{"model":"audit"}}`, false},
		{"missing_audit_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":1}`, false},
		{"missing_execution_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit"}}`, false},
		{"remote", `{"adapter":"codex-app-server","target":{"kind":"orca_environment","environment":"remote"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit"}}`, false},
		{"adapter", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent_launch":{"model":"execute"}}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit"}}`, false},
		{"override", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":1,"prompt_overrides":{"audit.spec":"No"}}`, false},
		{"unknown", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"other","version":1}`, false},
		{"null_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":1,"audit_model":null}`, false},
		{"null_effort", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit","reasoning_effort":null}}`, false},
		{"empty_effort", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":1,"audit_model":{"model":"audit","reasoning_effort":""}}`, false},
		{"null_overrides", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":1,"prompt_overrides":null}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"execution":` + tc.execution + `,"workflow":` + tc.workflow + `}`)
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			if accepted := schema.Validate(value) == nil; accepted != tc.valid {
				t.Fatalf("schema accepted=%v want=%v: %v", accepted, tc.valid, schema.Validate(value))
			}
			if accepted := contracts.ValidateRunConfiguration(raw) == nil; accepted != tc.valid {
				t.Fatalf("production accepted=%v want=%v: %v", accepted, tc.valid, contracts.ValidateRunConfiguration(raw))
			}
		})
	}
}
