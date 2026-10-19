package livetest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestSRIndependentTestConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	for _, raw := range []string{`{"schema":"steerrunner-live-config/v1"}`, `{"schema":"steerrunner-test-config/v1","unknown":true}`, `{"schema":"steerrunner-test-config/v1","configurations":[]}`, `{"schema":"steerrunner-test-config/v1","configurations":null}`, `{"schema":"steerrunner-test-config/v1","harness":{"admin_host":null}}`, `{"schema":"steerrunner-test-config/v1","schema":"steerrunner-test-config/v1"}`} {
		_ = os.WriteFile(path, []byte(raw), 0600)
		if _, _, err := ReadConfig(path); err == nil {
			t.Fatal("invalid outer configuration accepted", raw)
		}
	}
	fixture, _ := json.Marshal(TestFixture{Repository: contracts.Repository{URL: RemoteURL, Remote: "origin", BaseBranch: "main", BaseRevision: strings.Repeat("1", 40)}, Publication: &contracts.Publication{Provider: "command", Repository: Repository, BaseBranch: "main", Settings: json.RawMessage(`{"base_url":"https://github.com","query":{"executable":"gh","args":["api"]}}`)}})
	native := filepath.Join(dir, "native-codex.exe")
	settings, _ := json.Marshal(map[string]any{"executable": native, "access_mode": "sandbox"})
	entry, _ := json.Marshal(map[string]any{"execution": contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: settings}, "workflow": contracts.Workflow{ID: "development", Version: 2}})
	config := TestConfig{Schema: ConfigSchema, Configurations: map[string]json.RawMessage{"local-codex": entry, "orca-local-pi": json.RawMessage(`{}`)}, Fixture: fixture, Harness: Harness{AdminHost: "relative-fault-host"}}
	body, _ := json.Marshal(config)
	_ = os.WriteFile(path, body, 0600)
	loaded, digest, err := ReadConfig(path)
	if err != nil || digest == "" || loaded.Harness.AdminHost != filepath.Join(dir, "relative-fault-host") {
		t.Fatal("path or strict parsing failed", err)
	}
	report := AssessConfig(loaded, digest, resources.Catalog())
	if len(report.Baseline) != 0 {
		t.Fatal(report.Baseline)
	}
	found := map[string]Availability{}
	for _, test := range report.Cases {
		found[test.ScenarioID+"/"+test.CaseID] = test
	}
	if found["local-codex.feedback/default"].Availability != "available" || found["orca-local.pi/default"].Availability != "unavailable" || found["orca-remote.smoke/default"].Availability != "unavailable" {
		t.Fatal("one missing entry blocked independent routes", found)
	}
	if _, err := os.Stat(native); !os.IsNotExist(err) {
		t.Fatal("assessment must not create/probe the native executable")
	}
	wrongProvider := loaded
	var shared TestFixture
	_ = json.Unmarshal(fixture, &shared)
	shared.Publication.Provider = "gitea"
	shared.Publication.Settings = json.RawMessage(`{"base_url":"https://github.com"}`)
	wrongProvider.Fixture, _ = json.Marshal(shared)
	for _, test := range AssessConfig(wrongProvider, digest, resources.Catalog()).Cases {
		if test.ScenarioID == "local-codex.feedback" && test.Availability != "unavailable" {
			t.Fatal("fixture generation would silently replace the configured publication provider")
		}
	}
	shared.Publication.Provider = "command"
	shared.Publication.Settings = json.RawMessage(`{"base_url":"https://github.com/","query":{"executable":"gh","args":["api"]}}`)
	wrongProvider.Fixture, _ = json.Marshal(shared)
	for _, test := range AssessConfig(wrongProvider, digest, resources.Catalog()).Cases {
		if test.ScenarioID == "environment-check.local-sandbox" && test.Availability != "unavailable" {
			t.Fatal("noncanonical fixture URL cannot pass the actual check admission", test)
		}
	}
	// Removing workflow only disables Run cases; a Check still consumes the same Execution.
	var value map[string]json.RawMessage
	_ = json.Unmarshal(entry, &value)
	delete(value, "workflow")
	entry, _ = json.Marshal(value)
	loaded.Configurations["local-codex-check"] = entry
	baseScenario, baseCase, _ := Lookup("environment-check.local-sandbox", "complete")
	baseCase.Requirements = &Requirements{Roles: map[string]RoleRequirement{"primary": {Configuration: "local-codex-check", Adapter: "codex-app-server", TargetKind: "local", Agent: "codex", AccessMode: "sandbox"}}, Operation: "environment-check start", Publication: true}
	if result := assessCase(loaded, baseScenario, baseCase, resources.Catalog()); result.Availability != "available" {
		t.Fatal("Check required workflow", result)
	}
	baseline := loaded.Configurations["local-codex"]
	delete(loaded.Configurations, "local-codex")
	if result := assessCase(loaded, baseScenario, baseCase, resources.Catalog()); result.Availability != "available" {
		t.Fatal("unrelated baseline became a global live prerequisite", result)
	}
	loaded.Configurations["local-codex"] = baseline
	// A frozen object survives source-file changes; a subsequent batch sees them.
	_ = os.WriteFile(path, []byte(`{"schema":"steerrunner-test-config/v1"}`), 0600)
	if len(loaded.Configurations) != 3 {
		t.Fatal("source edit mutated the batch snapshot")
	}
	next, _, _ := ReadConfig(path)
	if len(next.BaselineIssues()) == 0 {
		t.Fatal("missing baseline became delivery-ready")
	}
}

