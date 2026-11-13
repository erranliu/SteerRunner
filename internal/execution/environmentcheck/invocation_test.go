package environmentcheck

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestExistingCheckoutInvocation(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = root
		body, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, body)
		}
		return strings.TrimSpace(string(body))
	}
	runGit("init", "--quiet")
	runGit("remote", "add", "origin", "https://example.invalid/project.git")
	runGit("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgSign=false", "commit", "--allow-empty", "-m", "different HEAD")
	manifest := `{"version":6}`
	fence := strings.Repeat(string(rune(96)), 3)
	write := func(name, text string) {
		t.Helper()
		name = filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(contracts.EnvironmentDocument, "Project notes\n"+fence+"steerline-environment\n"+manifest+"\n"+fence+"\n")
	interpreter, suffix, script := "bash", ".sh", `printf '%s' '{"completed":true}' > "$STEERLINE_RESULT_FILE"`
	if runtime.GOOS == "windows" {
		interpreter, suffix, script = "powershell.exe", ".ps1", `[IO.File]::WriteAllText($env:STEERLINE_RESULT_FILE, '{"completed":true}')`
	}
	name := "SteerlineFlow/scripts/verify-environment" + suffix
	normalized := script + "\n"
	write(name, strings.ReplaceAll(normalized, "\n", "\r\n"))
	if runtime.GOOS != "windows" {
		write(name, normalized)
	}
	request := contracts.RunRequest{Repository: contracts.Repository{URL: "https://example.invalid/project.git", Remote: "origin", BaseRevision: strings.Repeat("a", 40)},
		EnvironmentCheck: &contracts.EnvironmentCheckInput{Interpreter: interpreter, Script: name, ManifestSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(manifest))),
			Files: map[string]string{name: fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))}},
		ResultFile: &contracts.ResultFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte("{}"))}}
	before := runGit("status", "--porcelain")
	for _, tc := range []struct {
		name   string
		mutate bool
		code   int
	}{{"matching materials", false, 0}, {"different script", true, 125}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutate {
				write(name, normalized+"# drift")
			}
			invocation, err := Build(request, root, "fixture", runtime.GOOS == "windows", false)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(invocation.Executable, invocation.Args...)
			cmd.Dir = root
			out, _ := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
				t.Fatalf("expected exit %d, got %v: %s", tc.code, cmd.ProcessState, out)
			}
			result, err := Decode(strings.Split(string(out), "\n"), "fixture", &tc.code)
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			if tc.mutate {
				if len(result.Data) != 0 || !strings.Contains(result.Diagnostic, "material differs") {
					t.Fatal(result)
				}
			} else if string(result.Data) != `{"completed":true}` || result.Diagnostic != "" {
				t.Fatal(result)
			}
		})
	}
	if got := runGit("status", "--porcelain"); got != before {
		t.Fatalf("checkout status changed: %q -> %q", before, got)
	}
	if _, err := os.Stat(filepath.Join(root, ".steerrunner")); !os.IsNotExist(err) {
		t.Fatalf("run files leaked into repository: %v", err)
	}
}
