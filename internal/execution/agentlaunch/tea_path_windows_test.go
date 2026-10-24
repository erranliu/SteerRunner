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

func TestNativeShellPathGuardIsScopedAndComposesWithTea(t *testing.T) {
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		t.Skip("PowerShell 7 unavailable")
	}
	for _, scenario := range []string{"alias", "concrete", "missing", "unresolved", "agent-failure"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"first", "second"} {
				path := filepath.Join(root, dir)
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
				binary, err := os.ReadFile(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "tea.exe"), binary, 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(root, "first") + ";" + filepath.Join(root, "second") + ";" + filepath.Join(os.Getenv("SystemRoot"), "System32")
			// Mock only Get-Command's pwsh lookup; actual tea lookup, executable
			// hashes, filesystem checks and scoped PATH changes run in PowerShell.
			mock := `function Get-Command { param($Name,$CommandType,$ErrorAction); if($Name -ne 'pwsh') { return Microsoft.PowerShell.Core\Get-Command @PSBoundParameters }; if($srTestScenario -eq 'missing') { return }; if($srTestScenario -eq 'concrete') { return [pscustomobject]@{Source=(Join-Path $PSHOME 'pwsh.exe')} }; if($srTestScenario -ne 'unresolved' -and $env:PATH.StartsWith($PSHOME+';')) { return [pscustomobject]@{Source=(Join-Path $PSHOME 'pwsh.exe')} }; return [pscustomobject]@{Source=(Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'Microsoft\WindowsApps\pwsh.exe')} }`
			body := `if(@(Get-Command tea -CommandType Application).Count -ne 1) { throw 'tea ambiguous' }; if($srTestScenario -in 'alias','agent-failure') { if(-not $env:PATH.StartsWith($PSHOME+';')) { throw 'concrete shell missing' } } elseif($env:PATH.StartsWith($PSHOME+';')) { throw 'custom shell changed' }; Write-Output 'AGENT_STARTED'`
			if scenario == "agent-failure" {
				body += `; throw 'agent failed'`
			}
			script := `$srTestScenario=` + quote(scenario) + `; $env:PATH=` + quote(path) + `; $srTestPath=$env:PATH; ` + mock + `; try { ` + teaPathLaunch(body) + ` } catch { if($env:PATH -cne $srTestPath) { throw 'RESTORE_FAILED' }; Write-Output $_.Exception.Message; exit 23 }; if($env:PATH -cne $srTestPath) { throw 'RESTORE_FAILED' }; Write-Output 'RESTORED'`
			raw, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
			out := string(raw)
			if strings.Contains(out, "RESTORE_FAILED") {
				t.Fatal(out)
			}
			switch scenario {
			case "unresolved":
				if err == nil || !strings.Contains(out, "Concrete PowerShell executable resolution could not be verified") || strings.Contains(out, "AGENT_STARTED") {
					t.Fatal(out, err)
				}
			case "agent-failure":
				if err == nil || !strings.Contains(out, "agent failed") || !strings.Contains(out, "AGENT_STARTED") {
					t.Fatal(out, err)
				}
			default:
				if err != nil || !strings.Contains(out, "RESTORED") || !strings.Contains(out, "AGENT_STARTED") {
					t.Fatal(out, err)
				}
			}
		})
	}
}