func TestSRBaselineConstraintsAndGateReasons(t *testing.T) {
	for _, executable := range []string{"relative-codex.exe", filepath.Join(t.TempDir(), "codex.cmd"), filepath.Join(t.TempDir(), "node.exe")} {
		raw, _ := json.Marshal(map[string]any{"execution": contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: marshalConfig(map[string]any{"executable": executable, "access_mode": "sandbox"})}, "workflow": contracts.Workflow{ID: "development", Version: 2}})
		if len((TestConfig{Configurations: map[string]json.RawMessage{"local-codex": raw}}).BaselineIssues()) == 0 {
			t.Fatal("non-native or relative baseline accepted", executable)
		}
	}
	config := TestConfig{Configurations: map[string]json.RawMessage{"unsupported": json.RawMessage(`{"execution":{"adapter":"not-implemented","target":{"kind":"local"},"settings":{}}}`)}}
	if _, err := config.Configuration("unsupported"); err == nil || err.Error() != "unsupported_capability" {
		t.Fatal("production unsupported fault was mislabeled as malformed configuration", err)
	}
	if err := (Manifest{RequirementsSHA256: strings.Repeat("0", 64)}).Validate(); err == nil || !strings.Contains(err.Error(), "requirements") {
		t.Fatal("native driver ignored registry identity", err)
	}
}

func marshalConfig(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func TestSRAllLiveCasesHaveRequirements(t *testing.T) {
	for _, s := range Scenarios() {
		for _, c := range s.Cases {
			if s.Level == "L4" {
				continue
			}
			if c.Requirements == nil || len(c.Requirements.Roles) == 0 {
				t.Fatal("missing requirements", s.ID, c.ID)
			}
			if s.Level == "L2" && c.Requirements.Roles["primary"].Configuration != "local-codex" {
				t.Fatal("L2 baseline drift")
			}
		}
	}
}

func TestSRCheckAuthenticationAvailability(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "test-config.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config TestConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	var entry struct {
		Execution contracts.Execution `json:"execution"`
		Workflow  contracts.Workflow  `json:"workflow"`
	}
	_ = json.Unmarshal(config.Configurations["local-codex"], &entry)
	entry.Execution.Settings = marshalConfig(map[string]any{"executable": filepath.Join(t.TempDir(), "codex.exe"), "access_mode": "sandbox", "github_auth": map[string]any{"app_id": 1, "installation_id": 2, "private_key_env": "TEST_APP_KEY"}})
	config.Configurations["local-codex"] = marshalConfig(entry)
	report := AssessConfig(config, "source", resources.Catalog())
	if report.RequirementsDigest != RegistryDigest() {
		t.Fatal("assessment lost registry identity")
	}
	for _, test := range report.Cases {
		switch test.ScenarioID {
		case "environment-check.local-sandbox", "local-codex.feedback":
			found := false
			for _, reason := range test.Reasons {
				found = found || reason.Code == "invalid_configuration"
			}
			if test.Availability != "unavailable" || !found {
				t.Fatal("known unsupported check authentication became available", test)
			}
		}
	}
}

