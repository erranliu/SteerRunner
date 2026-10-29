package contracts

import (
	"encoding/json"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"path/filepath"
	"strings"
)

type DirectoryAccess struct {
	Path   string `json:"path"`
	Access string `json:"access"`
}
type ConversationInput struct {
	Instruction string          `json:"instruction"`
	Materials   []Document      `json:"materials"`
	Directory   DirectoryAccess `json:"directory"`
}
type GenerateInput struct {
	Repository   *GenerateRepository `json:"repository,omitempty"`
	Instruction  string              `json:"instruction"`
	Materials    []Document          `json:"materials"`
	Directory    *DirectoryAccess    `json:"directory,omitempty"`
	OutputSchema json.RawMessage     `json:"output_schema,omitempty"`
}
type MessageInput struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	Text             string `json:"text"`
}

const MaxGenerateResultBytes = 1 << 20
const MaxGenerateRepositoryBytes = 64 << 20
const MaxGenerateRepositoryFiles = 4096
const MaxGenerateRepositoryManifestBytes = 1 << 20

type GenerateRepository struct {
	LocalPath string   `json:"local_path"`
	Commit    string   `json:"commit"`
	Paths     []string `json:"paths,omitempty"`
}

type RepositoryFile struct {
	Path      string `json:"path"`
	Blob      string `json:"blob"`
	SHA256    string `json:"sha256"`
	SizeBytes int    `json:"size_bytes"`
}

type RepositorySource struct {
	LocalPath string           `json:"local_path"`
	Commit    string           `json:"commit"`
	Tree      string           `json:"tree"`
	Files     []RepositoryFile `json:"files"`
}

func (r RunRequest) ContentInput() (string, []Document, *DirectoryAccess) {
	if r.Conversation != nil {
		return r.Conversation.Instruction, r.Conversation.Materials, &r.Conversation.Directory
	}
	if r.Generate != nil {
		return r.Generate.Instruction, r.Generate.Materials, r.Generate.Directory
	}
	return "", nil, nil
}

func validateContentRun(data []byte, r RunRequest) *Fault {
	invalid := Fail(2, "invalid_input", "Content input must match its workflow, inline materials and explicit read-only directory access.")
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	for _, key := range []string{"repository", "documents", "publication"} {
		if _, ok := fields[key]; ok {
			return invalid
		}
	}
	if r.Workflow.ID == "conversation" && (r.Conversation == nil || r.Generate != nil) || r.Workflow.ID == "generate" && (r.Generate == nil || r.Conversation != nil) {
		return invalid
	}
	instruction, materials, directory := r.ContentInput()
	var contentFields map[string]json.RawMessage
	_ = json.Unmarshal(fields[r.Workflow.ID], &contentFields)
	if _, present := contentFields["materials"]; !present {
		return invalid
	}
	if raw, present := contentFields["directory"]; present && string(raw) == "null" {
		return invalid
	}
	if strings.TrimSpace(instruction) == "" || strings.ContainsRune(instruction, 0) {
		return invalid
	}
	if directory != nil && (!filepath.IsAbs(directory.Path) || directory.Access != "read_only" || !plain(directory.Path)) {
		return invalid
	}
	seen := map[string]bool{}
	for _, d := range materials {
		if !safeDocument(d) || d.Kind != "inline" || seen[strings.ToLower(d.Path)] {
			return invalid
		}
		seen[strings.ToLower(d.Path)] = true
	}
	for k := range r.Correlation {
		if secretArgument.MatchString(k) {
			return invalid
		}
	}
	if r.Generate != nil && len(r.Generate.OutputSchema) > 0 {
		if _, err := ra.CompileOutputSchema(r.Generate.OutputSchema); err != nil {
			return Fail(2, "invalid_output_schema", err.Error())
		}
	}
	if r.Generate != nil {
		if raw, present := contentFields["repository"]; present && string(raw) == "null" {
			return invalid
		}
		if repo := r.Generate.Repository; repo != nil {
			var repositoryFields map[string]json.RawMessage
			_ = json.Unmarshal(contentFields["repository"], &repositoryFields)
			if raw, present := repositoryFields["paths"]; present && string(raw) == "null" {
				return invalid
			}
			if repo.Paths != nil {
				if len(repo.Paths) == 0 || len(repo.Paths) > MaxGenerateRepositoryFiles {
					return invalid
				}
				selected := map[string]bool{}
				for _, path := range repo.Paths {
					if !ValidArtifactPath(path) || selected[strings.ToLower(path)] {
						return invalid
					}
					selected[strings.ToLower(path)] = true
				}
			}
			if directory != nil || !filepath.IsAbs(repo.LocalPath) || !plain(repo.LocalPath) || !commitPattern.MatchString(repo.Commit) || len(r.Generate.OutputSchema) == 0 {
				return invalid
			}
		}
	}
	configuration, _ := json.Marshal(map[string]json.RawMessage{"workflow": fields["workflow"], "execution": fields["execution"]})
	return ValidateRunConfiguration(configuration)
}

type ContentResult struct {
	RepositorySource *RepositorySource `json:"repository_source,omitempty"`
	Status           string            `json:"status"`
	Text             string            `json:"text,omitempty"`
	JSON             json.RawMessage   `json:"json,omitempty"`
	Diagnostic       string            `json:"diagnostic,omitempty"`
	Source           FinalView         `json:"source"`
}
type HistoryEntry struct {
	RequestCommandID string         `json:"request_command_id,omitempty"`
	CommandID        string         `json:"command_id"`
	SessionID        string         `json:"session_id,omitempty"`
	TurnID           string         `json:"turn_id,omitempty"`
	Role             string         `json:"role"`
	Text             string         `json:"text,omitempty"`
	Question         *QuestionView  `json:"question,omitempty"`
	Result           *ContentResult `json:"result,omitempty"`
}
