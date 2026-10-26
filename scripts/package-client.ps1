param([Parameter(Mandatory=$true)][string]$OutDir,[string]$RunnerExe='',[string]$ViewerExe='')
$ErrorActionPreference='Stop'
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
$taskOut=[IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'OutDir must be new; no existing bundle will be replaced.' }
if (-not $RunnerExe) { $RunnerExe=Join-Path $taskRepo 'bin/steerrunner.exe' }
if (-not $ViewerExe) { $ViewerExe=Join-Path $taskRepo 'bin/steerrunner-viewer.exe' }
foreach ($taskExe in @($RunnerExe,$ViewerExe)) { if (-not [IO.File]::Exists([IO.Path]::GetFullPath($taskExe))) { throw 'Build Runner and Viewer before packaging.' } }
$taskRunnerBuild=Get-SRBuildRecord ([IO.Path]::GetFullPath($RunnerExe)) 'runner'
$taskViewerBuild=Get-SRBuildRecord ([IO.Path]::GetFullPath($ViewerExe)) 'viewer'
$taskSourcePaths=@('client','skills','LICENSE','NOTICE','THIRD_PARTY_NOTICES.txt','scripts/install-client.ps1','scripts/delivery-contract.ps1','scripts/verify-client-package.ps1','scripts/package-client.ps1')
$taskSource=Get-SRSourceIdentity $taskRepo $taskSourcePaths
[IO.Directory]::CreateDirectory($taskOut) | Out-Null
Copy-Item -LiteralPath ([IO.Path]::GetFullPath($RunnerExe)) -Destination (Join-Path $taskOut 'steerrunner.exe')
Copy-Item -LiteralPath ([IO.Path]::GetFullPath($ViewerExe)) -Destination (Join-Path $taskOut 'steerrunner-viewer.exe')
Copy-Item -LiteralPath ([IO.Path]::GetFullPath($RunnerExe)+'.build.json') -Destination (Join-Path $taskOut 'steerrunner.exe.build.json')
Copy-Item -LiteralPath ([IO.Path]::GetFullPath($ViewerExe)+'.build.json') -Destination (Join-Path $taskOut 'steerrunner-viewer.exe.build.json')
foreach ($taskName in @('client','skills','LICENSE','NOTICE','THIRD_PARTY_NOTICES.txt')) { Copy-Item -LiteralPath (Join-Path $taskRepo $taskName) -Destination $taskOut -Recurse }
[IO.Directory]::CreateDirectory((Join-Path $taskOut 'scripts')) | Out-Null
foreach ($taskScript in @('install-client.ps1','delivery-contract.ps1','verify-client-package.ps1')) { Copy-Item -LiteralPath (Join-Path $PSScriptRoot $taskScript) -Destination (Join-Path $taskOut ('scripts/'+$taskScript)) }
$taskAfterCopy=Get-SRSourceIdentity $taskRepo $taskSourcePaths
if ($taskAfterCopy.source_commit -cne $taskSource.source_commit -or $taskAfterCopy.source_digest -cne $taskSource.source_digest) { throw 'Package source changed during copying; create a new package.' }
$taskEntries=@(Get-SRPackageInventory $taskOut)
$taskVersionText=& (Join-Path $taskOut 'steerrunner.exe') version --json
if ($LASTEXITCODE -ne 0) { throw 'Cannot read the bundled Runner version.' }
$taskVersion=$taskVersionText | ConvertFrom-Json
if (-not $taskVersion.ok) { throw 'Invalid bundled Runner version response.' }
if ($taskVersion.data.source_commit -cne ($taskRunnerBuild.source_commit + $(if($taskRunnerBuild.source_dirty){'-dirty'}else{''}))) { throw 'Runner version and build provenance disagree.' }
$taskRunnerVersion=@{version=$taskVersion.data.version;source_commit=$taskVersion.data.source_commit;platform=$taskVersion.data.platform;resource_version=$taskVersion.data.resource_version;resource_digest=$taskVersion.data.resource_digest;protocols=$taskVersion.data.protocols}
$taskRunnerVersion['build']=$taskRunnerBuild
$taskSource['schema']='steerrunner-package-source/v1'
$taskManifest=@{version=2;snapshot_schema_version=1;provenance=$taskSource;components=@{runner=$taskRunnerVersion;viewer=@{build=$taskViewerBuild};client_contract_version=1;viewer_snapshot_schema_version=1};files=$taskEntries}
[IO.File]::WriteAllText((Join-Path $taskOut 'manifest.json'),($taskManifest | ConvertTo-Json -Depth 10),(New-Object Text.UTF8Encoding($false)))
$null=Test-SRClientPackage $taskOut
Write-Output $taskOut
