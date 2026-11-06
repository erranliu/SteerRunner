package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func configurationRequest(t *testing.T, in ExecutionConfigurationRequest) ExecutionConfigurationResult {
	t.Helper()
	raw, _ := json.Marshal(in)
	out, fault := ConfigureExecution(raw, ExecutionConfigurationSource{})
	if fault != nil {
		t.Fatalf("%s: %v", raw, fault)
	}
	return out
}

func TestExecutionConfigurationIdentityAndDerivation(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		t.Run(adapter, func(t *testing.T) {
			settings := `{"access_mode":"full_access","model":"chosen","reasoning_effort":"high"}`
			if adapter == "orca-terminal" {
				settings = `{"access_mode":"full_access","agent_launch":{"model":"chosen","reasoning_effort":"high"}}`
			}
			execution := Execution{Adapter: adapter, Target: Target{Kind: "local"}, Settings: json.RawMessage(settings)}
			describe := ExecutionConfigurationRequest{Operation: "describe", Execution: &execution, Workflow: Workflow{ID: "development", Version: 2}}
			first := configurationRequest(t, describe)
			other := execution
			other.Settings = json.RawMessage(strings.ReplaceAll(strings.ReplaceAll(settings, "chosen", "different"), "high", "low"))
			describe.Execution = &other
			second := configurationRequest(t, describe)
			if first.Identity.SourceExecutionDigest == second.Identity.SourceExecutionDigest || ConfigurationDigest(first.Identity.Context) != ConfigurationDigest(second.Identity.Context) {
				t.Fatal("model preferences changed context or lost source identity")
			}
			if first.View.Model == nil || *first.View.Model != "chosen" || first.Execution != nil {
				t.Fatal(first)
			}
			execute := configurationRequest(t, ExecutionConfigurationRequest{Operation: "derive", Execution: &execution, Workflow: Workflow{ID: "execute", Version: 1}, AccessMode: "full_access"})
			if ConfigurationDigest(*execute.Execution) != ConfigurationDigest(execution) || execute.SourceIdentity.SourceExecutionDigest != first.Identity.SourceExecutionDigest {
				t.Fatal("execute changed original native configuration")
			}
			if string(execution.Settings) != settings {
				t.Fatal("source mutated")
			}
			if adapter == "codex-app-server" {
				generated := configurationRequest(t, ExecutionConfigurationRequest{Operation: "derive", Execution: &execution, Workflow: Workflow{ID: "generate", Version: 1}, AccessMode: "read_only"})
				if generated.View.AccessMode != "read_only" || *generated.View.Model != "chosen" || generated.SourceIdentity.SourceExecutionDigest != first.Identity.SourceExecutionDigest {
					t.Fatal(generated)
				}
			}
		})
	}
}

