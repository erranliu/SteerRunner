package testprotocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func TestStagedFixtureConsumesRenderedPlanDrivenImplementation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("STEERRUNNER_SUCCESS_OUTPUT", "1")
	t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "")
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("Git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "main")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "base")
	base := git("rev-parse", "HEAD")
	prompt, err := workflow.RenderPrompt(workflow.StepImplement, map[string]any{"materials": map[string]any{"spec": workflow.FileReference{Path: "spec.md"}, "plan": workflow.FileReference{Path: "plan.md"}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := StagedBusiness(root, prompt, "unused-provider", "unused-remote")
	if err != nil || !strings.HasSuffix(result, "实施及必要验证已完成") {
		t.Fatal("current rendered implementation was not recognized", result, err)
	}
	if git("rev-parse", "HEAD") == base {
		t.Fatal("implementation fixture did not commit its original work")
	}
	if _, err := os.Stat(filepath.Join(root, "stage-result.txt")); err != nil {
		t.Fatal(err)
	}
}
