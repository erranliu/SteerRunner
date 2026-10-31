package contracts

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

type Repository struct {
	URL          string `json:"url"`
	BaseRevision string `json:"base_revision"`
	Remote       string `json:"remote"`
	BaseBranch   string `json:"base_branch"`
	LocalPath    string `json:"local_path,omitempty"`
}

type Document struct {
	// Development uses only Path. Kind/Content/Commit retain the existing wire
	// shape for callers and for content workflows' inline uploaded materials.
	Kind    string  `json:"kind,omitempty"`
	Path    string  `json:"path"`
	Content *string `json:"content,omitempty"`
	Commit  string  `json:"commit,omitempty"`
}

type Documents struct {
	Spec       Document   `json:"spec"`
	Plan       Document   `json:"plan"`
	References []Document `json:"references,omitempty"`
}

type Workflow struct {
	ID              string                  `json:"id"`
	Version         int                     `json:"version"`
	PromptOverrides map[string]string       `json:"prompt_overrides,omitempty"`
	AuditModel      *workflow.ModelSettings `json:"audit_model,omitempty"`
}

type Target struct {
	Kind        string `json:"kind"`
	Environment string `json:"environment,omitempty"`
}

type Execution struct {
	Adapter  string          `json:"adapter"`
	Target   Target          `json:"target"`
	Settings json.RawMessage `json:"settings"`
}

type Publication struct {
	Provider   string          `json:"provider"`
	Repository string          `json:"repository,omitzero"`
	BaseBranch string          `json:"base_branch"`
	Settings   json.RawMessage `json:"settings"`
}

type RunRequest struct {
	ProgressFile *ProgressFileInput `json:"progress_file,omitempty"`
	ResultFile   *ResultFileInput   `json:"result_file,omitempty"`
	Conversation *ConversationInput `json:"conversation,omitempty"`
	Generate     *GenerateInput     `json:"generate,omitempty"`
	Title        string             `json:"title"`
	NamePrefix   string             `json:"name_prefix,omitempty"`
	Correlation  map[string]string  `json:"correlation,omitempty"`
	Repository   Repository         `json:"repository,omitzero"`
	Documents    Documents          `json:"documents,omitzero"`
	Workflow     Workflow           `json:"workflow"`
	Execution    Execution          `json:"execution"`
	Publication  Publication        `json:"publication,omitzero"`
}

type AgentSettings struct {
	RuntimeSHA256     string   `json:"runtime_sha256,omitempty"`
	RuntimeExecutable string   `json:"runtime_executable,omitempty"`
	Executable        string   `json:"executable,omitempty"`
	Model             string   `json:"model,omitempty"`
	ReasoningEffort   string   `json:"reasoning_effort,omitempty"`
	Args              []string `json:"args,omitempty"`
}

