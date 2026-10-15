param(
    [Parameter(Mandatory=$true)][string]$Manifest,
    [string]$Pattern = '^TestSRNativeLocalCodex$'
)
$ErrorActionPreference = 'Stop'
if (-not [IO.Path]::IsPathRooted($Manifest)) { throw 'Native manifest must be absolute' }
$taskManifestPath = [IO.Path]::GetFullPath($Manifest)
$taskNativeManifest = Get-Content -LiteralPath $taskManifestPath -Raw | ConvertFrom-Json
$taskNativeRoot = [IO.Path]::GetFullPath($taskNativeManifest.debug_root).TrimEnd('\')
if (-not $taskNativeRoot.StartsWith('C:\SteerlineDebug\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Use a newly generated SteerlineDebug package' }
$taskEvidence = Join-Path $taskNativeRoot ('evidence\native-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss'))
if (Test-Path -LiteralPath $taskEvidence) { throw 'Native evidence directory must be new' }
New-Item -ItemType Directory -Path $taskEvidence | Out-Null
$taskTestExe = Join-Path $taskEvidence 'native.test.exe'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Push-Location $taskRepo
try {
    & go test -tags=live -c -o $taskTestExe ./tests/native
    if ($LASTEXITCODE -ne 0) { throw 'Native driver build failed' }
} finally { Pop-Location }
function Quote-WindowsArgument([string]$Argument) { '"' + ($Argument -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"' }
$taskArguments = @($taskTestExe,"-test.run=$Pattern",'-test.v','-test.timeout=15m','-native-manifest',$taskManifestPath,'-evidence-dir',$taskEvidence)
$taskCommandLine = ($taskArguments | ForEach-Object { Quote-WindowsArgument $_ }) -join ' '
$taskStartup = New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ShowWindow=[uint16]0}
$taskCreated = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=$taskCommandLine;CurrentDirectory=$taskRepo;ProcessStartupInformation=$taskStartup}
if ($taskCreated.ReturnValue -ne 0) { throw "Native harness startup failed: $($taskCreated.ReturnValue)" }
$taskMetadata = [ordered]@{pid=$taskCreated.ProcessId;manifest_sha256=(Get-FileHash -LiteralPath $taskManifestPath -Algorithm SHA256).Hash;driver_sha256=(Get-FileHash -LiteralPath $taskTestExe -Algorithm SHA256).Hash;runner_sha256=$taskNativeManifest.runner_sha256;source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));pattern=$Pattern}
[IO.File]::WriteAllText((Join-Path $taskEvidence 'source-identity.json'),($taskMetadata | ConvertTo-Json),[Text.UTF8Encoding]::new($false))
Write-Output "Native harness running; PID $($taskCreated.ProcessId); evidence $taskEvidence"
