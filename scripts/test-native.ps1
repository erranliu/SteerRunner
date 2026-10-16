param(
    [Parameter(Mandatory=$true)][string]$Manifest,
    [string]$Pattern = '^TestSRLiveScenario$',
    [ValidateRange(1,60)][int]$TimeoutMinutes = 30,
    [switch]$ValidateOnly
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskManifestPath = [IO.Path]::GetFullPath($Manifest)
$taskNative = Get-Content -LiteralPath $taskManifestPath -Raw | ConvertFrom-Json
if ($taskNative.schema -notin @('steerrunner-test/v1','steerrunner-test/v2')) { throw 'Use scripts/new-test-fixture.ps1 to create a Runner-owned fixture' }
$taskRoot = [IO.Path]::GetFullPath($taskNative.root)
$taskPreparation = Get-Content -LiteralPath (Join-Path $taskRoot 'preparation.json') -Raw | ConvertFrom-Json
if ($taskPreparation.status -ne 'ready' -or $taskPreparation.fixture_id -ne $taskNative.id -or $taskPreparation.base_branch -ne $taskNative.base_branch) { throw 'Fixture preparation is incomplete; do not start an agent' }
$taskEvidence = Join-Path $taskRoot 'evidence'
if (-not $ValidateOnly -and (Test-Path -LiteralPath (Join-Path $taskEvidence 'native.log'))) { throw 'Fixture is frozen after execution; generate a new fixture' }
if ((Get-FileHash -LiteralPath $taskNative.runner_exe -Algorithm SHA256).Hash -ne $taskNative.runner_sha256) { throw 'Frozen Runner executable changed' }
$taskTestExe = Join-Path $taskRoot 'bin/native.test.exe'
$taskResultPath = Join-Path $taskEvidence 'result.json'
if ($ValidateOnly) {
    if ($taskNative.driver -ne 'environment-check') { throw 'Read-only result validation currently supports environment-check fixtures only' }
    $taskOriginal = Get-Content -LiteralPath (Join-Path $taskEvidence 'source-identity.json') -Raw | ConvertFrom-Json
    if ($taskOriginal.manifest_sha256 -ne (Get-FileHash -LiteralPath $taskManifestPath -Algorithm SHA256).Hash -or $taskOriginal.driver_sha256 -ne (Get-FileHash -LiteralPath $taskTestExe -Algorithm SHA256).Hash -or $taskOriginal.runner_sha256 -ne $taskNative.runner_sha256 -or $taskOriginal.scenario -ne $taskNative.scenario) { throw 'Original frozen fixture/build identity changed' }
} else {
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
if ($taskNative.schema -eq 'steerrunner-test/v2') { $taskMetadata['helper_sha256']=$taskNative.stability.helper.sha256;$taskMetadata['runtime_sha256']=$taskNative.stability.runtime_files.sha256;$taskMetadata['network_sha256']=$taskNative.stability.firewall.sha256;$taskMetadata['case_id']=$taskNative.case_id;$taskMetadata['repeat_index']=$taskNative.repeat_index }
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

}
$taskResult = Get-Content -LiteralPath $taskResultPath -Raw | ConvertFrom-Json
if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne 1) { throw "Live scenario failed; inspect $taskEvidence/native.log" }
$taskScenario = Get-Content -LiteralPath (Join-Path $taskEvidence 'scenario.json') -Raw | ConvertFrom-Json
$taskMerge = if ($taskScenario.PSObject.Properties['merge_commit']) { [string]$taskScenario.merge_commit } else { '' }
if ($taskNative.schema -eq 'steerrunner-test/v2') {
    & $taskNative.stability.helper.path validate-result -manifest $taskManifestPath
    if ($LASTEXITCODE -ne 0) { throw 'Stability result lacks case assertions or confirmed cleanup' }
} else {
    if (-not $taskScenario.passed -or -not $taskScenario.run_stopped -or -not $taskScenario.service_stopped) { throw 'Live result lacks confirmed completion or cleanup' }
    if ($taskNative.driver -eq 'environment-check') {
 $taskCheckEnvelope = Get-Content -LiteralPath (Join-Path $taskEvidence 'check-latest.json') -Raw | ConvertFrom-Json
 $taskCheck = $taskCheckEnvelope.data.check
 if (-not $taskCheckEnvelope.ok -or $taskCheck.check_id -ne $taskScenario.run_id -or $taskCheck.status -ne 'completed' -or $taskCheck.result -ne 'passed' -or $taskCheck.cleanup_status -ne 'completed' -or $taskCheck.input.execution.settings.access_mode -ne $taskNative.execution.settings.access_mode -or $taskCheck.actual.access_mode -ne $taskNative.execution.settings.access_mode -or $taskCheck.actual.approval_policy -ne 'never' -or -not $taskCheck.actual.session_id -or -not $taskCheck.actual.turn_id -or $taskCheck.resources.base_sha -ne $taskNative.base_revision -or $taskCheck.resources.head_sha -ne $taskScenario.original_head -or $taskCheck.resources.pr_url -ne $taskScenario.pr_url) { throw 'Original complete Check context/result does not match the frozen native case' }
 foreach ($taskField in @('pr_closed','base_deleted','head_deleted','writer_stopped','local_removed')) { if (-not $taskCheck.resources.$taskField) { throw "Unsettled original Check resource: $taskField" } }
 if (@($taskCheck.items).Count -ne 12) { throw 'Original native manifest is incomplete' }
 foreach ($taskItem in $taskCheck.items) {
  if (-not $taskItem.required -or $taskItem.status -ne 'passed' -or @($taskItem.evidence).Count -eq 0) { throw 'Original native item lacks passed required evidence' }
  foreach ($taskFact in $taskItem.evidence) { if (-not $taskFact.verified -or $taskFact.source -ne 'native_tool_and_owner_verification' -or $taskFact.check_id -ne $taskCheck.check_id -or $taskFact.session_id -ne $taskCheck.actual.session_id -or $taskFact.turn_id -ne $taskCheck.actual.turn_id -or $taskFact.nonce -ne $taskCheck.actual.nonce -or -not $taskFact.operation_id -or -not $taskFact.tool_call_id) { throw 'Original native evidence identity conflicts with its Check' } }
 }

 if (-not $taskScenario.pr_url -or -not $taskScenario.original_head -or $taskMerge) { throw 'Environment check must verify and close its owned PR without merging' }
 } elseif ($taskNative.driver -eq 'development-audited') {
        if (-not $taskScenario.pr_url -or -not $taskScenario.original_head -or $taskMerge) { throw 'Audited diagnostic must verify its PR and leave it unmerged' }
    } elseif (-not $taskMerge) { throw 'Live smoke lacks its verified merge' }
}
Write-Output $taskEvidence
