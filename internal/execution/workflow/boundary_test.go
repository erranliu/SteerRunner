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
	prompt, err := RenderInputWithBody(Snapshot{}, input, "Custom implementation instruction")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixed := range []string{"Custom implementation instruction", "Spec: fixed/spec.md", "Plan: fixed/plan.md", "Reference: fixed/reference.md"} {
		if !strings.Contains(prompt, fixed) {
			t.Fatalf("override removed fixed material: %s", fixed)
		}
	}
	prompt, err = RenderPromptWithBody(StepReport, map[string]string{"source_command_id": "original"}, "Custom report instruction")
	if err != nil || !strings.Contains(prompt, ResultContract) || !strings.Contains(prompt, "original") {
		t.Fatal("override removed machine/source contract")
	}
	if DevelopmentFlow != "development/1" || StepPRReady != "pr_ready" {
		t.Fatal("Runner did not own its standalone milestone")
	}
}
func forbidden(path, layer string) bool {
	if strings.Contains(path, "SteerLine") || strings.Contains(path, "/application") || strings.Contains(path, "/runstate") || strings.Contains(path, "/storage") || strings.Contains(path, "/task") {
		return true
	}
	if layer == "reliable" && (strings.Contains(path, "/workflow") || strings.Contains(path, "/publication")) {
		return true
	}
	if layer == "workflow" && (strings.Contains(path, "/adapters") || strings.Contains(path, "/backend")) {
		return true
	}
	return false
}
func TestSRExecutionDependencyBoundary(t *testing.T) {
	for _, layer := range []string{"workflow", "reliable", "backend", "adapters", "workspace", "publication", "process", "evidence", "conversation"} {
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
				if forbidden(value, layer) {
					t.Errorf("%s imports forbidden owner %s", path, value)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, negative := range []struct{ layer, path string }{{"workflow", "github.com/erranliu/SteerRunner/internal/execution/backend"}, {"reliable", "github.com/erranliu/SteerRunner/internal/execution/workflow"}, {"adapters", "github.com/erranliu/SteerRunner/internal/storage"}, {"backend", "github.com/erranliu/SteerLine/internal/task"}} {
		if !forbidden(negative.path, negative.layer) {
			t.Fatal("guard did not catch a forbidden dependency")
		}
	}
}
