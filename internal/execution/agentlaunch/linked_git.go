package agentlaunch

import "strings"

// Resolve on the execution host, immediately before the native agent starts.
// This is a finite shell launch prefix: no helper deployment, credential changes,
// business Git writes, or extra terminal. Only the current linked worktree admin
// and the repository's three fixed data directories become writable.
func linkedGitLaunch(repository, remote, command string) string {
	if remote == "" {
		remote = "origin"
	}
	lines := []string{
		`$ErrorActionPreference='Stop'`,
		`foreach($srName in @('GIT_DIR','GIT_WORK_TREE','GIT_COMMON_DIR','GIT_OBJECT_DIRECTORY','GIT_ALTERNATE_OBJECT_DIRECTORIES','GIT_INDEX_FILE','GIT_NAMESPACE')) { if([Environment]::GetEnvironmentVariable($srName)) { throw 'Linked Git environment override rejected' } }`,
		`function srGitPath([string[]]$srArgv) { $srOut=@(& git @srArgv); if($LASTEXITCODE -ne 0 -or $srOut.Count -ne 1 -or -not [IO.Path]::IsPathRooted($srOut[0])) { throw 'Linked Git path unavailable' }; return [IO.Path]::GetFullPath($srOut[0]).TrimEnd('\') }`,
		`function srGitPlain([string]$srPath) { $srItem=Get-Item -LiteralPath $srPath -Force; while($null -ne $srItem) { if(($srItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Linked Git path redirected' }; if($srItem -is [IO.FileInfo]) { $srItem=$srItem.Directory } else { $srItem=$srItem.Parent } } }`,
		`$srWorkspace=[IO.Path]::GetFullPath((Get-Location).ProviderPath).TrimEnd('\')`,
		`$srTop=srGitPath @('rev-parse','--show-toplevel')`,
		`$srCommon=srGitPath @('rev-parse','--path-format=absolute','--git-common-dir')`,
		`$srAdmin=srGitPath @('rev-parse','--absolute-git-dir')`,
		`if($srTop -ine $srWorkspace -or [IO.Path]::GetDirectoryName($srAdmin) -ine (Join-Path $srCommon 'worktrees')) { throw 'Linked Git workspace/admin identity differs' }`,
		`$srGitFile=Join-Path $srWorkspace '.git'; srGitPlain $srGitFile; srGitPlain $srCommon; srGitPlain $srAdmin`,
		`$srText=[IO.File]::ReadAllText($srGitFile).Trim(); if(-not $srText.StartsWith('gitdir: ')) { throw 'Linked Git file unavailable' }; $srDeclared=$srText.Substring(8); if(-not [IO.Path]::IsPathRooted($srDeclared)) { $srDeclared=Join-Path $srWorkspace $srDeclared }; if([IO.Path]::GetFullPath($srDeclared).TrimEnd('\') -ine $srAdmin) { throw 'Linked Git file identity differs' }`,
		`$srBacklink=Join-Path $srAdmin 'gitdir'; $srCommonFile=Join-Path $srAdmin 'commondir'; srGitPlain $srBacklink; srGitPlain $srCommonFile`,
		`if([IO.Path]::GetFullPath([IO.File]::ReadAllText($srBacklink).Trim()) -ine $srGitFile -or [IO.Path]::GetFullPath((Join-Path $srAdmin ([IO.File]::ReadAllText($srCommonFile).Trim()))).TrimEnd('\') -ine $srCommon) { throw 'Linked Git backlinks differ' }`,
		`$srRemote=@(& git remote get-url ` + quote(remote) + `); if($LASTEXITCODE -ne 0 -or $srRemote.Count -ne 1 -or $srRemote[0] -cne ` + quote(repository) + `) { throw 'Linked Git repository differs' }`,
		`$srGitWriteArgs=@(); foreach($srLeaf in @('objects','refs','logs')) { $srPath=srGitPath @('rev-parse','--path-format=absolute','--git-path',$srLeaf); if($srPath -ine (Join-Path $srCommon $srLeaf) -or -not [IO.Directory]::Exists($srPath)) { throw 'Linked Git data root differs' }; srGitPlain $srPath; $srGitWriteArgs+=@('--add-dir',$srPath) }; $srGitWriteArgs+=@('--add-dir',$srAdmin)`,
		`if((srGitPath @('rev-parse','--path-format=absolute','--git-common-dir')) -ine $srCommon -or (srGitPath @('rev-parse','--absolute-git-dir')) -ine $srAdmin) { throw 'Linked Git identity changed during launch' }`,
		command,
	}
	return "& { " + strings.Join(lines, "; ") + " }"
}
