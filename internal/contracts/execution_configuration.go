package contracts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
)

const ExecutionConfigurationContract = "steerrunner-execution-configuration/v1"
const ExecutionIdentityContract = "steerrunner-execution-identity/v1"
const ExecutionConfigurationLimit = 1 << 20

type ExecutionSelection struct {
	Adapter         string  `json:"adapter"`
	Target          Target  `json:"target"`
	Agent           string  `json:"agent"`
	Model           *string `json:"model,omitempty"`
	ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	AccessMode      string  `json:"access_mode"`
}
type ExecutionIdentity struct {
	Contract              string    `json:"contract"`
	SourceExecutionDigest string    `json:"source_execution_digest"`
	Context               Execution `json:"context"`
}
type ExecutionConfigurationRequest struct {
	Operation     string              `json:"operation"`
	Execution     *Execution          `json:"execution,omitempty"`
	Workflow      Workflow            `json:"workflow"`
	AccessMode    string              `json:"access_mode,omitempty"`
	BaseExecution *Execution          `json:"base_execution,omitempty"`
	Selection     *ExecutionSelection `json:"selection,omitempty"`
	Unset         []string            `json:"unset,omitempty"`
}
type ExecutionConfigurationSource struct {
	Version          string `json:"version"`
	ExecutableSHA256 string `json:"executable_sha256"`
	ResourceDigest   string `json:"resource_digest"`
}
type ExecutionConfigurationResult struct {
	Contract       string                       `json:"contract"`
	Operation      string                       `json:"operation"`
	InputDigest    string                       `json:"input_digest"`
	Source         ExecutionConfigurationSource `json:"source"`
	Execution      *Execution                   `json:"execution,omitempty"`
	View           ExecutionSelection           `json:"view"`
	Identity       *ExecutionIdentity           `json:"identity,omitempty"`
	SourceIdentity *ExecutionIdentity           `json:"source_identity,omitempty"`
}

