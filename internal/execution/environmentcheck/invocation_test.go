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
	runGit("remote", "add", "origin", "https://example.invalid/project")
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
	padding := 4096
	interpreter, suffix, script := "bash", ".sh", `printf '%s' '{"completed":true}' > "$STEERLINE_RESULT_FILE"`
	if runtime.GOOS == "windows" {
		padding = 48 << 10
		interpreter, suffix, script = "powershell.exe", ".ps1", `$saved = Get-Content -LiteralPath $env:STEERLINE_RESULT_FILE -Raw | ConvertFrom-Json
if ($saved.padding.Length -ne 49152 -or (Get-ChildItem Env:STEERRUNNER_ENV_*)) { exit 126 }
[IO.File]::WriteAllText($env:STEERLINE_RESULT_FILE, '{"completed":true}')`
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
		ResultFile: &contracts.ResultFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte(`{"padding":"` + strings.Repeat("x", padding) + `"}`))}}
	before := runGit("status", "--porcelain")
	type scenario struct {
		name     string
		mutate   bool
		terminal bool
		code     int
	}
	cases := []scenario{{name: "matching materials"}}
	if runtime.GOOS == "windows" {
		cases = append(cases, scenario{name: "large Windows terminal invocation", terminal: true})
	}
	cases = append(cases, scenario{name: "different script", mutate: true, code: 125})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutate {
				write(name, normalized+"# drift")
			}
			cmd := terminalOrLocalCommand(t, request, root, tc.terminal)
			temp := t.TempDir()
			cmd.Env = append(cmd.Environ(), "TEMP="+temp, "TMP="+temp)
			out, _ := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
				t.Fatalf("expected exit %d, got %v: %s", tc.code, cmd.ProcessState, out)
			}
			entries, err := os.ReadDir(temp)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invocation left temporary files: %v, %v", entries, err)
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

func terminalOrLocalCommand(t *testing.T, request contracts.RunRequest, root string, terminal bool) *exec.Cmd {
	t.Helper()
	if !terminal {
		invocation := localCheckoutCommand(t, request, root)
		cmd := exec.Command(invocation.Executable, invocation.Args...)
		cmd.Dir = root
		return cmd
	}
	invocation, err := Build(request, root, "fixture", true, true)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(invocation.Shell, "-NoProfile", "-NonInteractive", "-Command", invocation.Startup)
	cmd.Dir = root
	cmd.Env = os.Environ()
	for name, value := range invocation.Environment {
		if len(value) > 8192 {
			t.Fatalf("terminal environment value too large: %s (%d bytes)", name, len(value))
		}
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	return cmd
}

func localCheckoutCommand(t *testing.T, request contracts.RunRequest, root string) Command {
	t.Helper()
	if runtime.GOOS == "windows" {
		cmd, err := PrepareLocalWindows(request, root, "fixture", filepath.Join(t.TempDir(), "invocation files"))
		if err != nil {
			t.Fatal(err)
		}
		return cmd
	}
	cmd, err := Build(request, root, "fixture", false, false)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}
