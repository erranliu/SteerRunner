package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
)

func TestCreationFixtureOnlyAcceptsOriginalBootstrapOnce(t *testing.T) {
	run := strings.Repeat("a", 32)
	plain, err := (&agentlaunch.Config{Executable: "codex", AccessMode: "full_access", RepositoryURL: "http://fixture/repo"}).Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	command := agentlaunch.FileCommands(plain, "", "")
	for _, fault := range []string{"none", "business", "suffix", "foreign-handle", "missing-request", "missing-enter", "foreign-file-scope"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("STEERRUNNER_FIXTURE_STATE", root)
			t.Setenv("STEERRUNNER_CREATION_RUNTIME", "original-runtime")
			value := struct {
				Worktree native.Worktree
				Terminal native.Terminal
			}{native.Worktree{DisplayName: "sr-" + run}, native.Terminal{Handle: "original", IncarnationID: "original-incarnation"}}
			raw, _ := json.Marshal(value)
			if err := os.WriteFile(filepath.Join(root, "worktree-created.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			text, handle, request := command, "original", "original-request"
			switch fault {
			case "business":
				text = "Implement the task"
			case "suffix":
				text += "; Write-Output 'another command'"
			case "foreign-handle":
				handle = "replacement"
			case "missing-request":
				request = ""
			case "foreign-file-scope":
				text = agentlaunch.FileCommands(plain, strings.Repeat("b", 32), "")
			}
			args := []string{"terminal", "send", "--terminal", handle, "--text", text, "--retry-request", request}
			if fault != "missing-enter" {
				args = append(args, "--enter")
			}
			out, err := os.Create(filepath.Join(root, "response.json"))
			if err != nil {
				t.Fatal(err)
			}
			prior := os.Stdout
			os.Stdout = out
			defer func() { os.Stdout = prior; _ = out.Close() }()
			code := orcaCreationFixture(args)
			_, recorded := os.Stat(filepath.Join(root, "bootstrap-send.json"))
			if fault != "none" {
				if code == 0 || !os.IsNotExist(recorded) {
					t.Fatal("unverified send acquired acceptance", code, recorded)
				}
				return
			}
			if code != 0 || recorded != nil {
				t.Fatal("exact bootstrap was not accepted", code, recorded)
			}
			if orcaCreationFixture(args) == 0 {
				t.Fatal("same request was dispatched twice")
			}
			args[7] = "replacement-request"
			if orcaCreationFixture(args) == 0 {
				t.Fatal("replacement request acquired another bootstrap")
			}
		})
	}
}
