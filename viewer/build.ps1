param([string]$OutFile = '')
$ErrorActionPreference = 'Stop'
if (-not $OutFile) { $OutFile = Join-Path $PSScriptRoot '../bin/steerrunner-viewer.exe' }
$taskPath = [IO.Path]::GetFullPath($OutFile)
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskPath) | Out-Null
Push-Location $PSScriptRoot
try { & go build -trimpath -ldflags '-H=windowsgui' -o $taskPath .; if ($LASTEXITCODE -ne 0) { throw 'Viewer build failed' } } finally { Pop-Location }
Write-Output $taskPath
