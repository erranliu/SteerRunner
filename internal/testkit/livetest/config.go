package livetest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
)

const ConfigSchema = "steerrunner-test-config/v1"

type TestFixture struct {
	Repository  contracts.Repository   `json:"repository"`
	Publication *contracts.Publication `json:"publication,omitempty"`
}
type Harness struct {
	AdminHost string `json:"admin_host,omitempty"`
}
type TestConfig struct {
	Schema         string                     `json:"schema"`
	Configurations map[string]json.RawMessage `json:"configurations,omitempty"`
	Fixture        json.RawMessage            `json:"fixture,omitempty"`
	Harness        Harness                    `json:"harness,omitempty"`
}
type Configuration struct {
	Execution contracts.Execution
	Workflow  *contracts.Workflow
}
type Availability struct {
	ScenarioID   string            `json:"scenario_id"`
	CaseID       string            `json:"case_id"`
	Level        string            `json:"level"`
	Availability string            `json:"availability"`
	Reasons      []resources.Issue `json:"reasons"`
}
type ConfigReport struct {
	Schema             string            `json:"schema"`
	SourceDigest       string            `json:"source_digest"`
	ResourceDigest     string            `json:"resource_digest"`
	RequirementsDigest string            `json:"requirements_digest"`
	Configuration      TestConfig        `json:"configuration"`
	Baseline           []resources.Issue `json:"baseline"`
	Cases              []Availability    `json:"cases"`
}

func ReadConfig(path string) (TestConfig, string, error) {
	empty := TestConfig{Schema: ConfigSchema, Configurations: map[string]json.RawMessage{}}
	if path == "" {
		return empty, "", nil
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return empty, "", nil
	}
	if err != nil {
		return empty, "", errors.New("Cannot read test configuration.")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, contracts.MaxRequestBytes+1))
	if err != nil || len(raw) > contracts.MaxRequestBytes {
		return empty, "", errors.New("Cannot read bounded test configuration.")
	}
	var config TestConfig
	if contracts.DecodeStrict(raw, &config) != nil {
		return empty, "", errors.New("Invalid strict test configuration; check fields, types and duplicate keys.")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for _, name := range []string{"configurations", "fixture", "harness"} {
		if value, present := fields[name]; present && !bytes.HasPrefix(bytes.TrimSpace(value), []byte("{")) {
			return empty, "", errors.New("Test configuration containers must be objects when present.")
		}
	}
	var harness map[string]json.RawMessage
	_ = json.Unmarshal(fields["harness"], &harness)
	for _, value := range harness {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return empty, "", errors.New("Explicit test facility references must be strings, not null.")
		}
	}
	if config.Schema != ConfigSchema {
		return empty, "", fmt.Errorf("Unsupported test config; migrate live-config/v1|v2 to %s.", ConfigSchema)
	}
	base := filepath.Dir(path)
	for _, target := range []*string{&config.Harness.AdminHost} {
		if *target != "" && !filepath.IsAbs(*target) {
			*target = filepath.Join(base, *target)
		}
	}
	sum := sha256.Sum256(raw)
	return config, hex.EncodeToString(sum[:]), nil
}

func (c TestConfig) Configuration(name string) (Configuration, error) {
	var raw struct {
		Execution json.RawMessage `json:"execution"`
		Workflow  json.RawMessage `json:"workflow"`
	}
	data, exists := c.Configurations[name]
	if !exists {
		return Configuration{}, errors.New("missing_configuration")
	}
	if contracts.DecodeStrict(data, &raw) != nil {
		return Configuration{}, errors.New("invalid_configuration")
	}
	var execution contracts.Execution
	if contracts.DecodeStrict(raw.Execution, &execution) != nil {
		return Configuration{}, errors.New("invalid_configuration")
	}
	if fault := contracts.ValidateExecution(execution); fault != nil {
		if fault.Exit == 3 {
			return Configuration{}, errors.New(fault.Detail.Code)
		}
		return Configuration{}, errors.New("invalid_configuration")
	}
	result := Configuration{Execution: execution}
	if len(raw.Workflow) > 0 {
		input, _ := json.Marshal(map[string]json.RawMessage{"execution": raw.Execution, "workflow": raw.Workflow})
		if fault := contracts.ValidateRunConfiguration(input); fault != nil {
			if fault.Exit == 3 {
				return Configuration{}, errors.New(fault.Detail.Code)
			}
			return Configuration{}, errors.New("invalid_configuration")
		}
		var selection contracts.Workflow
		if contracts.DecodeStrict(raw.Workflow, &selection) != nil {
			return Configuration{}, errors.New("invalid_configuration")
		}
		result.Workflow = &selection
	}
	return result, nil
}

