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
		{"plain", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":2}`, true},
		{"legacy_empty_mode", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"access_mode":""}}`, `{"id":"development","version":2}`, true},
		{"legacy_null_app_reference", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"github_auth":null}}`, `{"id":"development","version":2}`, false},
		{"orca_legacy_empty_mode", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"access_mode":""}}`, `{"id":"development","version":2}`, true},
		{"orca_legacy", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":2}`, true},
		{"orca_pi", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi"}}`, `{"id":"development","version":2}`, true},
		{"orca_dsh", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"dsh","access_mode":"full_access"}}`, `{"id":"development","version":2}`, true},
		{"orca_empty_agent", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":""}}`, `{"id":"development","version":2}`, false},
		{"orca_null_agent", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":null}}`, `{"id":"development","version":2}`, false},
		{"orca_agent_shell", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi --unsafe"}}`, `{"id":"development","version":2}`, false},
		{"orca_pi_model", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi","agent_launch":{"model":"m"}}}`, `{"id":"development","version":2}`, true},
		{"orca_dsh_args", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"dsh","agent_launch":{"args":["--no-daemon"]}}}`, `{"id":"development","version":2}`, false},
		{"orca_app_auth_without_helper", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"github_auth":{"app_id":1,"installation_id":2,"private_key_env":"SR_GH_KEY"}}}`, `{"id":"development","version":2}`, false},
		{"orca_app_auth_launch_without_helper", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent_launch":{"executable":"codex.exe"},"github_auth":{"app_id":1,"installation_id":2,"private_key_env":"SR_GH_KEY"}}}`, `{"id":"development","version":2}`, false},
		{"orca_codex_app_auth", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"codex","agent_launch":{"runtime_executable":"helper.exe"},"github_auth":{"app_id":1,"installation_id":2,"private_key_env":"SR_GH_KEY"}}}`, `{"id":"development","version":2}`, false},
		{"orca_pi_app_auth", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"pi","agent_launch":{"runtime_executable":"helper.exe"},"github_auth":{"app_id":1,"installation_id":2,"private_key_env":"SR_GH_KEY"}}}`, `{"id":"development","version":2}`, false},
		{"orca_dsh_app_auth", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"dsh","agent_launch":{"runtime_executable":"helper.exe"},"github_auth":{"app_id":1,"installation_id":2,"private_key_env":"SR_GH_KEY"}}}`, `{"id":"development","version":2}`, false},
		{"orca_custom_app_auth", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent":"custom","agent_launch":{"runtime_executable":"helper.exe"},"github_auth":{"app_id":1,"installation_id":2,"private_key_env":"SR_GH_KEY"}}}`, `{"id":"development","version":2}`, false},
		{"audited_inherit_effort", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute-model"}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit-model"}}`, true},
		{"audited_explicit", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute-model","reasoning_effort":"medium"}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit-model","reasoning_effort":"high"},"prompt_overrides":{"audit.code":"Review code"}}`, true},

		{"plain_trimmed_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"x"}}`, `{"id":"development","version":2}`, true},
		{"plain_padded_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":" x "}}`, `{"id":"development","version":2}`, false},
		{"plain_blank_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"   "}}`, `{"id":"development","version":2}`, false},
		{"plain_newline_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"x\nx"}}`, `{"id":"development","version":2}`, false},
		{"plain_audit_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":2,"audit_model":{"model":"audit"}}`, false},
		{"missing_audit_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":2}`, false},
		{"missing_execution_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, false},
		{"remote", `{"adapter":"codex-app-server","target":{"kind":"orca_environment","environment":"remote"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, false},
		{"adapter", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent_launch":{"model":"execute"}}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, false},
		{"orca_audited", `{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"agent_launch":{"runtime_executable":"helper.exe","model":"execute"}}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, true},
		{"orca_remote_audited_no_helper", `{"adapter":"orca-terminal","target":{"kind":"orca_environment","environment":"paired"},"settings":{"agent_launch":{"model":"execute"}}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, false},
		{"orca_remote_audited_retired_helper", `{"adapter":"orca-terminal","target":{"kind":"orca_environment","environment":"paired"},"settings":{"agent_launch":{"model":"execute","runtime_executable":"helper.exe"}}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, false},
		{"orca_remote_audited_null_retired", `{"adapter":"orca-terminal","target":{"kind":"orca_environment","environment":"paired"},"settings":{"github_auth":null,"agent_launch":{"runtime_executable":"helper.exe","model":"execute"}}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit"}}`, false},
		{"override", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":2,"prompt_overrides":{"audit.spec":"No"}}`, false},
		{"unknown", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"other","version":2}`, false},
		{"null_model", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":2,"audit_model":null}`, false},
		{"null_effort", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit","reasoning_effort":null}}`, false},
		{"empty_effort", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"model":"execute"}}`, `{"id":"development-audited","version":2,"audit_model":{"model":"audit","reasoning_effort":""}}`, false},
		{"null_overrides", `{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}}`, `{"id":"development","version":2,"prompt_overrides":null}`, false},
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

func TestPublishedDevelopmentDocumentPaths(t *testing.T) {
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(ConfigurationSchemas(), &schemas); err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemas["run_input"])))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("https://runner.invalid/run", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("https://runner.invalid/run")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, spec string
		valid      bool
	}{
		{"path", `{"path":"docs/spec.md"}`, true},
		{"legacy_inline", `{"kind":"inline","path":"docs/spec.md","content":"caller text"}`, true},
		{"legacy_git", `{"kind":"git","path":"docs/spec.md","commit":"` + strings.Repeat("f", 40) + `"}`, true},
		{"untyped_content", `{"path":"docs/spec.md","content":"caller text"}`, false},
		{"missing_path", `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"title":"Path references","repository":{"url":"https://example.com/repo.git","base_revision":"` + strings.Repeat("a", 40) + `","remote":"origin","base_branch":"main"},"documents":{"spec":` + tc.spec + `,"plan":{"path":"docs/plan.md"},"references":[{"path":"docs/reference.md"}]},"workflow":{"id":"development","version":2},"execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}},"publication":{"provider":"gitea","repository":"owner/repo","base_branch":"main","settings":{"base_url":"https://example.com"}}}`)
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			if accepted := schema.Validate(value) == nil; accepted != tc.valid {
				t.Fatalf("schema accepted=%v want=%v: %v", accepted, tc.valid, schema.Validate(value))
			}
			if accepted := contracts.ValidateRun(raw) == nil; accepted != tc.valid {
				t.Fatalf("admission accepted=%v want=%v", accepted, tc.valid)
			}
		})
	}
}

func TestNativeRemoteCatalogDoesNotAdvertiseManagedGuarantees(t *testing.T) {
	if strings.Contains(string(ConfigurationSchemas()), `"github_auth"`) {
		t.Fatal("retired App configuration is still advertised")
	}
	count := 0
	for _, c := range Catalog().Combinations {
		if c.TargetKind != "orca_environment" {
			continue
		}
		count++
		raw, _ := json.Marshal(c)
		for _, retired := range []string{"host-gh-repository-broker", "native_codex_", "development-audited", "environment-check start"} {
			if strings.Contains(string(raw), retired) {
				t.Fatal("native remote catalog promises retired guarantee", string(raw))
			}
		}
	}
	if count == 0 {
		t.Fatal("native remote development disappeared")
	}
}
