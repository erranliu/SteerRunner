package contracts

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const NativeCatalogContract = "steerrunner-native-catalog/v1"
const NativeCatalogInputLimit = 64 << 10
const NativeCatalogOutputLimit = 1 << 20

var nativeAgentID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type NativeCatalogRequest struct {
	Execution  *Execution `json:"execution,omitempty"`
	Kind       string     `json:"kind"`
	Adapter    string     `json:"adapter,omitempty"`
	Target     *Target    `json:"target,omitempty"`
	Agent      string     `json:"agent,omitempty"`
	Executable string     `json:"executable,omitempty"`
}

func ReadNativeCatalog(raw []byte) (NativeCatalogRequest, *Fault) {
	var input NativeCatalogRequest
	invalid := func() (NativeCatalogRequest, *Fault) {
		return input, Fail(2, "invalid_arguments", "Invalid native catalog query.")
	}
	if len(raw) > NativeCatalogInputLimit || DecodeStrict(raw, &input) != nil {
		return invalid()
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for _, value := range fields {
		if string(value) == "null" {
			return invalid()
		}
	}
	if input.Kind == "environments" {
		if len(fields) != 1 {
			return invalid()
		}
		return input, nil
	}
	if input.Kind != "agents" && input.Kind != "models" || input.Target == nil || input.Adapter == "" {
		return invalid()
	}
	if input.Target.Kind != "local" && input.Target.Kind != "orca_environment" || input.Target.Kind == "local" && input.Target.Environment != "" || input.Target.Kind == "orca_environment" && strings.TrimSpace(input.Target.Environment) == "" {
		return invalid()
	}
	if input.Kind == "models" && !nativeAgentID.MatchString(input.Agent) || input.Kind == "agents" && fields["agent"] != nil {
		return invalid()
	}
	if input.Adapter != "orca-terminal" && input.Adapter != "codex-app-server" {
		return input, Fail(3, "unsupported_capability", "Native catalog adapter is unsupported.")
	}
	if input.Execution != nil {
		if fields["executable"] != nil {
			return invalid()
		}
		if fault := validateSource(*input.Execution); fault != nil {
			return input, fault
		}
		view := DescribeExecutionConfiguration(*input.Execution)
		if input.Adapter != view.Adapter || *input.Target != view.Target || input.Kind == "models" && input.Agent != view.Agent {
			return invalid()
		}
	}
	if input.Adapter == "orca-terminal" && fields["executable"] != nil {
		return invalid()
	}
	if input.Adapter == "codex-app-server" && (input.Kind != "models" || input.Target.Kind != "local" || input.Agent != "codex") {
		return input, Fail(3, "unsupported_capability", "Native catalog combination is unsupported.")
	}
	if fields["executable"] != nil && strings.TrimSpace(input.Executable) == "" {
		return invalid()
	}
	return input, nil
}

type NativeCatalogSource struct {
	Version          string `json:"version"`
	ExecutableSHA256 string `json:"executable_sha256"`
	ResourceDigest   string `json:"resource_digest"`
	NativeVersion    string `json:"native_version,omitempty"`
	RuntimeID        string `json:"runtime_id,omitempty"`
	Protocol         int    `json:"protocol,omitempty"`
}
type NativeEnvironment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type NativeAgent struct {
	ID        string `json:"id"`
	Directory string `json:"directory"`
}
type NativeEffort struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type NativeModel struct {
	ID            string         `json:"id"`
	Label         string         `json:"label"`
	Provider      string         `json:"provider,omitempty"`
	Default       bool           `json:"is_default"`
	DefaultEffort string         `json:"default_effort,omitempty"`
	Efforts       []NativeEffort `json:"efforts"`
}
type NativeCatalogResult struct {
	Contract          string               `json:"contract"`
	Kind              string               `json:"kind"`
	Adapter           string               `json:"adapter,omitempty"`
	Target            *Target              `json:"target,omitempty"`
	Agent             string               `json:"agent,omitempty"`
	State             string               `json:"state"`
	Complete          bool                 `json:"complete"`
	ObservedAt        time.Time            `json:"observed_at"`
	ContextUnverified bool                 `json:"context_unverified"`
	Source            NativeCatalogSource  `json:"source"`
	Origin            string               `json:"origin"`
	NativeFetchedAt   *time.Time           `json:"native_fetched_at,omitempty"`
	ListingInProgress bool                 `json:"listing_in_progress"`
	Environments      *[]NativeEnvironment `json:"environments,omitempty"`
	Agents            *[]NativeAgent       `json:"agents,omitempty"`
	Models            *[]NativeModel       `json:"models,omitempty"`
}
