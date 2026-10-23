param([Parameter(Mandatory=$true)][string]$OutDir,[string]$RunnerExe='',[string]$ViewerExe='')
$ErrorActionPreference='Stop'
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskOut=[IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'OutDir must be new; no existing bundle will be replaced.' }
if (-not $RunnerExe) { $RunnerExe=Join-Path $taskRepo 'bin/steerrunner.exe' }
if (-not $ViewerExe) { $ViewerExe=Join-Path $taskRepo 'bin/steerrunner-viewer.exe' }
foreach ($taskExe in @($RunnerExe,$ViewerExe)) { if (-not [IO.File]::Exists([IO.Path]::GetFullPath($taskExe))) { throw 'Build Runner and Viewer before packaging.' } }
[IO.Directory]::CreateDirectory($taskOut) | Out-Null
Copy-Item -LiteralPath ([IO.Path]::GetFullPath($RunnerExe)) -Destination (Join-Path $taskOut 'steerrunner.exe')
Copy-Item -LiteralPath ([IO.Path]::GetFullPath($ViewerExe)) -Destination (Join-Path $taskOut 'steerrunner-viewer.exe')
foreach ($taskName in @('client','skills','LICENSE','NOTICE','THIRD_PARTY_NOTICES.txt')) { Copy-Item -LiteralPath (Join-Path $taskRepo $taskName) -Destination $taskOut -Recurse }
[IO.Directory]::CreateDirectory((Join-Path $taskOut 'scripts')) | Out-Null
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'install-client.ps1') -Destination (Join-Path $taskOut 'scripts/install-client.ps1')
$taskEntries=@(Get-ChildItem -LiteralPath $taskOut -Recurse -File | ForEach-Object { @{path=$_.FullName.Substring($taskOut.Length+1).Replace('\','/');sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()} })
$taskVersionText=& ([IO.Path]::GetFullPath($RunnerExe)) version --json
if ($LASTEXITCODE -ne 0) { throw 'Cannot read the bundled Runner version.' }
$taskVersion=$taskVersionText | ConvertFrom-Json
if (-not $taskVersion.ok) { throw 'Invalid bundled Runner version response.' }
$taskRunnerVersion=@{version=$taskVersion.data.version;source_commit=$taskVersion.data.source_commit;platform=$taskVersion.data.platform;resource_version=$taskVersion.data.resource_version;resource_digest=$taskVersion.data.resource_digest;protocols=$taskVersion.data.protocols}
$taskManifest=@{version=1;snapshot_schema_version=1;components=@{runner=$taskRunnerVersion;client_contract_version=1;viewer_snapshot_schema_version=1};files=$taskEntries}
[IO.File]::WriteAllText((Join-Path $taskOut 'manifest.json'),($taskManifest | ConvertTo-Json -Depth 8),(New-Object Text.UTF8Encoding($false)))
Write-Output $taskOut