func TestExecutionConfigurationComposePreservesTrustedPrograms(t *testing.T) {
	base := Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"executable":"C:/trusted/codex.exe","access_mode":"full_access","model":"old","reasoning_effort":"high"}`)}
	chosen := "new"
	in := ExecutionConfigurationRequest{Operation: "compose", BaseExecution: &base, Workflow: Workflow{ID: "development", Version: 2}, Selection: &ExecutionSelection{Adapter: base.Adapter, Target: base.Target, Agent: "codex", AccessMode: "full_access", Model: &chosen}, Unset: []string{"reasoning_effort"}}
	out := configurationRequest(t, in)
	var values CodexSettings
	_ = json.Unmarshal(out.Execution.Settings, &values)
	if values.Executable != "C:/trusted/codex.exe" || values.Model != "new" || values.ReasoningEffort != "" {
		t.Fatal(values)
	}
	if !strings.Contains(string(base.Settings), `"old"`) {
		t.Fatal("source changed")
	}
	for _, change := range []func(){func() { in.Selection.Target = Target{Kind: "orca_environment", Environment: "other"} }, func() { in.Selection.Target = base.Target; in.Unset = []string{"model"} }, func() { in.Unset = []string{"reasoning_effort", "reasoning_effort"} }} {
		change()
		raw, _ := json.Marshal(in)
		if _, fault := ConfigureExecution(raw, ExecutionConfigurationSource{}); fault == nil {
			t.Fatal("unsafe compose accepted", string(raw))
		}
	}
}

func TestExecutionConfigurationComposePreferencesKeepOrcaIdentity(t *testing.T) {
	var common string
	for _, settings := range []string{`{"access_mode":"full_access"}`, `{"access_mode":"full_access","agent_launch":{}}`, `{"access_mode":"full_access","agent_launch":{"model":"one"}}`, `{"access_mode":"full_access","agent_launch":{"reasoning_effort":"high"}}`} {
		base := Execution{Adapter: "orca-terminal", Target: Target{Kind: "local"}, Settings: json.RawMessage(settings)}
		initial := configurationRequest(t, ExecutionConfigurationRequest{Operation: "describe", Execution: &base, Workflow: Workflow{ID: "development", Version: 2}})
		key := ConfigurationDigest(initial.Identity.Context)
		if common == "" {
			common = key
		}
		if key != common {
			t.Fatal("empty preference container changed identity", settings)
		}
		model := "new"
		composed := configurationRequest(t, ExecutionConfigurationRequest{Operation: "compose", BaseExecution: &base, Workflow: Workflow{ID: "development", Version: 2}, Selection: &ExecutionSelection{Adapter: base.Adapter, Target: base.Target, Agent: "codex", AccessMode: "full_access", Model: &model}})
		cleared := configurationRequest(t, ExecutionConfigurationRequest{Operation: "compose", BaseExecution: composed.Execution, Workflow: Workflow{ID: "development", Version: 2}, Selection: &ExecutionSelection{Adapter: base.Adapter, Target: base.Target, Agent: "codex", AccessMode: "full_access"}, Unset: []string{"model", "reasoning_effort"}})
		if ConfigurationDigest(composed.Identity.Context) != key || ConfigurationDigest(cleared.Identity.Context) != key {
			t.Fatal("preference compose changed context", settings)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(composed.Execution.Settings, &fields)
		if _, ok := fields["agent"]; ok {
			t.Fatal("compose added an implicit agent to a saved base")
		}
		if string(base.Settings) != settings {
			t.Fatal("source changed")
		}
	}
}

func TestExecutionConfigurationRejectsAmbiguityAndRetiredSources(t *testing.T) {
	valid := `{"operation":"derive","execution":{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"access_mode":"full_access","agent_launch":{"model":"selected"}}},"workflow":{"id":"execute","version":1},"access_mode":"full_access"}`
	for name, raw := range map[string]string{
		"duplicate":     strings.Replace(valid, `"operation":"derive"`, `"operation":"derive","operation":"describe"`, 1),
		"trailing":      valid + `{}`,
		"helper":        strings.Replace(valid, `"model":"selected"`, `"model":"selected","runtime_executable":"retired","runtime_sha256":"`+strings.Repeat("a", 64)+`"`, 1),
		"executable":    strings.Replace(valid, `"model":"selected"`, `"model":"selected","executable":"retired"`, 1),
		"args":          strings.Replace(valid, `"model":"selected"`, `"model":"selected","args":["--search"]`, 1),
		"permission":    strings.Replace(valid, `"access_mode":"full_access"}`, `"access_mode":"read_only"}`, 1),
		"selector-body": strings.Replace(valid, `"id":"execute","version":1`, `"id":"execute","version":1,"prompt_overrides":{}`, 1),
		"null":          strings.Replace(valid, `"access_mode":"full_access"}`, `"access_mode":null}`, 1),
		"unknown":       strings.Replace(valid, `"operation":"derive"`, `"operation":"derive","task_id":"task"`, 1),
		"limit":         strings.Repeat(" ", ExecutionConfigurationLimit) + valid,
	} {
		t.Run(name, func(t *testing.T) {
			if _, fault := ConfigureExecution([]byte(raw), ExecutionConfigurationSource{}); fault == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestNativeCatalogUsesTrustedExecutionContext(t *testing.T) {
	valid := `{"kind":"models","adapter":"codex-app-server","target":{"kind":"local"},"agent":"codex","execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"access_mode":"full_access","executable":"C:/chosen/codex.exe"}}}`
	in, fault := ReadNativeCatalog([]byte(valid))
	if fault != nil || in.Execution == nil {
		t.Fatal(in, fault)
	}
	for _, raw := range []string{strings.Replace(valid, `"kind":"models"`, `"kind":"models","executable":"other"`, 1), strings.Replace(valid, `"agent":"codex"`, `"agent":"pi"`, 1)} {
		if _, fault := ReadNativeCatalog([]byte(raw)); fault == nil {
			t.Fatal("mismatched directory context accepted")
		}
	}
	agents := `{"kind":"agents","adapter":"orca-terminal","target":{"kind":"local"},"execution":{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"access_mode":"full_access","agent":"pi"}}}`
	if _, fault := ReadNativeCatalog([]byte(agents)); fault != nil {
		t.Fatal(fault)
	}
	if _, fault := ReadNativeCatalog([]byte(strings.Replace(agents, `"kind":"agents"`, `"kind":"agents","agent":"pi"`, 1))); fault == nil {
		t.Fatal("agents accepted agent filter")
	}
}

func TestExecutionConfigurationSourceDigestPreservesExplicitFields(t *testing.T) {
	execution := json.RawMessage(`{"adapter":"codex-app-server","target":{"kind":"local","environment":""},"settings":{"access_mode":"full_access","model":"selected"}}`)
	for _, operation := range []string{"describe", "derive"} {
		input := map[string]any{"operation": operation, "execution": execution, "workflow": Workflow{ID: "execute", Version: 1}}
		if operation == "derive" {
			input["access_mode"] = "full_access"
		}
		raw, _ := json.Marshal(input)
		out, fault := ConfigureExecution(raw, ExecutionConfigurationSource{})
		if fault != nil {
			t.Fatal(fault)
		}
		identity := out.Identity
		if operation == "derive" {
			identity = out.SourceIdentity
		}
		if identity.SourceExecutionDigest != ConfigurationDigest(execution) {
			t.Fatal("original source fields lost", operation, identity)
		}
	}
	for _, tc := range []struct {
		raw  string
		exit int
	}{{`{}`, 2}, {`{"operation":""}`, 2}, {`{"operation":"unknown"}`, 3}} {
		_, fault := ConfigureExecution([]byte(tc.raw), ExecutionConfigurationSource{})
		if fault == nil || fault.Exit != tc.exit {
			t.Fatal(tc, fault)
		}
	}
}
