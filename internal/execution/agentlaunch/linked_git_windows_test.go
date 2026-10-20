//go:build windows

package agentlaunch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLinkedGitLaunchUsesVerifiedExecutionHostRoots(t *testing.T) {
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo's space"), filepath.Join(root, "work's space")
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	git(repo, "init", "--initial-branch=main")
	git(repo, "config", "user.name", "test")
	git(repo, "config", "user.email", "test@example.invalid")
	git(repo, "commit", "--allow-empty", "-m", "fixture")
	git(repo, "remote", "add", "chosen", "https://github.com/owner/repo.git")
	git(repo, "worktree", "add", "-b", "worker", work)
	admin := git(work, "rev-parse", "--absolute-git-dir")
	common := filepath.Join(repo, ".git")
	executable, capture := filepath.Join(root, "capture's args.ps1"), filepath.Join(root, "arguments.json")
	stub := filepath.Join(root, "native-login")
	if err := os.Mkdir(stub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stub, "gh.cmd"), []byte("@echo off\r\necho unit-only-native-gh-credential\r\nexit /b 0\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte(`[IO.File]::WriteAllText($env:SR_LINKED_CAPTURE,(ConvertTo-Json -InputObject @($args) -Compress)); [IO.File]::WriteAllText(($env:SR_LINKED_CAPTURE+'.auth'),(ConvertTo-Json -Compress @{gh_host=$env:GH_HOST;token_matches=($env:GH_TOKEN -ceq 'unit-only-native-gh-credential');git_key=$env:GIT_CONFIG_KEY_0;git_count=$env:GIT_CONFIG_COUNT;header_matches=($env:GIT_CONFIG_VALUE_0 -ceq ('Authorization: Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('x-access-token:unit-only-native-gh-credential'))))}))`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Executable: executable, RepositoryURL: "https://github.com/owner/repo.git", RepositoryRemote: "chosen", AccessMode: "sandbox", Model: "writer", ReasoningEffort: "low"}
	launch := func(c *Config, dir string, extra ...string) ([]string, error) {
		t.Helper()
		_ = os.Remove(capture)
		_ = os.Remove(capture + ".restored")
		command, err := c.Command("codex")
		if err != nil {
			t.Fatal(err)
		}
		command = `$ErrorActionPreference='Stop'; ` + command + `; [IO.File]::WriteAllText(($env:SR_LINKED_CAPTURE+'.restored'),(ConvertTo-Json -Compress @{host_restored=($env:GH_HOST -ceq 'enterprise.invalid');token_restored=($env:GH_TOKEN -ceq 'caller-only-preserved-token');git_count_restored=([string]$env:GIT_CONFIG_COUNT -ceq '')}))`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), append([]string{"SR_LINKED_CAPTURE=" + capture, "PATH=" + stub + ";" + os.Getenv("PATH"), "GH_HOST=enterprise.invalid", "GH_TOKEN=caller-only-preserved-token", "PRIVATE_PEER_SECRET=unrelated-secret", "GIT_CONFIG_COUNT=", "GIT_CONFIG_PARAMETERS="}, extra...)...)
		out, callErr := cmd.CombinedOutput()
		raw, err := os.ReadFile(capture)
		if callErr != nil {
			if !os.IsNotExist(err) {
				t.Fatal("rejected preparation launched agent", string(raw))
			}
			return nil, callErr
		}
		var args []string
		if err != nil || json.Unmarshal(raw, &args) != nil {
			t.Fatalf("missing launch arguments: %v: %s", err, out)
		}
		return args, nil
	}
	args, err := launch(cfg, work)
	if err != nil {
		t.Fatal("valid linked launch rejected", err)
	}
	var roots []string
	for i, arg := range args {
		if arg == "--add-dir" && i+1 < len(args) {
			roots = append(roots, filepath.Clean(args[i+1]))
		}
	}
	want := []string{filepath.Join(common, "objects"), filepath.Join(common, "refs"), filepath.Join(common, "logs"), filepath.Clean(admin)}
	if !reflect.DeepEqual(roots, want) || !strings.HasPrefix(args[len(args)-1], "Initialize this agent session. Reply READY only. Do not use tools or change files.") || !strings.Contains(args[len(args)-1], "Do not print environment") {
		t.Fatal("linked roots or initialization input changed", args)
	}
	var auth struct {
		Host          string `json:"gh_host"`
		TokenMatches  bool   `json:"token_matches"`
		HeaderMatches bool   `json:"header_matches"`
		Key           string `json:"git_key"`
		Count         string `json:"git_count"`
	}
	raw, err := os.ReadFile(capture + ".auth")
	if err != nil || json.Unmarshal(raw, &auth) != nil || auth.Host != "github.com" || !auth.TokenMatches || !auth.HeaderMatches || auth.Count != "1" || auth.Key != "http.https://github.com/owner/repo.git.extraHeader" {
		t.Fatal("native process did not receive exact execution-host authentication", auth, err)
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "unit-only-native-gh-credential") || strings.Contains(joined, "PRIVATE_PEER_SECRET") || strings.Contains(joined, "ORCA_PAIRING_CODE") || !strings.Contains(joined, "shell_environment_policy.include_only=") || !strings.Contains(joined, "'GH_TOKEN'") || !strings.Contains(joined, "'ORCA_AGENT_HOOK_TOKEN'") || strings.Count(joined, "--no-daemon") != 1 {
		t.Fatal("credential entered argv or native environment was not fixed", args)
	}
	var restored struct {
		Host  bool `json:"host_restored"`
		Token bool `json:"token_restored"`
		Git   bool `json:"git_count_restored"`
	}
	raw, err = os.ReadFile(capture + ".restored")
	if err != nil || json.Unmarshal(raw, &restored) != nil || !restored.Host || !restored.Token || !restored.Git {
		t.Fatal("launch altered caller environment after native exit", restored, err)
	}
	t.Run("tracing-rejected", func(t *testing.T) {
		if _, err := launch(cfg, work, "GH_DEBUG=api"); err == nil {
			t.Fatal("credential transport accepted trace output")
		}
	})
	t.Run("foreign-repository", func(t *testing.T) {
		copy := cfg.Clone()
		copy.RepositoryURL += "-foreign"
		if _, err := launch(copy, work); err == nil {
			t.Fatal("foreign repository launched agent")
		}
	})
	t.Run("main-checkout", func(t *testing.T) {
		if _, err := launch(cfg, repo); err == nil {
			t.Fatal("main Git directory became writable")
		}
	})
	t.Run("environment-override", func(t *testing.T) {
		if _, err := launch(cfg, work, "GIT_INDEX_FILE="+filepath.Join(root, "foreign-index")); err == nil {
			t.Fatal("foreign index override launched agent")
		}
	})
	t.Run("redirected-root", func(t *testing.T) {
		objects, saved := filepath.Join(common, "objects"), filepath.Join(common, "saved-objects")
		if err := os.Rename(objects, saved); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Rename(saved, objects) }()
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "New-Item -ItemType Junction -Path "+quote(objects)+" -Target "+quote(saved)+" | Out-Null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("junction fixture: %v: %s", err, out)
		}
		defer func() { _ = os.Remove(objects) }()
		if _, err := launch(cfg, work); err == nil {
			t.Fatal("redirected Git root launched agent")
		}
	})
	for _, mode := range []string{"full_access", "content"} {
		t.Run(mode, func(t *testing.T) {
			copy := cfg.Clone()
			if mode == "full_access" {
				copy.AccessMode = mode
			} else {
				copy.RepositoryURL = ""
			}
			command, err := copy.Command("codex")
			if err != nil || strings.Contains(command, "srGitWriteArgs") || strings.Contains(command, "--add-dir") {
				t.Fatal("unrelated launch acquired linked roots", command, err)
			}
		})
	}
}
