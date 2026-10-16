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
	Kind    string  `json:"kind"`
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
	Repository string          `json:"repository"`
	BaseBranch string          `json:"base_branch"`
	Settings   json.RawMessage `json:"settings"`
}

type RunRequest struct {
	Title       string            `json:"title"`
	Correlation map[string]string `json:"correlation,omitempty"`
	Repository  Repository        `json:"repository"`
	Documents   Documents         `json:"documents"`
	Workflow    Workflow          `json:"workflow"`
	Execution   Execution         `json:"execution"`
	Publication Publication       `json:"publication"`
}

type AgentSettings struct {
	Executable      string   `json:"executable,omitempty"`
	Model           string   `json:"model,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
	Args            []string `json:"args,omitempty"`
}

type CodexSettings struct {
	Executable      string              `json:"executable,omitempty"`
	Model           string              `json:"model,omitempty"`
	ReasoningEffort string              `json:"reasoning_effort,omitempty"`
	GitHubAuth      *GitHubAppReference `json:"github_auth,omitempty"`
}

type GitHubAppReference struct {
	AppID          int64  `json:"app_id"`
	InstallationID int64  `json:"installation_id"`
	PrivateKeyEnv  string `json:"private_key_env"`
}

type TransportSettings struct {
	Executable  string   `json:"executable,omitempty"`
	Args        []string `json:"args,omitempty"`
	WaitTimeout string   `json:"wait_timeout,omitempty"`
}

type OrcaSettings struct {
	Transport   TransportSettings `json:"transport,omitempty"`
	AgentLaunch AgentSettings     `json:"agent_launch,omitempty"`
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
	if DecodeStrict(data, &value) != nil || !plain(value.Title) || !safeURL(value.Repository.URL) || !commitPattern.MatchString(value.Repository.BaseRevision) || !plain(value.Repository.Remote) || !plain(value.Repository.BaseBranch) {
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
	profile, supported := workflow.Lookup(value.Workflow.ID, value.Workflow.Version)
	if !supported {
		return Fail(3, "unsupported_schema", "The selected workflow is not defined by this executable.")
	}
	var requestFields map[string]json.RawMessage
	_ = json.Unmarshal(data, &requestFields)
	var workflowFields map[string]json.RawMessage
	_ = json.Unmarshal(requestFields["workflow"], &workflowFields)
	if raw, present := workflowFields["audit_model"]; present {
		if !profile.AuditModelRequired || value.Workflow.AuditModel == nil || !optionalStrings(raw, "model", "reasoning_effort") {
			return invalid
		}
	}
	if profile.AuditModelRequired {
		model := value.Workflow.AuditModel
		if model == nil || !plain(model.Model) || strings.TrimSpace(model.Model) != model.Model || strings.ContainsFunc(model.Model, unicode.IsControl) || !effort(model.ReasoningEffort) {
			return invalid
		}
		if !slices.Contains(profile.SupportedAdapters, value.Execution.Adapter) || value.Execution.Target.Kind != "local" || value.Execution.Target.Environment != "" {
			return Fail(3, "unsupported_capability", "The audited workflow requires local codex-app-server.")
		}
		var settings CodexSettings
		if DecodeStrict(value.Execution.Settings, &settings) != nil || !plain(settings.Model) || strings.ContainsFunc(settings.Model, unicode.IsControl) {
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
	for key := range value.Correlation {
		if secretArgument.MatchString(key) {
			return invalid
		}
	}
	switch value.Execution.Adapter {
	case "codex-app-server":
		var settings CodexSettings
		if value.Execution.Target.Kind != "local" || value.Execution.Target.Environment != "" || DecodeStrict(value.Execution.Settings, &settings) != nil || !optionalStrings(value.Execution.Settings, "executable", "model", "reasoning_effort") || !effort(settings.ReasoningEffort) || settings.Model != strings.TrimSpace(settings.Model) {
			return invalid
		}
		if settings.GitHubAuth != nil && (settings.GitHubAuth.AppID <= 0 || settings.GitHubAuth.InstallationID <= 0 || !regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`).MatchString(settings.GitHubAuth.PrivateKeyEnv)) {
			return invalid
		}
	case "orca-terminal":
		var settings OrcaSettings
		if DecodeStrict(value.Execution.Settings, &settings) != nil || !effort(settings.AgentLaunch.ReasoningEffort) {
			return invalid
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(value.Execution.Settings, &fields)
		if raw, ok := fields["transport"]; ok && !optionalStrings(raw, "executable", "wait_timeout") {
			return invalid
		}
		if raw, ok := fields["agent_launch"]; ok && !optionalStrings(raw, "executable", "model", "reasoning_effort") {
			return invalid
		}
		if value.Execution.Target.Kind != "local" && value.Execution.Target.Kind != "orca_environment" || value.Execution.Target.Kind == "local" && value.Execution.Target.Environment != "" || value.Execution.Target.Kind == "orca_environment" && !plain(value.Execution.Target.Environment) {
			return invalid
		}
		if strings.ContainsAny(settings.Transport.Executable+settings.AgentLaunch.Executable+settings.AgentLaunch.Model, "\x00\r\n") {
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
		for _, arg := range settings.Transport.Args {
			if strings.HasPrefix(arg, "--environment") || strings.HasPrefix(arg, "--pairing-code") {
				return invalid
			}
		}
	default:
		return Fail(3, "unsupported_capability", "The execution adapter is not defined by this executable.")
	}
	if !plain(value.Publication.Repository) || value.Publication.BaseBranch != value.Repository.BaseBranch {
		return invalid
	}
	switch value.Publication.Provider {
	case "command":
		var settings CommandProviderSettings
		if DecodeStrict(value.Publication.Settings, &settings) != nil || !safeURL(settings.BaseURL) || !plain(settings.Query.Executable) || len(settings.Query.Args) == 0 {
			return invalid
		}
		for _, arg := range settings.Query.Args {
			if strings.ContainsAny(arg, "\x00\r\n") || secretArgument.MatchString(arg) {
				return invalid
			}
		}
	case "gitea":
		var settings GiteaSettings
		if DecodeStrict(value.Publication.Settings, &settings) != nil || !safeURL(settings.BaseURL) {
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
	case "submit":
		return ValidateRun(data)
	case "service stop":
		var value ServiceStopInput
		if DecodeStrict(data, &value) != nil || !plain(value.ExpectedInstanceID) {
			return invalid
		}
	case "resume", "stop":
		var value RevisionInput
		if DecodeStrict(data, &value) != nil || value.ExpectedRevision == 0 {
			return invalid
		}
	case "reply":
		var value ReplyInput
		if DecodeStrict(data, &value) != nil || value.ExpectedRevision == 0 || value.Text == "" || value.Kind != "answer" && value.Kind != "feedback" || value.Kind == "answer" && !plain(value.QuestionID) || value.Kind == "feedback" && value.QuestionID != "" {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}
