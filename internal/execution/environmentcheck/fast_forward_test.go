package environmentcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestCheckoutFastForward(t *testing.T) {
	root := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(root, "empty-config"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	upstream, checkout := filepath.Join(root, "upstream"), filepath.Join(root, "checkout")
	git(root, "init", "-q", "-b", "main", upstream)
	commit := func(dir, message string) string {
		t.Helper()
		git(dir, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgSign=false", "commit", "--allow-empty", "-qm", message)
		return git(dir, "rev-parse", "HEAD")
	}
	initial := commit(upstream, "original")
	git(root, "clone", "-q", "--", upstream, checkout)
	git(checkout, "remote", "set-url", "origin", "https://example.invalid/project.git")
	git(checkout, "config", "url."+filepath.ToSlash(upstream)+".insteadOf", "https://example.invalid/project.git")
	latest := commit(upstream, "new")
	r := contracts.RunRequest{FastForward: true, Repository: contracts.Repository{URL: "https://example.invalid/project.git", Remote: "origin", BaseBranch: "main", BaseRevision: latest},
		ResultFile: &contracts.ResultFileInput{InitialContentBase64: "e30="}}
	invoke := func(code int, diagnostic string) {
		t.Helper()
		built, err := Build(r, checkout, "fixture", runtime.GOOS == "windows", false)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(built.Executable, built.Args...)
		cmd.Dir = checkout
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(root, "empty-config"))
		output, _ := cmd.CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
			t.Fatalf("expected exit %d: %v %s", code, cmd.ProcessState, output)
		}
		result, err := Decode(strings.Split(string(output), "\n"), "fixture", &code)
		if err != nil || !strings.Contains(result.Diagnostic, diagnostic) {
			t.Fatalf("unexpected result %+v, %v: %s", result, err, output)
		}
	}
	t.Run("dirty checkout preserved", func(t *testing.T) {
		name := filepath.Join(checkout, "untracked")
		if err := os.WriteFile(name, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		invoke(125, "local changes")
		if head := git(checkout, "rev-parse", "HEAD"); head != initial {
			t.Fatal(head)
		}
		if raw, err := os.ReadFile(name); err != nil || string(raw) != "keep" {
			t.Fatal(raw, err)
		}
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("wrong branch preserved", func(t *testing.T) {
		git(checkout, "switch", "-qc", "other")
		invoke(125, "not on the requested branch")
		if head := git(checkout, "rev-parse", "HEAD"); head != initial {
			t.Fatal(head)
		}
		git(checkout, "switch", "-q", "main")
	})
	t.Run("new upstream tip refuses stale baseline", func(t *testing.T) {
		r.Repository.BaseRevision = initial
		invoke(125, "remote branch changed")
		if head := git(checkout, "rev-parse", "HEAD"); head != initial {
			t.Fatal(head)
		}
		r.Repository.BaseRevision = latest
	})
	t.Run("advances to the exact baseline", func(t *testing.T) {
		invoke(0, "")
		if head := git(checkout, "rev-parse", "HEAD"); head != latest {
			t.Fatal(head)
		}
	})
	t.Run("divergent checkout preserved", func(t *testing.T) {
		r.Repository.BaseRevision = commit(upstream, "advance upstream")
		local := commit(checkout, "local commit")
		invoke(125, "cannot fast-forward")
		if head := git(checkout, "rev-parse", "HEAD"); head != local {
			t.Fatal(head)
		}
	})
}
