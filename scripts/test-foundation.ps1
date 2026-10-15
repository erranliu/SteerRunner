param(
    [string]$OutDir = '',
    [string]$Pattern = '^(TestSRStandalonePackage|TestSRServiceOwnership)$'
)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $OutDir) { $OutDir = Join-Path $taskRepo ('.evidence/foundation-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss')) }
$taskEvidence = [IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskEvidence) { throw 'Evidence directory must be new.' }
New-Item -ItemType Directory -Path $taskEvidence | Out-Null
$taskRunnerExe = Join-Path $taskEvidence 'steerrunner.exe'
$taskTestExe = Join-Path $taskEvidence 'integration.test.exe'
$taskHelperExe = Join-Path $taskEvidence 'native-fixture.exe'
Push-Location $taskRepo
try {
    & (Join-Path $PSScriptRoot 'build.ps1') -OutFile $taskRunnerExe
    & go test ./internal/contracts ./internal/cli ./internal/storage ./internal/host -run '^(TestSRCLIContract|TestSRShutdownReceipt|TestSRUnsupportedStorage|TestSRStartupIntent)$' -count=1 -timeout=2m -v 2>&1 | Tee-Object -FilePath (Join-Path $taskEvidence 'unit-foundation.log') | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Foundation unit tests failed; inspect unit-foundation.log.' }
    & go test -c -o $taskTestExe ./tests/integration
    if ($LASTEXITCODE -ne 0) { throw 'Integration fixture build failed.' }
    & go build -o $taskHelperExe ./tests/helpers/native
    if ($LASTEXITCODE -ne 0) { throw 'Native protocol fixture build failed.' }
} finally { Pop-Location }

function Quote-WindowsArgument([string]$Argument) {
    '"' + ($Argument -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"'
}
$taskArgs = @("-test.run=$Pattern", '-test.v', '-test.timeout=5m', '-runner-exe', $taskRunnerExe, '-evidence-dir', $taskEvidence,'-helper-exe',$taskHelperExe)
$taskCommandLine = (@($taskTestExe) + $taskArgs | ForEach-Object { Quote-WindowsArgument $_ }) -join ' '
$taskStartup = New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ ShowWindow = [uint16]0 }
# Test harness only: a CIM-created process does not inherit the tool caller's
# Job chain. The production startup path continues to verify breakaway itself.
$taskCreated = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
    CommandLine = $taskCommandLine
    CurrentDirectory = $taskRepo
    ProcessStartupInformation = $taskStartup
}
if ($taskCreated.ReturnValue -ne 0) { throw "Independent test harness startup failed: $($taskCreated.ReturnValue)" }
$taskProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$($taskCreated.ProcessId)"
if ($taskProcess) {
    $taskOwner = Invoke-CimMethod -InputObject $taskProcess -MethodName GetOwnerSid
    if ($taskOwner.Sid -ne [Security.Principal.WindowsIdentity]::GetCurrent().User.Value) { throw 'Test harness user identity mismatch.' }
}
$taskMetadata = [ordered]@{
    pid = $taskCreated.ProcessId
    pattern = $Pattern
    runner_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $taskRunnerExe).Hash
    fixture_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $taskTestExe).Hash
    source_commit = (& git -C $taskRepo rev-parse HEAD).Trim()
    source_dirty = -not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String))
    go_version = (& go version | Out-String).Trim()
    os_version = [Environment]::OSVersion.VersionString
    machine = $env:COMPUTERNAME
}
[IO.File]::WriteAllText((Join-Path $taskEvidence 'source-identity.json'), ($taskMetadata | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
Write-Output "Independent tests running; PID $($taskCreated.ProcessId); evidence $taskEvidence"
