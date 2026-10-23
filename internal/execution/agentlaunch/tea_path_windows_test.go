package agentlaunch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTeaPathLaunchUsesOneIdenticalBinaryAndRestoresEnvironment(t *testing.T) {
	for _, shell := range []string{"powershell.exe", "pwsh.exe"} {
		shellPath, err := exec.LookPath(shell)
		if err != nil {
			continue
		}
		for _, scenario := range []string{"missing", "single", "identical", "different", "other-tools", "agent-failure"} {
			t.Run(shell+"/"+scenario, func(t *testing.T) {
				root := t.TempDir()
				first, second := filepath.Join(root, "first install"), filepath.Join(root, "second's install")
				for _, dir := range []string{first, second} {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				binary, err := os.ReadFile(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"))
				if err != nil {
					t.Fatal(err)
				}
				if scenario != "missing" {
					if err := os.WriteFile(filepath.Join(first, "tea.exe"), binary, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if scenario != "missing" && scenario != "single" {
					other := append([]byte(nil), binary...)
					if scenario == "different" {
						other = append(other, 'x')
					}
					if err := os.WriteFile(filepath.Join(second, "tea.exe"), other, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "other-tools" {
					if err := os.WriteFile(filepath.Join(second, "other.exe"), binary, 0600); err != nil {
						t.Fatal(err)
					}
				}
				command := `Write-Output 'AGENT_STARTED'; if(@(Get-Command tea -CommandType Application -ErrorAction SilentlyContinue).Count -gt 1) { throw 'still ambiguous' }`
				if scenario != "missing" {
					command += `; if((Get-Command tea -CommandType Application).Source -ine ` + quote(filepath.Join(first, "tea.exe")) + `) { throw 'first executable changed' }; & (Get-Command tea -CommandType Application).Source /c echo FIRST_EXECUTABLE`
				}
				if scenario == "agent-failure" {
					command += `; throw 'agent failed'`
				}
				path := first + ";" + second + ";" + filepath.Join(os.Getenv("SystemRoot"), "System32")
				script := `$env:PATH=` + quote(path) + `; $srTestPath=$env:PATH; try { ` + teaPathLaunch(command) + ` } catch { if($env:PATH -cne $srTestPath) { throw 'RESTORE_FAILED' }; Write-Output $_.Exception.Message; exit 23 }; if($env:PATH -cne $srTestPath) { throw 'RESTORE_FAILED' }; Write-Output 'RESTORED'`
				output, err := exec.Command(shellPath, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
				text := string(output)
				rejected := scenario == "different" || scenario == "other-tools"
				if rejected {
					expected := "Ambiguous tea executables differ"
					if scenario == "other-tools" {
						expected = "Duplicate tea directory supplies other tools"
					}
					if err == nil || !strings.Contains(text, expected) || strings.Contains(text, "AGENT_STARTED") || strings.Contains(text, "RESTORE_FAILED") {
						t.Fatal("unsafe resolution launched or changed environment", text, err)
					}
				} else if scenario == "agent-failure" {
					if err == nil || !strings.Contains(text, "agent failed") || strings.Contains(text, "RESTORE_FAILED") {
						t.Fatal("failed agent did not restore environment", text, err)
					}
				} else if err != nil || !strings.Contains(text, "AGENT_STARTED") || !strings.Contains(text, "RESTORED") || scenario != "missing" && !strings.Contains(text, "FIRST_EXECUTABLE") {
					t.Fatal("original executable did not run in scoped environment", text, err)
				}
			})
		}
	}
}
