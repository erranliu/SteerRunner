param(
    [string]$OutFile = '',
    [string]$Version = '0.2.0-dev.3-r2'
)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
if (-not $OutFile) { $OutFile = Join-Path $taskRepo 'bin/steerrunner.exe' }
$taskOutput = [IO.Path]::GetFullPath($OutFile)
if ($Version -notmatch '^[0-9A-Za-z][0-9A-Za-z.+-]*$') { throw 'Version must be a single version identifier.' }
if ((& go env GOOS).Trim() -ne 'windows' -or (& go env GOARCH).Trim() -ne 'amd64') { throw 'This package targets Windows/amd64.' }
$taskSourcePaths = @('cmd/steerrunner','internal','go.mod','go.sum','scripts/build.ps1','scripts/delivery-contract.ps1')
$taskIdentity = Get-SRSourceIdentity $taskRepo $taskSourcePaths
$taskCommit = $taskIdentity.source_commit
$taskDirty = $taskIdentity.source_dirty
$taskSource = $taskCommit
if ($taskDirty) { $taskSource += '-dirty' }
$taskDirectory = Split-Path -Parent $taskOutput
New-Item -ItemType Directory -Force -Path $taskDirectory | Out-Null
$taskTemporary = Join-Path $taskDirectory ('build-' + [Guid]::NewGuid().ToString('N') + '.exe')
Push-Location $taskRepo
try {
    & go build -trimpath -ldflags "-X github.com/erranliu/SteerRunner/internal/resources.Version=$Version -X github.com/erranliu/SteerRunner/internal/resources.SourceCommit=$taskSource" -o $taskTemporary ./cmd/steerrunner
    if ($LASTEXITCODE -ne 0) { throw 'Runner build failed.' }
    $taskAfterBuild = Get-SRSourceIdentity $taskRepo $taskSourcePaths
    if ($taskAfterBuild.source_commit -cne $taskCommit -or $taskAfterBuild.source_digest -cne $taskIdentity.source_digest) { throw 'Runner source changed during build; build again from stable inputs.' }
    # A running target cannot be overwritten; leave that service to its owner.
    Move-Item -LiteralPath $taskTemporary -Destination $taskOutput -Force
} finally {
    Pop-Location
    if (Test-Path -LiteralPath $taskTemporary) { Remove-Item -LiteralPath $taskTemporary }
}
$taskRecord = [ordered]@{
    schema = 'steerrunner-build/v1'; component = 'runner'; source_digest = $taskIdentity.source_digest
    version = $Version; source_commit = $taskCommit; source_dirty = $taskDirty
    protocol = 'steerrunner-cli/v1'; platform = 'windows/amd64'
    sha256 = Get-SRFileDigest $taskOutput
}
[IO.File]::WriteAllText(($taskOutput + '.build.json'), ($taskRecord | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
Write-Output $taskOutput
