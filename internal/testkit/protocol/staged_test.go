package testprotocol

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/resources"
)

func TestStagedFixtureConsumesCurrentPromptsWithRealGitFacts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("STEERRUNNER_SUCCESS_OUTPUT", "")
	t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "")
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "--bare", "remote.git")
	git(root, "init", "-b", "main", "source")
	source := filepath.Join(root, "source")
	git(source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "base")
	git(source, "remote", "add", "origin", filepath.Join(root, "remote.git"))
	base := git(source, "rev-parse", "HEAD")
	var prompts map[string]string
	if err := json.Unmarshal(resources.Read("prompts.json"), &prompts); err != nil {
		t.Fatal(err)
	}
	provider := filepath.Join(root, "provider.json")
	if _, err := StagedBusiness(source, prompts["implement.execute"], provider, "origin"); err != nil {
		t.Fatal(err)
	}
	head := git(source, "rev-parse", "HEAD")
	if head == base {
		t.Fatal("implementation did not commit actual Git changes")
	}
	if _, err := os.Stat(filepath.Join(source, "stage-result.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := StagedBusiness(source, prompts["implement.report"], provider, "origin"); err != nil {
		t.Fatal(err)
	}
	if git(source, "rev-parse", "HEAD") != head {
		t.Fatal("report changed implementation")
	}
	if _, err := StagedBusiness(source, prompts["publish"], provider, "origin"); err != nil {
		t.Fatal(err)
	}
	if git(filepath.Join(root, "remote.git"), "rev-parse", "refs/heads/main") != head {
		t.Fatal("publication did not push original HEAD")
	}
	raw, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	var publication struct {
		Head string `json:"head_revision"`
		URL  string `json:"url"`
	}
	if json.Unmarshal(raw, &publication) != nil || publication.Head != head || publication.URL == "" {
		t.Fatal("publication lacks original Git facts", string(raw))
	}
	report, err := StagedBusiness(source, prompts["publish.report"], provider, "origin")
	if err != nil || !strings.Contains(report, publication.URL) || git(source, "rev-parse", "HEAD") != head {
		t.Fatal("publication report changed or lost original result", report, err)
	}
	if _, err := StagedBusiness(source, "unknown fixture instruction", provider, "origin"); err == nil {
		t.Fatal("unknown prompt accepted")
	}
}
