package contracts

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

const EnvironmentDocument = "SteerlineFlow/environment.md"
const EnvironmentCheckContract = "steerrunner-checkout-probe/v1"

type EnvironmentCheckInput struct {
	Interpreter    string            `json:"interpreter"`
	Script         string            `json:"script"`
	ManifestSHA256 string            `json:"manifest_sha256"`
	Files          map[string]string `json:"files"`
}

var environmentDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidateEnvironmentCheck(r RunRequest) *Fault {
	c := r.EnvironmentCheck
	invalid := Fail(2, "invalid_input", "Environment check requires a fixed repository, one project entry point, material digests and a result file.")
	if c == nil || r.Workflow.ID != workflow.SingleExecution || r.Workflow.Version != 1 ||
		len(r.Workflow.PromptOverrides) != 0 || r.Workflow.AuditModel != nil || r.Workflow.ProjectPolicySlots != nil ||
		r.ReuseWorktreeFromRunID != "" || r.Conversation != nil || r.Generate != nil || r.ProgressFile != nil ||
		r.ResultFile == nil || r.Publication.Provider != "" || r.Publication.Repository != "" || r.Publication.BaseBranch != "" || len(r.Publication.Settings) != 0 ||
		!validRepository(r.Repository) || !filepath.IsAbs(r.Repository.LocalPath) ||
		!environmentDigest.MatchString(c.ManifestSHA256) || len(c.Files) == 0 || len(c.Files) > 128 {
		return invalid
	}
	if !(c.Interpreter == "powershell.exe" && c.Script == "SteerlineFlow/scripts/verify-environment.ps1" ||
		c.Interpreter == "bash" && c.Script == "SteerlineFlow/scripts/verify-environment.sh") ||
		!environmentDigest.MatchString(c.Files[c.Script]) {
		return invalid
	}
	seen := map[string]bool{strings.ToLower(EnvironmentDocument): true}
	for path, digest := range c.Files {
		lower := strings.ToLower(path)
		if !ValidArtifactPath(path) || seen[lower] || !environmentDigest.MatchString(digest) ||
			strings.ContainsAny(path, "*?[]") || strings.HasPrefix(lower, ".git/") || lower == ".git" {
			return invalid
		}
		seen[lower] = true
	}
	return ValidateExecutionSelection(r.Execution, r.Workflow)
}
