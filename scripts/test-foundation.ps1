param(
    [string]$OutDir = '',
    [string]$Pattern = '^(TestSRStandalonePackage|TestSRServiceOwnership)$',
    [string]$BaselineExe = '',
    [string]$ContentNativeExecutable = '',
    [string]$ContentNativeModel = '',
    [string]$PreparedArtifactsDirectory = '',
    [string]$RunnerExe = '',
    [ValidateRange(1,60)][int]$TimeoutMinutes = 25
)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if ($BaselineExe) {
    $BaselineExe = (Resolve-Path -LiteralPath $BaselineExe).Path
}
if (-not $OutDir) { $OutDir = Join-Path $taskRepo ('.evidence/foundation-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss')) }
$taskEvidence = [IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskEvidence) { throw 'Evidence directory must be new.' }
New-Item -ItemType Directory -Path $taskEvidence | Out-Null
$taskRunnerExe = if ($RunnerExe) { (Resolve-Path -LiteralPath $RunnerExe).Path } else { Join-Path $taskEvidence 'steerrunner.exe' }
$taskTestExe = Join-Path $taskEvidence 'integration.test.exe'
$taskHelperExe = Join-Path $taskEvidence 'native-fixture.exe'
if ($PreparedArtifactsDirectory) {
    if ($RunnerExe) { throw 'PreparedArtifactsDirectory cannot be combined with RunnerExe; select one frozen artifact source.' }
    if ($BaselineExe -or $ContentNativeExecutable) { throw 'Prepared controlled artifacts cannot be used for native or cross-build tests.' }
    $PreparedArtifactsDirectory = (Resolve-Path -LiteralPath $PreparedArtifactsDirectory).Path
    $taskOriginalIdentity = Get-Content -LiteralPath (Join-Path $PreparedArtifactsDirectory 'source-identity.json') -Raw | ConvertFrom-Json
    $taskOriginalResult = Get-Content -LiteralPath (Join-Path $PreparedArtifactsDirectory 'result.json') -Raw | ConvertFrom-Json
    if ($taskOriginalResult.exit_code -ne 0 -or $taskOriginalResult.scenario_count -lt 1 -or (Select-String -LiteralPath (Join-Path $PreparedArtifactsDirectory 'integration.log') -Pattern '--- SKIP:' -Quiet)) { throw 'Prepared controlled artifacts require a passed prerequisite without skips.' }
    foreach ($taskArtifact in @(@{name='steerrunner.exe';field='runner_sha256'},@{name='integration.test.exe';field='fixture_sha256'},@{name='native-fixture.exe';field='native_fixture_sha256'})) {
        $taskPath = Join-Path $PreparedArtifactsDirectory $taskArtifact.name
        if (-not $taskOriginalIdentity.PSObject.Properties[$taskArtifact.field] -or (Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash -ine $taskOriginalIdentity.($taskArtifact.field)) { throw 'Prepared controlled artifact identity changed or is missing.' }
    }
    $taskRunnerExe = Join-Path $PreparedArtifactsDirectory 'steerrunner.exe'
    $taskTestExe = Join-Path $PreparedArtifactsDirectory 'integration.test.exe'
    $taskHelperExe = Join-Path $PreparedArtifactsDirectory 'native-fixture.exe'
} else {
    Push-Location $taskRepo
    try {
        if (-not $RunnerExe) { & (Join-Path $PSScriptRoot 'build.ps1') -OutFile $taskRunnerExe }
        & go test ./internal/contracts ./internal/cli ./internal/storage ./internal/host -run '^(TestSRCLIContract|TestSRShutdownReceipt|TestSRUnsupportedStorage|TestSRStartupIntent)$' -count=1 -timeout=2m -v 2>&1 | Tee-Object -FilePath (Join-Path $taskEvidence 'unit-foundation.log') | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Foundation unit tests failed; inspect unit-foundation.log.' }
        if ($ContentNativeExecutable) {
            & go test -tags live -c -o $taskTestExe ./tests/integration
        } else {
            & go test -c -o $taskTestExe ./tests/integration
        }
        if ($LASTEXITCODE -ne 0) { throw 'Integration fixture build failed.' }
        & go build -o $taskHelperExe ./tests/helpers/native
        if ($LASTEXITCODE -ne 0) { throw 'Native protocol fixture build failed.' }
    } finally { Pop-Location }

}

function Quote-WindowsArgument([string]$Argument) {
    '"' + ($Argument -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"'
}
# The caller selects the aggregate harness budget. Individual CLI, preparation,
# fault and scenario polling deadlines are unchanged.
$taskArgs = @("-test.run=$Pattern", '-test.v', "-test.timeout=$($TimeoutMinutes)m", '-runner-exe', $taskRunnerExe, '-evidence-dir', $taskEvidence,'-helper-exe',$taskHelperExe)
if ($BaselineExe) { $taskArgs += @('-baseline-exe', $BaselineExe) }
if ($ContentNativeExecutable) { $taskArgs += @('-content-native-exe', (Resolve-Path -LiteralPath $ContentNativeExecutable).Path) }
if ($ContentNativeModel) { $taskArgs += @('-content-native-model', $ContentNativeModel) }
$taskCommandLine = (@($taskTestExe) + $taskArgs | ForEach-Object { Quote-WindowsArgument $_ }) -join ' '
$taskStartup = New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ ShowWindow = [uint16]0 }
# Test harness only: a CIM-created process does not inherit the tool caller's
# Job chain. The production startup path continues to verify breakaway itself.
$taskArtifactIdentity=@{
    runner_sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $taskRunnerExe).Hash
    fixture_sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $taskTestExe).Hash
    native_fixture_sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $taskHelperExe).Hash
}
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
    process_started_at = if($taskProcess){$taskProcess.CreationDate.ToUniversalTime().ToString('o')}else{''}
    pattern = $Pattern
    runner_sha256 = $taskArtifactIdentity.runner_sha256
    fixture_sha256 = $taskArtifactIdentity.fixture_sha256
    native_fixture_sha256 = $taskArtifactIdentity.native_fixture_sha256
    source_commit = (& git -C $taskRepo rev-parse HEAD).Trim()
    source_dirty = -not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String))
    go_version = (& go version | Out-String).Trim()
    os_version = [Environment]::OSVersion.VersionString
    machine = $env:COMPUTERNAME
}
if ($PreparedArtifactsDirectory) {
    $taskMetadata['prepared_artifacts_directory'] = $PreparedArtifactsDirectory
    $taskMetadata.source_commit = $taskOriginalIdentity.source_commit
    $taskMetadata.source_dirty = $taskOriginalIdentity.source_dirty
}
if ($BaselineExe) {
    $taskMetadata['baseline_exe'] = $BaselineExe
    $taskMetadata['baseline_sha256'] = (Get-FileHash -Algorithm SHA256 -LiteralPath $BaselineExe).Hash
}
[IO.File]::WriteAllText((Join-Path $taskEvidence 'source-identity.json'), ($taskMetadata | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
Write-Output "Independent tests running; PID $($taskCreated.ProcessId); evidence $taskEvidence"
