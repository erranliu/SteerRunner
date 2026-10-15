param(
    [Parameter(Mandatory=$true)][string]$Manifest,
    [string]$Pattern = '^TestSRLiveScenario$',
    [ValidateRange(1,60)][int]$TimeoutMinutes = 30
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskManifestPath = [IO.Path]::GetFullPath($Manifest)
$taskNative = Get-Content -LiteralPath $taskManifestPath -Raw | ConvertFrom-Json
if ($taskNative.schema -ne 'steerrunner-test/v1') { throw 'Use scripts/new-test-fixture.ps1 to create a Runner-owned fixture' }
$taskRoot = [IO.Path]::GetFullPath($taskNative.root)
$taskPreparation = Get-Content -LiteralPath (Join-Path $taskRoot 'preparation.json') -Raw | ConvertFrom-Json
if ($taskPreparation.status -ne 'ready' -or $taskPreparation.fixture_id -ne $taskNative.id -or $taskPreparation.base_branch -ne $taskNative.base_branch) { throw 'Fixture preparation is incomplete; do not start an agent' }
$taskEvidence = Join-Path $taskRoot 'evidence'
if (Test-Path -LiteralPath (Join-Path $taskEvidence 'native.log')) { throw 'Fixture is frozen after execution; generate a new fixture' }
if ((Get-FileHash -LiteralPath $taskNative.runner_exe -Algorithm SHA256).Hash -ne $taskNative.runner_sha256) { throw 'Frozen Runner executable changed' }
$taskTestExe = Join-Path $taskRoot 'bin/native.test.exe'
Push-Location $taskRepo
try {
    & go test -tags=live -c -o $taskTestExe ./tests/native
    if ($LASTEXITCODE -ne 0) { throw 'Native driver build failed' }
} finally { Pop-Location }
function Quote-WindowsArgument([string]$Argument) { '"' + ($Argument -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"' }
$taskArguments = @($taskTestExe,"-test.run=$Pattern",'-test.v',("-test.timeout={0}m" -f $TimeoutMinutes),'-native-manifest',$taskManifestPath,'-evidence-dir',$taskEvidence)
$taskCommandLine = ($taskArguments | ForEach-Object { Quote-WindowsArgument $_ }) -join ' '
$taskStartup = New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ShowWindow=[uint16]0}
$taskCreated = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=$taskCommandLine;CurrentDirectory=$taskRepo;ProcessStartupInformation=$taskStartup}
if ($taskCreated.ReturnValue -ne 0) { throw "Independent native harness startup failed: $($taskCreated.ReturnValue)" }
$taskMetadata = [ordered]@{pid=$taskCreated.ProcessId;manifest_sha256=(Get-FileHash -LiteralPath $taskManifestPath -Algorithm SHA256).Hash;driver_sha256=(Get-FileHash -LiteralPath $taskTestExe -Algorithm SHA256).Hash;runner_sha256=$taskNative.runner_sha256;source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));pattern=$Pattern;scenario=$taskNative.scenario;level=$taskNative.level}
[IO.File]::WriteAllText((Join-Path $taskEvidence 'source-identity.json'),($taskMetadata|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
Write-Host "Live harness PID $($taskCreated.ProcessId); evidence $taskEvidence"
$taskUntil = [DateTime]::UtcNow.AddMinutes($TimeoutMinutes).AddSeconds(30)
$taskResultPath = Join-Path $taskEvidence 'result.json'
$taskPreviousStage = ''
while (-not (Test-Path -LiteralPath $taskResultPath)) {
    if ([DateTime]::UtcNow -gt $taskUntil) { throw "Harness did not finish; preserve $taskManifestPath and reconcile original resources from caller-commands.jsonl" }
    $taskStagePath = Join-Path $taskEvidence 'scenario.json'
    if (Test-Path -LiteralPath $taskStagePath) {
        try {
            $taskStage = (Get-Content -LiteralPath $taskStagePath -Raw | ConvertFrom-Json).stage
            $taskViewPath = Join-Path $taskEvidence 'latest-run.json'
            if (Test-Path -LiteralPath $taskViewPath) {
                $taskView = Get-Content -LiteralPath $taskViewPath -Raw | ConvertFrom-Json
                $taskStage += ' / ' + $taskView.step + ' / ' + $taskView.phase
            }
            if ($taskStage -ne $taskPreviousStage) { Write-Host "$($taskNative.level) $($taskNative.scenario): $taskStage"; $taskPreviousStage = $taskStage }
        } catch { Write-Verbose 'Waiting for an in-progress checkpoint write' }
    }
    if (-not (Get-Process -Id $taskCreated.ProcessId -ErrorAction SilentlyContinue)) {
        if (-not (Test-Path -LiteralPath $taskResultPath)) { throw "Harness exited without a result; preserve $taskEvidence and reconcile original resources" }
    }
    Start-Sleep -Seconds 2
}
$taskResult = Get-Content -LiteralPath $taskResultPath -Raw | ConvertFrom-Json
if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne 1) { throw "Live scenario failed; inspect $taskEvidence/native.log" }
$taskScenario = Get-Content -LiteralPath (Join-Path $taskEvidence 'scenario.json') -Raw | ConvertFrom-Json
if (-not $taskScenario.passed -or -not $taskScenario.run_stopped -or -not $taskScenario.service_stopped) { throw 'Live result lacks confirmed completion or cleanup' }
if ($taskNative.driver -eq 'development-audited') {
    if (-not $taskScenario.pr_url -or -not $taskScenario.original_head -or $taskScenario.merge_commit) { throw 'Audited diagnostic must verify its PR and leave it unmerged' }
} elseif (-not $taskScenario.merge_commit) { throw 'Live smoke lacks its verified merge' }
Write-Output $taskEvidence
