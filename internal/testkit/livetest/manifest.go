// Package livetest owns test-only real-environment contracts. Production never imports it.
package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

const Repository = "erranliu/SteerlineTest"
const RemoteURL = "https://github.com/erranliu/SteerlineTest.git"
const Artifact = "evidence/native-101.md"

type Manifest struct {
	Schema          string                `json:"schema"`
	ID              string                `json:"id"`
	Level           string                `json:"level"`
	Scenario        string                `json:"scenario"`
	Driver          string                `json:"driver"`
	Root            string                `json:"root"`
	RunnerExe       string                `json:"runner_exe"`
	RunnerSHA256    string                `json:"runner_sha256"`
	SourceCommit    string                `json:"source_commit"`
	SourceDirty     bool                  `json:"source_dirty"`
	RepositoryPath  string                `json:"repository_path"`
	SourceRevision  string                `json:"source_revision"`
	BaseBranch      string                `json:"base_branch"`
	BaseRevision    string                `json:"base_revision"`
	SpecPath        string                `json:"spec_path"`
	SpecSHA256      string                `json:"spec_sha256"`
	PlanPath        string                `json:"plan_path"`
	PlanSHA256      string                `json:"plan_sha256"`
	Execution       contracts.Execution   `json:"execution"`
	Publication     contracts.Publication `json:"publication"`
	RemoteRuntimeID string                `json:"remote_runtime_id,omitempty"`
	CreatedAt       string                `json:"created_at"`
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{12}$`)
var scenarioPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]+$`)
var revisionPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

func (m Manifest) Validate() error {
	createdAt, createdErr := time.Parse(time.RFC3339Nano, m.CreatedAt)
	if m.Schema != "steerrunner-test/v1" || !idPattern.MatchString(m.ID) || !scenarioPattern.MatchString(m.Scenario) || m.Driver == "" || createdErr != nil || createdAt.IsZero() {
		return errors.New("invalid live fixture identity")
	}
	if m.BaseBranch != "codex/srtest/"+m.ID+"/base" || !revisionPattern.MatchString(m.BaseRevision) || !revisionPattern.MatchString(m.SourceRevision) || !revisionPattern.MatchString(m.SourceCommit) {
		return errors.New("fixture must freeze source revisions and its dedicated test base")
	}
	if m.Publication.Repository != Repository || m.Publication.BaseBranch != m.BaseBranch {
		return errors.New("live publication is restricted to the dedicated SteerlineTest base")
	}
	switch m.Level {
	case "L2":
		if m.Execution.Adapter != "codex-app-server" || m.Execution.Target.Kind != "local" || m.Execution.Target.Environment != "" || m.RemoteRuntimeID != "" {
			return errors.New("L2 requires local Codex")
		}
	case "L3":
		// Additional real-environment drivers can be registered without changing levels.
		if m.Execution.Adapter == "orca-terminal" && (m.Execution.Target.Kind != "orca_environment" || m.Execution.Target.Environment == "" || m.RemoteRuntimeID == "") {
			return errors.New("remote Orca requires a fixed environment and runtime")
		}
	default:
		return errors.New("live fixtures require L2 or L3")
	}
	if !filepath.IsAbs(m.Root) {
		return errors.New("fixture root must be absolute")
	}
	root, err := filepath.EvalSymlinks(m.Root)
	if err != nil || !strings.EqualFold(filepath.Clean(root), filepath.Clean(m.Root)) {
		return errors.New("fixture root is missing or redirected")
	}
	for _, path := range []string{m.RunnerExe, m.RepositoryPath, m.SpecPath, m.PlanPath} {
		actual, err := filepath.EvalSymlinks(path)
		rel, relErr := filepath.Rel(root, actual)
		if err != nil || relErr != nil || !filepath.IsAbs(path) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || !strings.EqualFold(filepath.Clean(path), actual) {
			return fmt.Errorf("fixture input escapes or redirects its root: %s", path)
		}
	}
	for path, want := range map[string]string{m.RunnerExe: m.RunnerSHA256, m.SpecPath: m.SpecSHA256, m.PlanPath: m.PlanSHA256} {
		body, err := os.ReadFile(path)
		sum := sha256.Sum256(body)
		if err != nil || hex.EncodeToString(sum[:]) != strings.ToLower(want) {
			return fmt.Errorf("frozen fixture input changed: %s", path)
		}
	}
	return nil
}

// ValidateArtifact is also used on Git objects fetched from the PR host, including remote runs.
func ValidateArtifact(body []byte, earliest, latest time.Time) (time.Time, error) {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) != 6 || lines[0] != "# Evidence sample 101" || lines[1] != "- sample_id: 101" || lines[3] != "- 模拟任务" || lines[4] != "- 未修改业务代码" || lines[5] != "" {
		return time.Time{}, errors.New("artifact differs from the five-line Spec")
	}
	stamp := strings.TrimPrefix(lines[2], "- generated_at: ")
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil || !strings.HasSuffix(stamp, "Z") || at.Before(earliest) || at.After(latest) {
		return time.Time{}, errors.New("artifact generation time is invalid or stale")
	}
	return at, nil
}

// CheckPR is a fail-closed guard immediately before a real merge.
func CheckPR(m Manifest, view contracts.RunView) error {
	p := view.PR
	if view.Status != "pr_ready" || p == nil || p.QueryError != "" || p.Repository != Repository || p.HeadRepository != Repository || p.BaseBranch != m.BaseBranch || p.HeadBranch == "" || p.HeadBranch != view.Execution.Branch || !revisionPattern.MatchString(p.HeadRevision) || p.Number <= 0 || p.URL != fmt.Sprintf("https://github.com/%s/pull/%d", Repository, p.Number) || p.State != "open" || p.Merged {
		return errors.New("PR is not a verified open PR on this fixture's exact repository/base/head")
	}
	f := view.Final
	if f == nil || !f.Verified || f.Source == "" || f.ItemID == "" || f.SessionID == "" || f.SessionID != view.Execution.SessionID || f.AttemptID != view.Execution.AttemptID || f.CommandID == "" || f.TurnID == "" || f.TurnID != view.Execution.TurnID {
		return errors.New("original native final provenance is incomplete")
	}
	if m.RemoteRuntimeID != "" && !strings.HasPrefix(f.SessionID, "orca-terminal:"+m.RemoteRuntimeID+":") {
		return errors.New("native session belongs to a different remote runtime")
	}
	return nil
}
