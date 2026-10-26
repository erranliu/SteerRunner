param([string]$InstallDir='', [string]$SkillsDir='')
$ErrorActionPreference='Stop'
$taskSource=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $InstallDir) { $InstallDir=Join-Path $env:LOCALAPPDATA 'Programs/SteerRunner' }
if (-not $SkillsDir) { $taskCodexHome=$env:CODEX_HOME; if (-not $taskCodexHome) { $taskCodexHome=Join-Path $env:USERPROFILE '.codex' };$SkillsDir=Join-Path $taskCodexHome 'skills' }
$taskInstall=[IO.Path]::GetFullPath($InstallDir);$taskSkills=[IO.Path]::GetFullPath($SkillsDir)
# Preflight every target before writing anything. Existing bundles are not
# upgraded under a running service; deploy a new directory explicitly instead.
if (Test-Path -LiteralPath $taskInstall) { throw 'InstallDir already exists; choose a new directory. Existing configuration and services were not changed.' }
foreach ($taskName in @('steerrunner-configure','steerrunner-run')) { if (Test-Path -LiteralPath (Join-Path $taskSkills $taskName)) { throw "Skill already exists: $taskName. No files were overwritten." } }
foreach ($taskRelative in @('steerrunner.exe','steerrunner-viewer.exe','client/steerrunner-client.ps1','manifest.json')) { if (-not (Test-Path -LiteralPath (Join-Path $taskSource $taskRelative))) { throw "Incomplete package: $taskRelative" } }
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
$null=Test-SRClientPackage $taskSource
[IO.Directory]::CreateDirectory($taskInstall) | Out-Null
foreach ($taskRelative in @('steerrunner.exe','steerrunner.exe.build.json','steerrunner-viewer.exe','steerrunner-viewer.exe.build.json','client','skills','scripts','manifest.json','LICENSE','NOTICE','THIRD_PARTY_NOTICES.txt')) { $taskFile=Join-Path $taskSource $taskRelative; if (Test-Path -LiteralPath $taskFile) { Copy-Item -LiteralPath $taskFile -Destination $taskInstall -Recurse } }
foreach ($taskName in @('steerrunner-configure','steerrunner-run')) {
    $taskTarget=Join-Path $taskSkills $taskName
    [IO.Directory]::CreateDirectory($taskSkills) | Out-Null
    Copy-Item -LiteralPath (Join-Path $taskInstall ('skills/'+$taskName)) -Destination $taskTarget -Recurse
    [IO.Directory]::CreateDirectory((Join-Path $taskTarget 'assets')) | Out-Null
    $taskLocation=@{helper_path=(Join-Path $taskInstall 'client/steerrunner-client.ps1')} | ConvertTo-Json
    [IO.File]::WriteAllText((Join-Path $taskTarget 'assets/client-location.json'),$taskLocation,(New-Object Text.UTF8Encoding($false)))
}
Write-Output (@{install_dir=$taskInstall;skills_dir=$taskSkills;viewer=(Join-Path $taskInstall 'steerrunner-viewer.exe')} | ConvertTo-Json -Compress)
