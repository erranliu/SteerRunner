package agentlaunch

import (
	"strings"
	"testing"
)

func TestNativeGitHubAuthenticationRejectsForeignCredentialTargets(t *testing.T) {
	for _, value := range []struct {
		url string
		ok  bool
	}{
		{"https://github.com/owner/repo.git", true},
		{"https://github.com/owner/repo", true},
		{"https://github.com.evil.invalid/owner/repo.git", false},
		{"http://github.com/owner/repo.git", false},
		{"https://other.invalid/owner/repo.git", false},
		{"https://user@github.com/owner/repo.git", false},
		{"https://github.com/owner/repo.git?redirect=elsewhere", false},
		{"https://github.com/owner/repo.git#fragment", false},
		{"https://github.com/owner/repo/child", false},
		{"https://github.com/owner/repo's.git", false},
	} {
		if got := gitHubRepository(value.url); got != value.ok {
			t.Errorf("credential target %s: got %v", value.url, got)
		}
	}
}

func TestNativeGitHubAuthenticationNeverCompilesCallerCredentials(t *testing.T) {
	t.Setenv("GH_TOKEN", "caller-only-credential-never-transport")
	command, err := (&Config{Executable: "codex", RepositoryURL: "https://github.com/owner/repo.git", AccessMode: "sandbox"}).Command("codex")
	if err != nil || strings.Contains(command, "caller-only-credential-never-transport") || !strings.Contains(command, "gh auth token --hostname github.com") || !strings.Contains(command, "finally") {
		t.Fatal("credential was transported or original environment is not restored", err)
	}
}
