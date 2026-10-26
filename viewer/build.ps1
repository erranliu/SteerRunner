param([string]$OutFile = '')
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $taskRepo 'scripts/delivery-contract.ps1')
if ((& go env GOOS).Trim() -ne 'windows' -or (& go env GOARCH).Trim() -ne 'amd64') { throw 'This package targets Windows/amd64.' }
$taskSourcePaths = @('viewer','scripts/delivery-contract.ps1')
$taskIdentity = Get-SRSourceIdentity $taskRepo $taskSourcePaths
if (-not $OutFile) { $OutFile = Join-Path $PSScriptRoot '../bin/steerrunner-viewer.exe' }
$taskPath = [IO.Path]::GetFullPath($OutFile)
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskPath) | Out-Null
$taskTemporary = Join-Path (Split-Path -Parent $taskPath) ('viewer-build-' + [Guid]::NewGuid().ToString('N') + '.exe')
Push-Location $PSScriptRoot
try {
    & go build -trimpath -ldflags '-H=windowsgui' -o $taskTemporary .
    if ($LASTEXITCODE -ne 0) { throw 'Viewer build failed' }
    $taskAfterBuild = Get-SRSourceIdentity $taskRepo $taskSourcePaths
    if ($taskAfterBuild.source_commit -cne $taskIdentity.source_commit -or $taskAfterBuild.source_digest -cne $taskIdentity.source_digest) { throw 'Viewer source changed during build; build again from stable inputs.' }
    Move-Item -LiteralPath $taskTemporary -Destination $taskPath -Force
} finally {
    Pop-Location
    if (Test-Path -LiteralPath $taskTemporary) { Remove-Item -LiteralPath $taskTemporary }
}
$taskRecord = @{schema='steerrunner-build/v1';component='viewer';platform='windows/amd64';source_commit=$taskIdentity.source_commit;source_dirty=$taskIdentity.source_dirty;source_digest=$taskIdentity.source_digest;sha256=(Get-SRFileDigest $taskPath)}
[IO.File]::WriteAllText(($taskPath + '.build.json'),($taskRecord | ConvertTo-Json),[Text.UTF8Encoding]::new($false))
Write-Output $taskPath