func (c TestConfig) BaselineIssues() []resources.Issue {
	issues := []resources.Issue{}
	value, err := c.Configuration("local-codex")
	if err != nil {
		return []resources.Issue{{Path: "/configurations/local-codex", Code: err.Error(), Message: "The mandatory local Codex baseline is not configured correctly."}}
	}
	var settings contracts.CodexSettings
	_ = json.Unmarshal(value.Execution.Settings, &settings)
	file := strings.ToLower(filepath.Base(settings.Executable))
	wrapper := slices.Contains([]string{"node.exe", "cmd.exe", "powershell.exe", "pwsh.exe", "sh.exe", "bash.exe"}, file) || slices.Contains([]string{".cmd", ".ps1", ".bat", ".js"}, strings.ToLower(filepath.Ext(file)))
	if value.Execution.Adapter != "codex-app-server" || value.Execution.Target.Kind != "local" || value.Workflow == nil || value.Workflow.ID != "development" || value.Workflow.Version != 2 || len(value.Workflow.PromptOverrides) != 0 || settings.AccessMode != "sandbox" || !filepath.IsAbs(settings.Executable) || wrapper {
		issues = append(issues, resources.Issue{Path: "/configurations/local-codex", Code: "invalid_configuration", Message: "Baseline requires local development/2, explicit sandbox and an absolute native Codex executable."})
	}
	return issues
}

func AssessConfig(config TestConfig, digest string, catalog resources.CapabilityCatalog) ConfigReport {
	result := ConfigReport{Schema: "steerrunner-test-availability/v1", SourceDigest: digest, ResourceDigest: resources.Digest(), RequirementsDigest: RegistryDigest(), Configuration: config, Baseline: config.BaselineIssues(), Cases: []Availability{}}
	for _, scenario := range Scenarios() {
		if scenario.Level != "L2" && scenario.Level != "L3" {
			continue
		}
		for _, test := range scenario.Cases {
			result.Cases = append(result.Cases, assessCase(config, scenario, test, catalog))
		}
	}
	return result
}