// CanonicalJSON preserves number spelling and array order, sorts object keys and
// rejects duplicate keys before decoding can erase their ambiguity.
func CanonicalJSON(raw []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := DecodeStrict(raw, &fields); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
func ConfigurationDigest(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	raw, err = CanonicalJSON(raw)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func DescribeExecutionConfiguration(e Execution) ExecutionSelection {
	var settings struct {
		Agent  string  `json:"agent"`
		Model  *string `json:"model"`
		Effort *string `json:"reasoning_effort"`
		Mode   string  `json:"access_mode"`
		Launch struct {
			Model  *string `json:"model"`
			Effort *string `json:"reasoning_effort"`
		} `json:"agent_launch"`
	}
	_ = json.Unmarshal(e.Settings, &settings)
	view := ExecutionSelection{Adapter: e.Adapter, Target: e.Target, Agent: "codex", Model: settings.Model, ReasoningEffort: settings.Effort, AccessMode: settings.Mode}
	if e.Adapter == "orca-terminal" {
		if settings.Agent != "" {
			view.Agent = settings.Agent
		}
		view.Model, view.ReasoningEffort = settings.Launch.Model, settings.Launch.Effort
	}
	return view
}
func executionIdentity(e Execution) *ExecutionIdentity {
	context := e
	var settings map[string]json.RawMessage
	_ = json.Unmarshal(e.Settings, &settings)
	if e.Adapter == "codex-app-server" {
		delete(settings, "model")
		delete(settings, "reasoning_effort")
	} else if raw, ok := settings["agent_launch"]; ok {
		var launch map[string]json.RawMessage
		_ = json.Unmarshal(raw, &launch)
		delete(launch, "model")
		delete(launch, "reasoning_effort")
		if len(launch) == 0 {
			delete(settings, "agent_launch")
		} else {
			settings["agent_launch"], _ = json.Marshal(launch)
		}
	}
	context.Settings, _ = json.Marshal(settings)
	return &ExecutionIdentity{Contract: ExecutionIdentityContract, SourceExecutionDigest: ConfigurationDigest(e), Context: context}
}

func ConfigureExecution(raw []byte, source ExecutionConfigurationSource) (ExecutionConfigurationResult, *Fault) {
	var in ExecutionConfigurationRequest
	invalid := func() (ExecutionConfigurationResult, *Fault) {
		return ExecutionConfigurationResult{}, Fail(2, "invalid_input", "Invalid execution configuration request.")
	}
	if len(raw) > ExecutionConfigurationLimit || DecodeStrict(raw, &in) != nil {
		return invalid()
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for _, v := range fields {
		if bytes.Equal(v, []byte("null")) {
			return invalid()
		}
	}
	allowed := []string{"operation", "workflow", "execution"}
	if in.Operation == "" {
		return invalid()
	}
	switch in.Operation {
	case "describe":
	case "derive":
		allowed = append(allowed, "access_mode")
	case "compose":
		allowed = []string{"operation", "workflow", "base_execution", "selection", "unset"}
	default:
		return ExecutionConfigurationResult{}, Fail(3, "unsupported_capability", "Execution configuration operation is unsupported.")
	}
	for name := range fields {
		if !slices.Contains(allowed, name) {
			return invalid()
		}
	}
	if in.Workflow.ID == "" || in.Workflow.Version <= 0 {
		return invalid()
	}
	if in.Operation != "compose" {
		var selected map[string]json.RawMessage
		_ = json.Unmarshal(fields["workflow"], &selected)
		if len(selected) != 2 || in.Execution == nil {
			return invalid()
		}
	}
	out := ExecutionConfigurationResult{Contract: ExecutionConfigurationContract, Operation: in.Operation, InputDigest: ConfigurationDigest(json.RawMessage(raw)), Source: source}
	var execution Execution
	if in.Operation == "compose" {
		if in.Selection == nil || in.Selection.Agent == "" || in.Selection.AccessMode == "" {
			return invalid()
		}
		var selection map[string]json.RawMessage
		_ = json.Unmarshal(fields["selection"], &selection)
		for _, v := range selection {
			if bytes.Equal(v, []byte("null")) {
				return invalid()
			}
		}
		v := in.Selection
		execution = Execution{Adapter: v.Adapter, Target: v.Target, Settings: json.RawMessage(`{}`)}
		if in.BaseExecution != nil {
			if fault := validateSource(*in.BaseExecution); fault != nil {
				return out, fault
			}
			base := DescribeExecutionConfiguration(*in.BaseExecution)
			if base.Adapter != v.Adapter || base.Target != v.Target || base.Agent != v.Agent {
				return invalid()
			}
			execution = *in.BaseExecution
		} else if v.Adapter == "codex-app-server" && v.Agent != "codex" {
			return invalid()
		}
		var settings map[string]json.RawMessage
		_ = json.Unmarshal(execution.Settings, &settings)
		put := func(m map[string]json.RawMessage, k string, value any) { m[k], _ = json.Marshal(value) }
		put(settings, "access_mode", v.AccessMode)
		modelSettings := settings
		if v.Adapter == "orca-terminal" {
			if in.BaseExecution == nil {
				put(settings, "agent", v.Agent)
			}
			modelSettings = map[string]json.RawMessage{}
			if launch, ok := settings["agent_launch"]; ok {
				_ = json.Unmarshal(launch, &modelSettings)
			}
		}
		if v.Model != nil {
			put(modelSettings, "model", *v.Model)
		}
		if v.ReasoningEffort != nil {
			put(modelSettings, "reasoning_effort", *v.ReasoningEffort)
		}
		seen := map[string]bool{}
		for _, key := range in.Unset {
			if seen[key] || key != "model" && key != "reasoning_effort" || key == "model" && v.Model != nil || key == "reasoning_effort" && v.ReasoningEffort != nil {
				return invalid()
			}
			seen[key] = true
			delete(modelSettings, key)
		}
		if v.Adapter == "orca-terminal" {
			_, hadLaunch := settings["agent_launch"]
			if len(modelSettings) > 0 || hadLaunch {
				settings["agent_launch"], _ = json.Marshal(modelSettings)
			} else {
				delete(settings, "agent_launch")
			}
		}
		execution.Settings, _ = json.Marshal(settings)
		configuration, _ := json.Marshal(RunConfiguration{Execution: execution, Workflow: in.Workflow})
		if fault := ValidateRunConfiguration(configuration); fault != nil {
			return out, fault
		}
	} else {
		execution = *in.Execution
		if in.Operation == "derive" {
			if fault := validateSource(execution); fault != nil {
				return out, fault
			}
			out.SourceIdentity = executionIdentity(execution)
			out.SourceIdentity.SourceExecutionDigest = ConfigurationDigest(fields["execution"])
			view := DescribeExecutionConfiguration(execution)
			switch {
			case in.Workflow.ID == "execute" && in.Workflow.Version == 1:
				if in.AccessMode != "full_access" || view.AccessMode != in.AccessMode {
					return invalid()
				}
			case in.Workflow.ID == "generate" && in.Workflow.Version == 1:
				if execution.Adapter != "codex-app-server" || execution.Target.Kind != "local" {
					return out, Fail(3, "unsupported_capability", "Generation requires local Codex.")
				}
				if in.AccessMode != "read_only" {
					return invalid()
				}
				var values map[string]json.RawMessage
				_ = json.Unmarshal(execution.Settings, &values)
				settings := map[string]json.RawMessage{"access_mode": json.RawMessage(`"read_only"`)}
				for _, key := range []string{"executable", "model", "reasoning_effort"} {
					if v, ok := values[key]; ok {
						settings[key] = v
					}
				}
				execution.Settings, _ = json.Marshal(settings)
			default:
				return out, Fail(3, "unsupported_capability", "Execution derivation target is unsupported.")
			}
		}
		if fault := ValidateExecutionSelection(execution, in.Workflow); fault != nil {
			return out, fault
		}
	}
	out.View = DescribeExecutionConfiguration(execution)
	if in.Operation != "derive" {
		out.Identity = executionIdentity(execution)
		if in.Operation == "describe" {
			out.Identity.SourceExecutionDigest = ConfigurationDigest(fields["execution"])
		}
	}
	if in.Operation != "describe" {
		out.Execution = &execution
	}
	return out, nil
}

func validateSource(execution Execution) *Fault {
	id, version := "development", 2
	if DescribeExecutionConfiguration(execution).AccessMode == "read_only" {
		id, version = "generate", 1
	}
	return ValidateExecutionSelection(execution, Workflow{ID: id, Version: version})
}
