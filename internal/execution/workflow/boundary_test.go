package workflow

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSRWorkflowDevelopment(t *testing.T) {
	input := StageInput{StepID: StepImplement, MaterialFacts: []byte(`{"spec":{"path":"fixed/spec.md"},"plan":{"path":"fixed/plan.md"},"references":[{"path":"fixed/reference.md"}]}`)}
	p, _ := Lookup("development", 2)
	prompt, err := RenderInputWithBody(Snapshot{Config: &p}, input, "Custom implementation instruction")
	if err != nil {
		t.Fatal(err)
	}
	const want = "Custom implementation instruction\nSpec: fixed/spec.md\nPlan: fixed/plan.md\n末行写「实施及必要验证已完成」或「未完成」。"
	if prompt != want {
		t.Fatalf("unexpected plan-driven input: %q", prompt)
	}
	prompt, err = RenderPromptWithBody(StepReport, map[string]string{"source_command_id": "original"}, "Custom report instruction")
	if err != nil || strings.Contains(prompt, "JSON") || !strings.Contains(prompt, "original") || !strings.Contains(prompt, "成功时末行：实施及必要验证已完成") {
		t.Fatal("override removed machine/source contract")
	}
	if DevelopmentFlow != "development/2" || StepPRReady != "pr_ready" {
		t.Fatal("Runner did not own its standalone milestone")
	}
}
func forbidden(path, layer string) bool {
	if layer == "orcacapability" && strings.HasPrefix(path, "github.com/erranliu/SteerRunner/internal/") {
		const capability = "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
		return path != capability && !strings.HasPrefix(path, capability+"/")
	}
	if strings.Contains(path, "SteerLine") || strings.Contains(path, "/application") || strings.Contains(path, "/runstate") || strings.Contains(path, "/storage") || strings.Contains(path, "/task") {
		return true
	}
	if layer == "reliable" && (strings.Contains(path, "/workflow") || strings.Contains(path, "/publication") || strings.Contains(path, "/adapters")) {
		return true
	}
	if layer == "backend" && (strings.Contains(path, "/reliable") || strings.Contains(path, "/adapters") || strings.Contains(path, "/publication") || strings.Contains(path, "/resultacquisition")) {
		return true
	}
	if layer == "model" && strings.Contains(path, "/internal/") {
		return true
	}
	if (layer == "adapters" || layer == "backend" || layer == "model") && strings.Contains(path, "/workflow") {
		return true
	}
	if layer == "workflow" && (strings.Contains(path, "/adapters") || strings.Contains(path, "/backend") || strings.Contains(path, "/reliable/operation")) {
		return true
	}
	return false
}
func TestSRExecutionDependencyBoundary(t *testing.T) {
	layers, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	// Discover every current execution owner, including newly added packages.
	for _, entry := range layers {
		if !entry.IsDir() {
			continue
		}
		layer := entry.Name()
		root := filepath.Join("..", layer)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				value, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if forbidden(value, layer) || strings.Contains(filepath.ToSlash(path), "/adapters/protocol/orca/") && forbidden(value, "orcacapability") {
					t.Errorf("%s imports forbidden owner %s", path, value)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, negative := range []struct{ layer, path string }{{"orcacapability", "github.com/erranliu/SteerRunner/internal/execution/backend"}, {"orcacapability", "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca_legacy"}, {"orcacapability", "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"}, {"orcacapability", "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"}, {"backend", "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"}, {"model", "github.com/erranliu/SteerRunner/internal/execution/workflow"}, {"adapters", "github.com/erranliu/SteerRunner/internal/execution/workflow"}, {"backend", "github.com/erranliu/SteerRunner/internal/execution/workflow"}, {"workflow", "github.com/erranliu/SteerRunner/internal/execution/backend"}, {"workflow", "github.com/erranliu/SteerRunner/internal/execution/reliable/operation"}, {"reliable", "github.com/erranliu/SteerRunner/internal/execution/workflow"}, {"reliable", "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"}, {"adapters", "github.com/erranliu/SteerRunner/internal/storage"}, {"backend", "github.com/erranliu/SteerLine/internal/task"}} {
		if !forbidden(negative.path, negative.layer) {
			t.Fatal("guard did not catch a forbidden dependency")
		}
	}
}
