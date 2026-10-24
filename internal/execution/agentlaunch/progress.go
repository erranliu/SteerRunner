package agentlaunch

import "strings"

// Finite preparation in the existing native launch. Initialization/collection
// belongs to the file channel; this prefix only ignores its managed directory
// and exports the path on the execution host before launching the original agent.
func ProgressCommand(command, run string) string {
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
	return "& { " + strings.Join(lines, "; ") + " }"
}
