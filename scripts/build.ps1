param(
    [string]$OutFile = '',
    [string]$Version = '0.2.0-dev.3-r3'
)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $OutFile) { $OutFile = Join-Path $taskRepo 'bin/steerrunner.exe' }
$taskOutput = [IO.Path]::GetFullPath($OutFile)
if ($Version -notmatch '^[0-9A-Za-z][0-9A-Za-z.+-]*$') { throw 'Version must be a single version identifier.' }
if ((& go env GOOS).Trim() -ne 'windows' -or (& go env GOARCH).Trim() -ne 'amd64') { throw 'This package targets Windows/amd64.' }
$taskCommit = (& git -C $taskRepo rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $taskCommit -notmatch '^[0-9a-f]{40}$') { throw 'Cannot identify source commit.' }
$taskDirty = -not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String))
$taskSource = $taskCommit
if ($taskDirty) { $taskSource += '-dirty' }
$taskDirectory = Split-Path -Parent $taskOutput
New-Item -ItemType Directory -Force -Path $taskDirectory | Out-Null
$taskTemporary = Join-Path $taskDirectory ('build-' + [Guid]::NewGuid().ToString('N') + '.exe')
Push-Location $taskRepo
try {
    & go build -trimpath -ldflags "-X github.com/erranliu/SteerRunner/internal/resources.Version=$Version -X github.com/erranliu/SteerRunner/internal/resources.SourceCommit=$taskSource" -o $taskTemporary ./cmd/steerrunner
    if ($LASTEXITCODE -ne 0) { throw 'Runner build failed.' }
    # A running target cannot be overwritten; leave that service to its owner.
    Move-Item -LiteralPath $taskTemporary -Destination $taskOutput -Force
} finally {
    Pop-Location
    if (Test-Path -LiteralPath $taskTemporary) { Remove-Item -LiteralPath $taskTemporary }
}
$taskRecord = [ordered]@{
    version = $Version; source_commit = $taskCommit; source_dirty = $taskDirty
    protocol = 'steerrunner-cli/v1'; platform = 'windows/amd64'
    sha256 = (Get-FileHash -LiteralPath $taskOutput -Algorithm SHA256).Hash
}
[IO.File]::WriteAllText(($taskOutput + '.build.json'), ($taskRecord | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
Write-Output $taskOutput
