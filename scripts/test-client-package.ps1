param([Parameter(Mandatory=$true)][string]$PackageDir,[Parameter(Mandatory=$true)][string]$OutDir)
$ErrorActionPreference='Stop'
$taskPackage=[IO.Path]::GetFullPath($PackageDir)
$taskOut=[IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Use a new isolated OutDir' }
[IO.Directory]::CreateDirectory($taskOut) | Out-Null
$taskInstall=Join-Path $taskOut 'installed bundle';$taskSkills=Join-Path $taskOut 'skills'
$taskInstaller=Join-Path $taskPackage 'scripts/install-client.ps1'
$taskPassed=New-Object Collections.Generic.List[string]
function Assert([bool]$value,[string]$name) { if (-not $value) { throw $name };$taskPassed.Add($name) }
$taskResult=& powershell.exe -NoProfile -File $taskInstaller -InstallDir $taskInstall -SkillsDir $taskSkills
if ($LASTEXITCODE -ne 0) { throw 'Clean isolated installation failed' }
$taskLocations=@(Get-ChildItem -LiteralPath $taskSkills -Directory | ForEach-Object { (Get-Content -LiteralPath (Join-Path $_.FullName 'assets/client-location.json') -Raw | ConvertFrom-Json).helper_path })
Assert ($taskLocations.Count -eq 2 -and $taskLocations[0] -ceq $taskLocations[1] -and $taskLocations[0] -ceq (Join-Path $taskInstall 'client/steerrunner-client.ps1')) 'exactly two skills locate one installed helper'
$taskClient=Join-Path $taskOut 'client records';$taskInput=Join-Path $taskOut 'empty.json'
$taskReport=& powershell.exe -NoProfile -File $taskLocations[0] -ClientRoot $taskClient -ProjectPath $env:SystemRoot
if ($LASTEXITCODE -ne 0) { throw 'Installed default configure failed' }
$taskDiscovery=$taskReport | ConvertFrom-Json
Assert ($taskDiscovery.ok -and $taskDiscovery.data.configuration.runner_executable -ceq (Join-Path $taskInstall 'steerrunner.exe') -and $null -eq $taskDiscovery.data.discovery.project) 'default configure runs outside checkout and locates bundled Runner'
$taskConfigHash=(Get-FileHash -LiteralPath (Join-Path $taskClient 'client.json')).Hash
$taskSkillHash=(Get-FileHash -LiteralPath (Join-Path $taskSkills 'steerrunner-configure/SKILL.md')).Hash
& powershell.exe -NoProfile -File $taskInstaller -InstallDir $taskInstall -SkillsDir $taskSkills 2> (Join-Path $taskOut 'existing-install-error.txt') | Out-Null
Assert ($LASTEXITCODE -ne 0 -and (Get-FileHash -LiteralPath (Join-Path $taskClient 'client.json')).Hash -ceq $taskConfigHash) 'existing installation rejected and caller configuration preserved'
$taskConflict=Join-Path $taskOut 'conflicting install'
& powershell.exe -NoProfile -File $taskInstaller -InstallDir $taskConflict -SkillsDir $taskSkills 2> (Join-Path $taskOut 'existing-skill-error.txt') | Out-Null
Assert ($LASTEXITCODE -ne 0 -and -not (Test-Path -LiteralPath $taskConflict) -and (Get-FileHash -LiteralPath (Join-Path $taskSkills 'steerrunner-configure/SKILL.md')).Hash -ceq $taskSkillHash) 'colliding skill rejected before any install mutation'
$taskTampered=Join-Path $taskOut 'tampered package';Copy-Item -LiteralPath $taskPackage -Destination $taskTampered -Recurse
[IO.File]::AppendAllText((Join-Path $taskTampered 'client/steerrunner-client.ps1'),"`n# tampered fixture")
$taskRejected=Join-Path $taskOut 'rejected install'
& powershell.exe -NoProfile -File (Join-Path $taskTampered 'scripts/install-client.ps1') -InstallDir $taskRejected -SkillsDir (Join-Path $taskOut 'unused skills') 2> (Join-Path $taskOut 'tampered-error.txt') | Out-Null
Assert ($LASTEXITCODE -ne 0 -and -not (Test-Path -LiteralPath $taskRejected)) 'manifest digest mismatch rejected before installation'
$taskMoved=Join-Path $taskOut 'relocated bundle'
foreach($taskPath in @($taskInstall,$taskMoved)) { if (-not [IO.Path]::GetFullPath($taskPath).StartsWith($taskOut+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) { throw 'Relocation target escaped the isolated OutDir' } }
Move-Item -LiteralPath $taskInstall -Destination $taskMoved
Assert (-not (Test-Path -LiteralPath $taskLocations[0]) -and (Get-FileHash -LiteralPath (Join-Path $taskClient 'client.json')).Hash -ceq $taskConfigHash) 'relocation leaves an explicit broken locator and preserves caller records'
$taskResult=@{passed=@($taskPassed);installed_configure=$taskDiscovery.ok;isolated=$true;manifest_sha256=(Get-FileHash -LiteralPath (Join-Path $taskPackage 'manifest.json')).Hash}
[IO.File]::WriteAllText((Join-Path $taskOut 'result.json'),($taskResult | ConvertTo-Json -Depth 6),(New-Object Text.UTF8Encoding($false)))
Write-Output ($taskResult | ConvertTo-Json -Depth 6)