type CodexSettings struct {
	AccessMode      string `json:"access_mode,omitempty"`
	Executable      string `json:"executable,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type TransportSettings struct {
	Executable  string   `json:"executable,omitempty"`
	Args        []string `json:"args,omitempty"`
	WaitTimeout string   `json:"wait_timeout,omitempty"`
}

type OrcaSettings struct {
	Agent       string            `json:"agent,omitempty"`
	AccessMode  string            `json:"access_mode,omitempty"`
	Transport   TransportSettings `json:"transport,omitempty"`
	AgentLaunch AgentSettings     `json:"agent_launch,omitempty"`
}

// AgentName preserves the original Codex default without rewriting old input.
func (s OrcaSettings) AgentName() string {
	if s.Agent == "" {
		return "codex"
	}
	return s.Agent
}

type QuerySettings struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

type CommandProviderSettings struct {
	BaseURL            string        `json:"base_url"`
	Query              QuerySettings `json:"query"`
	AuthenticationRefs []string      `json:"authentication_refs,omitempty"`
}

type GiteaSettings struct {
	BaseURL            string   `json:"base_url"`
	AuthenticationRefs []string `json:"authentication_refs,omitempty"`
}

type RevisionInput struct {
	ExpectedRevision uint64 `json:"expected_revision"`
}

type ReplyInput struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	Kind             string `json:"kind"`
	QuestionID       string `json:"question_id,omitempty"`
	Text             string `json:"text"`
}

var commitPattern = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
var secretArgument = regexp.MustCompile(`(?i)(password|passwd|token|secret|authorization|extraheader|api[._-]?key|private[_-]?key|bearer|^--?user(?:=|$)|^-u$)`)

func plain(value string) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\x00\r\n")
}

func safeURL(value string) bool {
	if !plain(value) {
		return false
	}
	if !strings.Contains(value, "://") {
		return !strings.ContainsAny(value, "?#")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.User != nil {
		_, password := parsed.User.Password()
		return parsed.Scheme == "ssh" && !password
	}
	return true
}

func safeDocument(document Document) bool {
	if !plain(document.Path) || strings.ContainsAny(document.Path, "\\:") || path.IsAbs(document.Path) || path.Clean(document.Path) != document.Path || document.Path == "." || document.Path == ".." || strings.HasPrefix(document.Path, "../") {
		return false
	}
	switch document.Kind {
	case "":
		return document.Content == nil && document.Commit == ""
	case "inline":
		return document.Content != nil && document.Commit == ""
	case "git":
		return document.Content == nil && commitPattern.MatchString(document.Commit)
	default:
		return false
	}
}

func effort(value string) bool {
	switch value {
	case "", "minimal", "low", "medium", "high", "xhigh", "max", "ultra":
		return true
	default:
		return false
	}
}

func optionalStrings(data []byte, names ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return false
	}
	for _, name := range names {
		if raw, present := fields[name]; present {
			var value string
			if json.Unmarshal(raw, &value) != nil || !plain(value) {
				return false
			}
		}
	}
	return true
}

// ValidateRun checks the public shape without reading files, fetching Git, or
// launching native programs. Runtime readiness is a separate admission gate.
func ValidateRun(data []byte) *Fault {
	invalid := Fail(2, "invalid_input", "Run input does not satisfy the embedded configuration contract.")
	var value RunRequest
	if DecodeStrict(data, &value) != nil || !plain(value.Title) {
		return invalid
	}
	if raw := map[string]json.RawMessage{}; json.Unmarshal(data, &raw) == nil {
		for _, name := range []string{"progress_file", "result_file"} {
			if v, ok := raw[name]; ok {
				if string(v) == "null" {
					return invalid
				}
				var fields map[string]json.RawMessage
				if json.Unmarshal(v, &fields) != nil {
					return invalid
				}
				if initial, ok := fields["initial_content_base64"]; !ok || string(initial) == "null" {
					return invalid
				}
			}
		}
	}
	if err := ValidateProgress(value); err != nil {
		return err
	}
	if err := ValidateResultFile(value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	if _, present := fields["name_prefix"]; present && (len(value.NamePrefix) > MaxNamePrefixLength || !namePrefixPattern.MatchString(value.NamePrefix)) {
		return invalid
	}
	if workflow.Content(value.Workflow.ID) {
		return validateContentRun(data, value)
	}
	if value.Conversation != nil || value.Generate != nil || !safeURL(value.Repository.URL) || !commitPattern.MatchString(value.Repository.BaseRevision) || !plain(value.Repository.Remote) || !plain(value.Repository.BaseBranch) {
		return invalid
	}
	if !safeDocument(value.Documents.Spec) || !safeDocument(value.Documents.Plan) || strings.EqualFold(value.Documents.Spec.Path, value.Documents.Plan.Path) {
		return invalid
	}
	for _, document := range value.Documents.References {
		if !safeDocument(document) {
			return invalid
		}
	}
	configuration, _ := json.Marshal(map[string]json.RawMessage{"execution": fields["execution"], "workflow": fields["workflow"]})
	if err := ValidateRunConfiguration(configuration); err != nil {
		return err
	}
	for key := range value.Correlation {
		if secretArgument.MatchString(key) {
			return invalid
		}
	}
	return ValidatePublication(value.Repository, value.Publication)
}

// RunConfiguration is the execution/workflow portion shared by presets and submit.
type RunConfiguration struct {
	Execution Execution `json:"execution"`
	Workflow  Workflow  `json:"workflow"`
}

// ValidateRunConfiguration is pure admission; it performs no environment I/O.
func ValidateRunConfiguration(data []byte) *Fault {
	invalid := Fail(2, "invalid_input", "Run configuration does not satisfy the published contract.")
	var value RunConfiguration
	if DecodeStrict(data, &value) != nil {
		return invalid
	}
	profile, supported := workflow.Lookup(value.Workflow.ID, value.Workflow.Version)
	if !supported {
		return Fail(3, "unsupported_schema", "The selected workflow is not defined by this executable.")
	}
	if workflow.Content(profile.ID) {
		if value.Execution.Adapter != "codex-app-server" || value.Execution.Target.Kind != "local" {
			return Fail(3, "unsupported_capability", "Content workflows require local codex-app-server.")
		}
		var settings CodexSettings
		if DecodeStrict(value.Execution.Settings, &settings) != nil || settings.AccessMode != "read_only" || len(value.Workflow.PromptOverrides) != 0 || value.Workflow.AuditModel != nil {
			return invalid
		}
	} else {
		var settings struct {
			AccessMode string `json:"access_mode"`
		}
		_ = json.Unmarshal(value.Execution.Settings, &settings)
		if settings.AccessMode == "read_only" {
			return invalid
		}
	}
	var requestFields map[string]json.RawMessage
	_ = json.Unmarshal(data, &requestFields)
	var workflowFields map[string]json.RawMessage
	_ = json.Unmarshal(requestFields["workflow"], &workflowFields)
	if workflow.Content(profile.ID) {
		if _, present := workflowFields["prompt_overrides"]; present {
			return invalid
		}
		var settingsFields map[string]json.RawMessage
		_ = json.Unmarshal(value.Execution.Settings, &settingsFields)
		if _, present := settingsFields["github_auth"]; present {
			return invalid
		}
	}
	if raw, present := workflowFields["audit_model"]; present {
		if !profile.AuditModelRequired || value.Workflow.AuditModel == nil || !optionalStrings(raw, "model", "reasoning_effort") {
			return invalid
		}
	}
	if failure := ValidateExecution(value.Execution); failure != nil {
		return failure
	}
	if profile.AuditModelRequired && value.Execution.Adapter == "orca-terminal" && value.Execution.Target.Kind == "orca_environment" {
		return Fail(3, "unsupported_capability", "Native remote Orca does not provide verified audit model or workspace provenance; use development/2.")
	}
	if profile.AuditModelRequired {
		model := value.Workflow.AuditModel
		if model == nil || !plain(model.Model) || strings.TrimSpace(model.Model) != model.Model || strings.ContainsFunc(model.Model, unicode.IsControl) || !effort(model.ReasoningEffort) {
			return invalid
		}
		if !slices.Contains(profile.SupportedAdapters, value.Execution.Adapter) {
			return Fail(3, "unsupported_capability", "The audited workflow requires a managed Codex execution adapter.")
		}
		settings, err := ExecutionModel(value.Execution)
		if err != nil || !plain(settings.Model) || strings.ContainsFunc(settings.Model, unicode.IsControl) || settings.Model == model.Model {
			return invalid
		}
	}
	for step, body := range value.Workflow.PromptOverrides {
		if strings.TrimSpace(body) == "" {
			return invalid
		}
		if !slices.Contains(profile.PromptSteps, step) {
			return invalid
		}
	}
	return ValidateExecution(value.Execution)
}

func ValidateExecution(value Execution) *Fault {
	// Retired settings are rejected even when null: never change auth identity
	// by silently treating a former App request as an execution-host gh login.
	var fields map[string]json.RawMessage
	if json.Unmarshal(value.Settings, &fields) == nil {
		if _, exists := fields["github_auth"]; exists {
			return Fail(2, "invalid_input", "GitHub App authentication is unsupported. Remove github_auth and sign in with gh on the execution host.")
		}
	}
	if value.Adapter == "codex-app-server" || value.Adapter == "orca-terminal" {
		var mode string
		if raw, ok := fields["access_mode"]; !ok || json.Unmarshal(raw, &mode) != nil || mode != "full_access" && !(value.Adapter == "codex-app-server" && mode == "read_only") {
			return Fail(2, "invalid_input", "execution.settings.access_mode must be explicitly full_access for development or read_only for local content; sandbox and implicit permissions are retired. Update the configuration source before submitting a new request.")
		}
	}
	invalid := Fail(2, "invalid_input", "Execution settings do not satisfy the embedded contract.")
	switch value.Adapter {
	case "codex-app-server":
		var settings CodexSettings
		if value.Target.Kind != "local" || value.Target.Environment != "" || DecodeStrict(value.Settings, &settings) != nil || !optionalStrings(value.Settings, "executable", "model", "reasoning_effort") || !effort(settings.ReasoningEffort) || settings.Model != strings.TrimSpace(settings.Model) {
			return invalid
		}
		if !validAccessMode(settings.AccessMode) && settings.AccessMode != "read_only" {
			return invalid
		}
	case "orca-terminal":
		var settings OrcaSettings
		if DecodeStrict(value.Settings, &settings) != nil || !validAccessMode(settings.AccessMode) || !optionalStrings(value.Settings, "agent") {
			return invalid
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(value.Settings, &fields)
		if value.Target.Kind == "orca_environment" {
			var launch map[string]json.RawMessage
			_ = json.Unmarshal(fields["agent_launch"], &launch)
			_, executable := launch["runtime_executable"]
			_, sha := launch["runtime_sha256"]
			if executable || sha {
				return Fail(2, "invalid_input", "Remote Orca uses the native agent; remove agent_launch.runtime_executable and runtime_sha256.")
			}
		}
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`).MatchString(settings.AgentName()) {
			return invalid
		}
		if settings.AgentName() == "codex" && !effort(settings.AgentLaunch.ReasoningEffort) {
			return invalid
		}
		if settings.AgentName() == "pi" && (value.Target.Kind != "local" || len(settings.AgentLaunch.Args) != 0 || !slices.Contains([]string{"", "minimal", "low", "medium", "high", "xhigh"}, settings.AgentLaunch.ReasoningEffort)) {
			return Fail(3, "unsupported_capability", "Pi requires the local managed TUI, fixed tools and a supported thinking level.")
		}
		if settings.AgentName() == "dsh" && (value.Target.Kind != "local" || len(settings.AgentLaunch.Args) != 0 || !slices.Contains([]string{"", "low", "medium", "high", "max"}, settings.AgentLaunch.ReasoningEffort)) {
			return Fail(3, "unsupported_capability", "DSH requires the local managed TUI and fixed native tools.")
		}
		if settings.AgentName() != "codex" && settings.AgentName() != "pi" && settings.AgentName() != "dsh" && (settings.AgentLaunch.Model != "" || settings.AgentLaunch.ReasoningEffort != "" || settings.AgentLaunch.RuntimeExecutable != "" || len(settings.AgentLaunch.Args) != 0) {
			return Fail(3, "unsupported_capability", "This agent has no verified model, argument or execution-host preparation mapping; no Codex settings were applied.")
		}
		if raw, ok := fields["transport"]; ok && !optionalStrings(raw, "executable", "wait_timeout") {
			return invalid
		}
		if raw, ok := fields["agent_launch"]; ok && !optionalStrings(raw, "runtime_executable", "executable", "model", "reasoning_effort") {
			return invalid
		}
		if value.Target.Kind != "local" && value.Target.Kind != "orca_environment" || value.Target.Kind == "local" && value.Target.Environment != "" || value.Target.Kind == "orca_environment" && !plain(value.Target.Environment) {
			return invalid
		}
		if strings.ContainsAny(settings.Transport.Executable+settings.AgentLaunch.Executable+settings.AgentLaunch.RuntimeExecutable+settings.AgentLaunch.Model, "\x00\r\n") {
			return invalid
		}
		if settings.AgentLaunch.RuntimeSHA256 != "" && (!regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(settings.AgentLaunch.RuntimeSHA256) || settings.AgentLaunch.RuntimeExecutable == "") {
			return invalid
		}
		if settings.Transport.WaitTimeout != "" {
			duration, err := time.ParseDuration(settings.Transport.WaitTimeout)
			if err != nil || duration <= 0 || duration > time.Minute {
				return invalid
			}
		}
		for _, arg := range append(append([]string{}, settings.Transport.Args...), settings.AgentLaunch.Args...) {
			if strings.ContainsAny(arg, "\x00\r\n") || secretArgument.MatchString(arg) {
				return invalid
			}
		}
		for _, arg := range settings.AgentLaunch.Args {
			switch arg {
			case "--no-alt-screen", "--no-daemon", "--search":
			default:
				return invalid
			}
		}
		for _, arg := range settings.Transport.Args {
			if strings.HasPrefix(arg, "--environment") || strings.HasPrefix(arg, "--pairing-code") {
				return invalid
			}
		}
	default:
		return Fail(3, "unsupported_capability", "The execution adapter is not defined by this executable.")
	}
	return nil
}