func assessCase(config TestConfig, scenario Scenario, test Case, catalog resources.CapabilityCatalog) Availability {
	result := Availability{ScenarioID: scenario.ID, CaseID: test.ID, Level: scenario.Level, Availability: "available", Reasons: []resources.Issue{}}
	add := func(path, code, message string) {
		result.Availability = "unavailable"
		result.Reasons = append(result.Reasons, resources.Issue{Path: path, Code: code, Message: message})
	}
	requirements := test.Requirements
	if requirements == nil {
		add("", "invalid_configuration", "Registered live case has no requirements.")
		return result
	}
	var fixture TestFixture
	fixtureOK := len(config.Fixture) > 0 && contracts.DecodeStrict(config.Fixture, &fixture) == nil
	if !fixtureOK {
		add("/fixture", "missing_fixture", "A valid test source fixture is required.")
	}
	if fixtureOK && (fixture.Repository.URL != RemoteURL || fixture.Repository.BaseRevision == "" || fixture.Repository.Remote != "origin" || fixture.Repository.BaseBranch == "") {
		add("/fixture/repository", "invalid_configuration", "Use the fixed SteerlineTest source and a complete source commit.")
		fixtureOK = false
	}
	if requirements.Publication && (fixture.Publication == nil || fixture.Publication.Repository != Repository) {
		add("/fixture/publication", "missing_fixture", "The case requires its explicit test publication configuration.")
	}
	if fixture.Publication != nil {
		var publication contracts.CommandProviderSettings
		if fixture.Publication.Provider != "command" || contracts.DecodeStrict(fixture.Publication.Settings, &publication) != nil || publication.BaseURL != "https://github.com" {
			add("/fixture/publication", "invalid_configuration", "The dedicated live fixture requires the GitHub command publication provider; other public providers are not substituted.")
		}
	}
	if requirements.Admin && config.Harness.AdminHost == "" {
		add("/harness/admin_host", "missing_test_facility", "The case requires an explicitly configured isolated network fault host.")
	}
	roles := make([]string, 0, len(requirements.Roles))
	var sharedLocalRuntime *contracts.AgentSettings
	for role := range requirements.Roles {
		roles = append(roles, role)
	}
	slices.Sort(roles)
	for _, role := range roles {
		required := requirements.Roles[role]
		path := "/configurations/" + required.Configuration
		value, err := config.Configuration(required.Configuration)
		if err != nil {
			add(path, err.Error(), "The required execution configuration is missing or invalid.")
			continue
		}
		var settings struct {
			Agent      string `json:"agent"`
			AccessMode string `json:"access_mode"`
		}
		_ = json.Unmarshal(value.Execution.Settings, &settings)
		agent := "codex"
		if settings.Agent != "" {
			agent = settings.Agent
		}
		mode := settings.AccessMode
		if mode == "" {
			mode = "sandbox"
		}
		if value.Execution.Adapter != required.Adapter || value.Execution.Target.Kind != required.TargetKind || agent != required.Agent || mode != required.AccessMode {
			add(path, "invalid_configuration", "Execution does not match the registered role, target, agent or permission mode.")
			continue
		}
		var selection *contracts.Workflow
		if requirements.Operation == "submit" {
			selection = value.Workflow
			if selection == nil || selection.ID != required.Workflow || selection.Version != 2 {
				add(path+"/workflow", "invalid_configuration", "The case requires its explicit registered workflow.")
				continue
			}
		}
		if !catalog.Declares(value.Execution, selection, requirements.Operation) {
			add(path, "undeclared_capability", "The selected build does not declare this execution route.")
		}
		if required.Configuration == "local-codex" && len(config.BaselineIssues()) > 0 {
			for _, issue := range config.BaselineIssues() {
				add(issue.Path, issue.Code, issue.Message)
			}
		}
		if required.DifferentModels {
			native, modelErr := contracts.ExecutionModel(value.Execution)
			if modelErr != nil || selection == nil || selection.AuditModel == nil || native.Model == "" || native.Model == selection.AuditModel.Model {
				add(path+"/workflow/audit_model", "invalid_configuration", "The audit case needs two explicit different models.")
			}
		}
		if required.Adapter == "orca-terminal" {
			var orca contracts.OrcaSettings
			_ = json.Unmarshal(value.Execution.Settings, &orca)
			if !filepath.IsAbs(orca.Transport.Executable) || (required.TargetKind == "local" && (!filepath.IsAbs(orca.AgentLaunch.RuntimeExecutable) || len(orca.AgentLaunch.RuntimeSHA256) != 64)) {
				add(path, "missing_configuration", "Orca tests require an absolute transport; local managed tests also require helper references.")
			}
			if fixtureOK && !filepath.IsAbs(fixture.Repository.LocalPath) {
				add("/fixture/repository/local_path", "missing_fixture", "Orca tests require the configured registered project entry.")
			}
			if scenario.ID == "orca-local.mixed" {
				if sharedLocalRuntime != nil && (orca.AgentLaunch.RuntimeExecutable != sharedLocalRuntime.RuntimeExecutable || orca.AgentLaunch.RuntimeSHA256 != sharedLocalRuntime.RuntimeSHA256) {
					add(path+"/execution/settings/agent_launch", "invalid_configuration", "The mixed local case requires both roles on the same configured native helper.")
				}
				launch := orca.AgentLaunch
				sharedLocalRuntime = &launch
			}
		}
		if required.Adapter == "codex-app-server" {
			var native contracts.CodexSettings
			_ = json.Unmarshal(value.Execution.Settings, &native)
			if !filepath.IsAbs(native.Executable) {
				add(path+"/execution/settings/executable", "invalid_configuration", "Native test preparation requires an explicit absolute Codex executable reference.")
			}
		}
		if fixtureOK {
			if requirements.Operation == "environment-check start" {
				raw, _ := json.Marshal(contracts.CheckInput{Repository: fixture.Repository, Execution: value.Execution, Publication: fixture.Publication})
				if fault := contracts.ValidateEnvironmentCheck(raw); fault != nil {
					add("/fixture", fault.Detail.Code, "The complete environment-check fixture violates the production contract.")
				}
			} else if selection != nil && fixture.Publication != nil {
				text := "fixture"
				raw, _ := json.Marshal(contracts.RunRequest{Title: "Test configuration", Repository: fixture.Repository, Execution: value.Execution, Workflow: *selection, Publication: *fixture.Publication, Documents: contracts.Documents{Spec: contracts.Document{Kind: "inline", Path: "spec.md", Content: &text}, Plan: contracts.Document{Kind: "inline", Path: "plan.md", Content: &text}}})
				if fault := contracts.ValidateRun(raw); fault != nil {
					add("/fixture", fault.Detail.Code, "The complete Run fixture violates the production contract.")
				}
			}
		}
	}
	return result
}
