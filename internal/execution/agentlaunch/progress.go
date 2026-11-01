package agentlaunch

import "strings"

// Finite preparation in the existing native launch. Initialization/collection
// belongs to the file channel; this prefix only ignores its managed directory
// and exports the path on the execution host before launching the original agent.
func ProgressCommand(command, run string) string {
	return progressCommand(command, run, false)
}
func progressCommand(command, run string, historical bool) string {
	if run == "" {
		return "& { $srProgressBefore=[Environment]::GetEnvironmentVariable('STEERLINE_PROGRESS_FILE'); try { [Environment]::SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$null); " + command + " } finally { [Environment]::SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$srProgressBefore) } }"
	}
	relative := ".steerrunner/progress/" + run + "/progress.bin"
	lines := []string{
		`$srProgressBefore=[Environment]::GetEnvironmentVariable('STEERLINE_PROGRESS_FILE')`,
		`try { $env:STEERLINE_PROGRESS_FILE=[IO.Path]::GetFullPath((Join-Path (Get-Location).ProviderPath ` + quote(relative) + `))`,
		`$srProgressExclude=@(& git rev-parse --path-format=absolute --git-path info/exclude); if($LASTEXITCODE -ne 0 -or $srProgressExclude.Count -ne 1) { throw 'Progress ignore unavailable' }; [IO.File]::AppendAllText($srProgressExclude[0],"` + "`n/.steerrunner/progress/`n" + `")`,
		command,
		`} finally { [Environment]::SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$srProgressBefore) }`,
	}
	if !historical {
		lines[2] = `try { $srProgressExclude=@(& git rev-parse --path-format=absolute --git-path info/exclude); if($LASTEXITCODE -eq 0 -and $srProgressExclude.Count -eq 1) { [IO.File]::AppendAllText($srProgressExclude[0],"` + "`n/.steerrunner/progress/`n" + `") } } catch { }`
	}
	return "& { " + strings.Join(lines, "; ") + " }"
}

// UnwrapProgressCommand recognizes only the compiler's complete fixed wrapper
// for this run, or the disabled wrapper. The caller verifies the full launch
// digest before interpreting the original command inside it.
func UnwrapProgressCommand(command, run string) (string, bool) {
	const marker = "\x00steerrunner-progress-command\x00"
	for _, candidate := range []string{run, ""} {
		for _, historical := range []bool{false, true} {
			prefix, suffix, _ := strings.Cut(progressCommand(marker, candidate, historical), marker)
			if !strings.HasPrefix(command, prefix) || !strings.HasSuffix(command, suffix) || len(command) < len(prefix)+len(suffix) {
				continue
			}
			inner := command[len(prefix) : len(command)-len(suffix)]
			return inner, progressCommand(inner, candidate, historical) == command
		}
	}
	return "", false
}

func ResultCommand(command, run string) string {
	return resultCommand(command, run, false)
}
func resultCommand(command, run string, historical bool) string {
	if run == "" {
		return "& { $srResultBefore=[Environment]::GetEnvironmentVariable('STEERLINE_RESULT_FILE'); try { [Environment]::SetEnvironmentVariable('STEERLINE_RESULT_FILE',$null); " + command + " } finally { [Environment]::SetEnvironmentVariable('STEERLINE_RESULT_FILE',$srResultBefore) } }"
	}
	relative := ".steerrunner/result/" + run + "/result.bin"
	lines := []string{
		`$srResultBefore=[Environment]::GetEnvironmentVariable('STEERLINE_RESULT_FILE')`,
		`try { $env:STEERLINE_RESULT_FILE=[IO.Path]::GetFullPath((Join-Path (Get-Location).ProviderPath ` + quote(relative) + `))`,
		`$srResultExclude=@(& git rev-parse --path-format=absolute --git-path info/exclude); if($LASTEXITCODE -ne 0 -or $srResultExclude.Count -ne 1) { throw 'Result ignore unavailable' }; [IO.File]::AppendAllText($srResultExclude[0],"` + "`n/.steerrunner/result/`n" + `")`,
		command,
		`} finally { [Environment]::SetEnvironmentVariable('STEERLINE_RESULT_FILE',$srResultBefore) }`,
	}
	if !historical {
		lines[2] = `try { $srResultExclude=@(& git rev-parse --path-format=absolute --git-path info/exclude); if($LASTEXITCODE -eq 0 -and $srResultExclude.Count -eq 1) { [IO.File]::AppendAllText($srResultExclude[0],"` + "`n/.steerrunner/result/`n" + `") } } catch { }`
	}
	return "& { " + strings.Join(lines, "; ") + " }"
}

// FileCommands adds exactly the two managed channels; historical frozen launches
// remain valid and are never recompiled during service recovery.
func FileCommands(command, progressRun, resultRun string) string {
	return ResultCommand(ProgressCommand(command, progressRun), resultRun)
}
func UnwrapFileCommands(command, run string) (string, bool) {
	const marker = "\x00steerrunner-result-command\x00"
	for _, candidate := range []string{run, ""} {
		for _, historical := range []bool{false, true} {
			prefix, suffix, _ := strings.Cut(resultCommand(marker, candidate, historical), marker)
			if strings.HasPrefix(command, prefix) && strings.HasSuffix(command, suffix) && len(command) >= len(prefix)+len(suffix) {
				inner := command[len(prefix) : len(command)-len(suffix)]
				if resultCommand(inner, candidate, historical) == command {
					return UnwrapProgressCommand(inner, run)
				}
			}
		}
	}
	return UnwrapProgressCommand(command, run)
}