func ValidatePublication(repository Repository, value Publication) *Fault {
	invalid := Fail(2, "invalid_input", "Publication settings do not match the fixed repository.")
	if !plain(value.Repository) || value.BaseBranch != repository.BaseBranch {
		return invalid
	}
	switch value.Provider {
	case "command":
		var settings CommandProviderSettings
		if DecodeStrict(value.Settings, &settings) != nil || !safeURL(settings.BaseURL) || !plain(settings.Query.Executable) || len(settings.Query.Args) == 0 {
			return invalid
		}
		for _, arg := range settings.Query.Args {
			if strings.ContainsAny(arg, "\x00\r\n") || secretArgument.MatchString(arg) {
				return invalid
			}
		}
	case "gitea":
		var settings GiteaSettings
		if DecodeStrict(value.Settings, &settings) != nil || !safeURL(settings.BaseURL) {
			return invalid
		}
	default:
		return Fail(3, "unsupported_capability", "The publication provider is not defined by this executable.")
	}
	return nil
}

func ValidateInput(command string, data []byte) *Fault {
	invalid := Fail(2, "invalid_input", "Input does not satisfy the command contract.")
	switch command {
	case "artifacts collect":
		var value CollectArtifactInput
		if DecodeStrict(data, &value) != nil || value.ExpectedRevision == 0 || !ValidArtifactPath(value.RelativePath) {
			return Fail(2, "artifact_path_invalid", "Expected revision and a portable relative artifact path are required.")
		}
	case "submit":
		return ValidateRun(data)
	case "service stop":
		var value ServiceStopInput
		if DecodeStrict(data, &value) != nil || !plain(value.ExpectedInstanceID) {
			return invalid
		}
	case "resume", "stop", "interrupt", "close":
		var value RevisionInput
		if DecodeStrict(data, &value) != nil || value.ExpectedRevision == 0 {
			return invalid
		}
	case "reply":
		var value ReplyInput
		if DecodeStrict(data, &value) != nil || value.ExpectedRevision == 0 || value.Text == "" || value.Kind != "answer" && value.Kind != "feedback" || value.Kind == "answer" && !plain(value.QuestionID) || value.Kind == "feedback" && value.QuestionID != "" {
			return invalid
		}
	case "message":
		var value MessageInput
		if DecodeStrict(data, &value) != nil || value.ExpectedRevision == 0 || strings.TrimSpace(value.Text) == "" {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}

func validAccessMode(value string) bool {
	return value == "full_access"
}
