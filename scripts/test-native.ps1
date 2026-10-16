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
if ($taskNative.schema -notin @('steerrunner-test/v1','steerrunner-test/v2')) { throw 'Use scripts/new-test-fixture.ps1 to create a Runner-owned fixture' }
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
if ($taskNative.driver -eq 'development-core') {
    $taskProof = Get-Content -LiteralPath $taskNative.feedback_qualification.path -Raw | ConvertFrom-Json
    if ($taskProof.identity.driver_sha256.ToLowerInvariant() -cne (Get-FileHash -LiteralPath $taskTestExe -Algorithm SHA256).Hash.ToLowerInvariant()) { throw 'Core smoke feedback qualification was produced by a different native driver build' }
}
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
$taskResult = Get-Content -LiteralPath $taskResultPath -Raw | ConvertFrom-Json
if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne 1) { throw "Live scenario failed; inspect $taskEvidence/native.log" }
$taskScenario = Get-Content -LiteralPath (Join-Path $taskEvidence 'scenario.json') -Raw | ConvertFrom-Json
if ($taskNative.schema -eq 'steerrunner-test/v2') {
    & $taskNative.stability.helper.path validate-result -manifest $taskManifestPath
    if ($LASTEXITCODE -ne 0) { throw 'Stability result lacks case assertions or confirmed cleanup' }
} else {
    . (Join-Path $PSScriptRoot 'native-result.ps1')
    Assert-SRNativeScenario $taskNative $taskResult $taskScenario
    if($taskNative.driver -eq 'development-audited'){
        $taskView=Get-Content -LiteralPath (Join-Path $taskEvidence 'latest-run.json') -Raw|ConvertFrom-Json
        $taskThread=Get-Content -LiteralPath (Join-Path $taskRoot ('s/runs/'+$taskScenario.run_id+'/app-server/thread.json')) -Raw|ConvertFrom-Json
        $taskModels=Assert-SRAuditModels $taskNative $taskView $taskThread.thread.path
        [IO.File]::WriteAllText((Join-Path $taskEvidence 'audit-models.json'),($taskModels|ConvertTo-Json -Depth 10),[Text.UTF8Encoding]::new($false))
    }
}
Write-Output $taskEvidence
