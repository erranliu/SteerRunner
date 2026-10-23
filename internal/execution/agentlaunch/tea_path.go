package agentlaunch

import "strings"

// UnwrapTeaPathLaunch recognizes only this compiler's exact fixed wrapper.
// Callers must still validate the inner command's grammar and the full launch
// digest. A readiness prompt appearing in arbitrary PowerShell proves nothing.
func UnwrapTeaPathLaunch(command string) (string, bool) {
	const marker = "\x00steerrunner-native-command\x00"
	prefix, suffix, _ := strings.Cut(teaPathLaunch(marker), marker)
	if !strings.HasPrefix(command, prefix) || !strings.HasSuffix(command, suffix) || len(command) < len(prefix)+len(suffix) {
		return "", false
	}
	inner := command[len(prefix) : len(command)-len(suffix)]
	return inner, teaPathLaunch(inner) == command
}

// Some Windows package managers expose the same tea binary through both Links
// and its package directory. PowerShell can return both for Get-Command tea;
// repository publication wrappers expecting a scalar then cannot start it.
// Restrict only this launch's inherited PATH, preserving the first executable.
// Never remove a directory that supplies another tool or a different tea build.
func teaPathLaunch(command string) string {
	lines := []string{
		`$ErrorActionPreference='Stop'`,
		`$srTeaOriginalPath=$env:PATH`,
		`try {`,
		`function srTeaDigest([string]$srTeaFile) { $srTeaStream=[IO.File]::OpenRead($srTeaFile); $srTeaHasher=[Security.Cryptography.SHA256]::Create(); try { return [BitConverter]::ToString($srTeaHasher.ComputeHash($srTeaStream)).Replace('-','') } finally { $srTeaHasher.Dispose(); $srTeaStream.Dispose() } }`,
		`$srTeaCommands=@(Get-Command tea -CommandType Application -ErrorAction SilentlyContinue)`,
		`if($srTeaCommands.Count -gt 1) {`,
		`$srTeaSources=@($srTeaCommands | Select-Object -ExpandProperty Source -Unique)`,
		`$srTeaFirst=$srTeaSources[0]; $srTeaHash=srTeaDigest $srTeaFirst`,
		`$srTeaRemove=[Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)`,
		`foreach($srTeaSource in @($srTeaSources | Select-Object -Skip 1)) {`,
		`if([IO.Path]::GetFileName($srTeaSource) -ine 'tea.exe' -or (srTeaDigest $srTeaSource) -cne $srTeaHash) { throw 'Ambiguous tea executables differ; publication environment is unverified' }`,
		`$srTeaDirectory=[IO.Path]::GetDirectoryName($srTeaSource)`,
		`$srTeaTools=@(Get-ChildItem -LiteralPath $srTeaDirectory -File -Force | Where-Object { $_.Extension -in '.exe','.com','.cmd','.bat','.ps1' })`,
		`if($srTeaTools.Count -ne 1 -or $srTeaTools[0].Name -ine 'tea.exe') { throw 'Duplicate tea directory supplies other tools; refusing to alter their resolution' }`,
		`[void]$srTeaRemove.Add([IO.Path]::GetFullPath($srTeaDirectory).TrimEnd([IO.Path]::DirectorySeparatorChar))`,
		`}`,
		`$srTeaSeen=[Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase); $srTeaEntries=@()`,
		`foreach($srTeaEntry in ($srTeaOriginalPath -split ';')) { $srTeaPlain=$srTeaEntry.Trim().Trim('"'); if([IO.Path]::IsPathRooted($srTeaPlain)) { $srTeaKey=[IO.Path]::GetFullPath($srTeaPlain).TrimEnd([IO.Path]::DirectorySeparatorChar); if($srTeaRemove.Contains($srTeaKey) -or -not $srTeaSeen.Add($srTeaKey)) { continue } }; $srTeaEntries+=$srTeaEntry }`,
		`$env:PATH=$srTeaEntries -join ';'`,
		`$srTeaSelected=@(Get-Command tea -CommandType Application -ErrorAction Stop); if($srTeaSelected.Count -ne 1 -or $srTeaSelected[0].Source -ine $srTeaFirst -or (srTeaDigest $srTeaFirst) -cne $srTeaHash) { throw 'Original tea executable resolution changed during launch' }`,
		`}`,
		command,
		`} finally { $env:PATH=$srTeaOriginalPath }`,
	}
	return "& { " + strings.Join(lines, "; ") + " }"
}