func TestSRNativePreparationReferencesAreStaticConstraints(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "test-config.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config TestConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var fixture TestFixture
	_ = json.Unmarshal(config.Fixture, &fixture)
	fixture.Repository.LocalPath = filepath.Join(dir, "uninstalled-project")
	config.Fixture = marshalConfig(fixture)
	for _, test := range []struct {
		name, transport, helper string
		available               bool
	}{
		{"absolute-uninstalled", filepath.Join(dir, "orca.cmd"), filepath.Join(dir, "helper.exe"), true},
		{"relative-transport", "orca.cmd", filepath.Join(dir, "helper.exe"), false},
		{"relative-local-helper", filepath.Join(dir, "orca.cmd"), "helper.exe", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			execution := contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: marshalConfig(contracts.OrcaSettings{AccessMode: "sandbox", Transport: contracts.TransportSettings{Executable: test.transport}, AgentLaunch: contracts.AgentSettings{RuntimeExecutable: test.helper, RuntimeSHA256: strings.Repeat("a", 64)}})}
			if fault := contracts.ValidateExecution(execution); fault != nil {
				t.Fatal("test constraint leaked into the public gate", fault)
			}
			config.Configurations["orca-local-codex"] = marshalConfig(map[string]any{"execution": execution, "workflow": contracts.Workflow{ID: "development", Version: 2}})
			scenario, c, err := Lookup("orca-local.codex", "default")
			if err != nil {
				t.Fatal(err)
			}
			result := assessCase(config, scenario, c, resources.Catalog())
			if (result.Availability == "available") != test.available {
				t.Fatal("availability disagrees with fixture path requirements", result)
			}
		})
	}
	for _, shared := range []bool{true, false} {
		for _, agent := range []string{"pi", "dsh"} {
			helper := filepath.Join(dir, "helper.exe")
			if !shared && agent == "dsh" {
				helper = filepath.Join(dir, "other-helper.exe")
			}
			execution := contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: marshalConfig(contracts.OrcaSettings{Agent: agent, AccessMode: "sandbox", Transport: contracts.TransportSettings{Executable: filepath.Join(dir, "orca.cmd")}, AgentLaunch: contracts.AgentSettings{RuntimeExecutable: helper, RuntimeSHA256: strings.Repeat("a", 64)}})}
			config.Configurations["orca-local-"+agent] = marshalConfig(map[string]any{"execution": execution, "workflow": contracts.Workflow{ID: "development", Version: 2}})
		}
		scenario, c, _ := Lookup("orca-local.mixed", "default")
		result := assessCase(config, scenario, c, resources.Catalog())
		if (result.Availability == "available") != shared {
			t.Fatal("mixed helper constraint was deferred to runtime", result)
		}
	}
}

func TestSRTestConfigOuterSchemaMatchesParser(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "test-config.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://runner.invalid/test-config", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://runner.invalid/test-config")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	for _, input := range []string{
		`{"schema":"steerrunner-test-config/v1"}`,
		`{"schema":"steerrunner-test-config/v1","configurations":{"bad-entry":null},"fixture":{"bad-field":true}}`,
		`{"schema":"steerrunner-test-config/v1","configurations":null}`,
		`{"schema":"steerrunner-test-config/v1","harness":{"admin_host":null}}`,
		`{"schema":"steerrunner-test-config/v1","harness":{"unknown":"x"}}`,
		`{"schema":"steerrunner-test-config/v1","extra":true}`,
	} {
		var value any
		_ = json.Unmarshal([]byte(input), &value)
		_ = os.WriteFile(path, []byte(input), 0600)
		_, _, parseErr := ReadConfig(path)
		if (schema.Validate(value) == nil) != (parseErr == nil) {
			t.Fatal("outer schema/parser drift", input, parseErr)
		}
	}
}
