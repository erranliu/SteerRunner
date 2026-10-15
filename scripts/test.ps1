param(
    [ValidateSet('L1','Controlled','L2','L3')][string[]]$Level = @('L1'),
    [string[]]$Scenario = @(),
    [string]$Config = '',
    [string]$Catalog = '',
    [string]$OutDir = '',
    [string]$FixtureParent = 'C:\srt'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $Catalog) { $Catalog = Join-Path $taskRepo 'tests/catalog.json' }
if (-not $OutDir) { $OutDir = Join-Path $taskRepo ('.evidence/tests-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,6)) }
$taskOut = [IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Test report directory must be new' }
New-Item -ItemType Directory -Path $taskOut | Out-Null
$taskSummary = [ordered]@{schema='steerrunner-test-report/v1';source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));started_at=[DateTime]::UtcNow.ToString('o');requested_levels=$Level;status='running';results=@()}
function Save-Summary { [IO.File]::WriteAllText((Join-Path $taskOut 'summary.json'),($taskSummary|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false)) }
Save-Summary
Push-Location $taskRepo
try {
    $taskCatalog = Get-Content -LiteralPath $Catalog -Raw | ConvertFrom-Json
    if ($taskCatalog.schema -ne 'steerrunner-tests/v1') { throw 'Unsupported catalog' }
    $taskIDs = @{}
    foreach ($taskEntry in $taskCatalog.scenarios) {
        if ($taskEntry.id -notmatch '^[a-z][a-z0-9.-]+$' -or $taskIDs.ContainsKey($taskEntry.id) -or $taskEntry.level -notin @('L2','L3') -or $taskEntry.timeout_minutes -lt 1 -or $taskEntry.timeout_minutes -gt 60) { throw 'Invalid or duplicate live catalog entry' }
        if (($taskEntry.level -eq 'L2' -and $taskEntry.target -ne 'local-codex') -or ($taskEntry.target -eq 'orca-environment' -and $taskEntry.level -ne 'L3')) { throw 'Catalog execution target conflicts with test level' }
        $taskIDs[$taskEntry.id] = $true
    }
    $taskSelected = @($taskCatalog.scenarios | Where-Object { $_.level -in $Level -and ($Scenario.Count -eq 0 -or $_.id -in $Scenario) })
    if (@($Scenario | Select-Object -Unique).Count -ne $Scenario.Count) { throw 'Duplicate scenario selection' }
    foreach ($taskID in $Scenario) { if (@($taskSelected | Where-Object { $_.id -eq $taskID }).Count -ne 1) { throw "Scenario was not selected by the requested levels: $taskID" } }
    foreach ($taskLiveLevel in @('L2','L3')) { if ($taskLiveLevel -in $Level -and @($taskSelected | Where-Object { $_.level -eq $taskLiveLevel }).Count -eq 0) { throw "No scenario matched $taskLiveLevel" } }
    if ($taskSelected.Count -gt 0 -and (-not $Config -or -not (Test-Path -LiteralPath $Config))) { throw 'Live levels require an explicit non-secret config; see tests/live-config.example.json' }
    # Unit tests and controlled integration are prerequisites for every live batch.
    foreach ($taskKey in @('STEERRUNNER_ORCA_STATE_PROBE_ROOT','STEERRUNNER_ORCA_WORKSPACE_PROBE_ROOT','STEERRUNNER_ORCA_LOCAL_PIPE_EVIDENCE')) { if ([Environment]::GetEnvironmentVariable($taskKey)) { throw "Unexpected live opt-in in L1: $taskKey" } }
    Write-Host 'Running L1'
    $taskFramework = Join-Path $taskOut 'framework'
    & (Join-Path $PSScriptRoot 'test-framework.ps1') -Evidence $taskFramework
    $taskCoverage = Join-Path $taskOut 'coverage.out'
    & go test @($taskCatalog.l1_packages) -count=1 -timeout=5m -json "-coverprofile=$taskCoverage" 1> (Join-Path $taskOut 'l1.jsonl') 2> (Join-Path $taskOut 'l1-stderr.log')
    $taskUnitCode = $LASTEXITCODE
    # Compile live drivers without running them. This gate precedes all real
    # fixture mutations and token-consuming execution.
    $taskDriver = Join-Path $taskOut 'native.test.exe'
    & go test -tags=live -c -o $taskDriver ./tests/native 1> (Join-Path $taskOut 'native-compile.log') 2> (Join-Path $taskOut 'native-compile-stderr.log')
    $taskCompileCode = $LASTEXITCODE
    $taskZeroGuard = $false
    if ($taskCompileCode -eq 0) {
        $taskZeroEvidence = Join-Path $taskFramework 'native-zero-match'
        New-Item -ItemType Directory -Path $taskZeroEvidence | Out-Null
        & $taskDriver '-test.run=^NoSuchScenario$' -evidence-dir $taskZeroEvidence
        $taskGuardCode = $LASTEXITCODE
        $taskGuardResult = Get-Content -LiteralPath (Join-Path $taskZeroEvidence 'result.json') -Raw | ConvertFrom-Json
        $taskZeroGuard = $taskGuardCode -eq 1 -and $taskGuardResult.exit_code -eq 1 -and $taskGuardResult.scenario_count -eq 0
    }
    $taskEvents = @(Get-Content -LiteralPath (Join-Path $taskOut 'l1.jsonl') | ForEach-Object { $_ | ConvertFrom-Json })
    $taskPassed = @($taskEvents | Where-Object { $_.Action -eq 'pass' -and $_.PSObject.Properties['Test'] -and $_.Test -notmatch '/' }).Count
    $taskSkips = @($taskEvents | Where-Object { $_.Action -eq 'skip' -and $_.PSObject.Properties['Test'] })
    $taskAllowedSkips = @('TestProcessHostCommandHelper','TestServiceStopWorkerHelper','TestOrcaLocalPipeNativeEvidence','TestOrcaNativeStateLiveLifecycle','TestOrcaWorkspaceStartupLive')
    $taskUnexpected = @($taskSkips | Where-Object { $_.Test -notin $taskAllowedSkips })
    $taskSummary.results += @{level='L1';exit_code=$taskUnitCode;test_count=$taskPassed;skipped_tests=@($taskSkips | ForEach-Object { $_.Test });evidence=$taskOut;framework_evidence=$taskFramework;native_driver_compile_exit=$taskCompileCode;native_zero_match_guard=$taskZeroGuard}
    Save-Summary
    if ($taskUnitCode -ne 0 -or $taskPassed -eq 0 -or $taskUnexpected.Count -gt 0 -or $taskCompileCode -ne 0 -or -not $taskZeroGuard) { throw 'L1 failed, had unexpected skips, or live driver compile/zero-match guard failed' }
    if ('Controlled' -in $Level -or $taskSelected.Count -gt 0) {
        Write-Host 'Running controlled integration prerequisite'
        $taskControlled = Join-Path $taskOut 'controlled'
        $taskPattern = '^(' + (($taskCatalog.controlled_scenarios | ForEach-Object { [regex]::Escape($_) }) -join '|') + ')$'
        & (Join-Path $PSScriptRoot 'test-foundation.ps1') -OutDir $taskControlled -Pattern $taskPattern | Out-Host
        $taskUntil = [DateTime]::UtcNow.AddMinutes(6)
        while (-not (Test-Path -LiteralPath (Join-Path $taskControlled 'result.json'))) {
            if ([DateTime]::UtcNow -gt $taskUntil) { throw 'Controlled harness timed out; inspect integration.log' }
            Start-Sleep -Seconds 2
        }
        $taskResult = Get-Content -LiteralPath (Join-Path $taskControlled 'result.json') -Raw | ConvertFrom-Json
        $taskSummary.results += @{level='Controlled';exit_code=$taskResult.exit_code;scenario_count=$taskResult.scenario_count;evidence=$taskControlled}
        Save-Summary
        if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne @($taskCatalog.controlled_scenarios).Count -or (Select-String -LiteralPath (Join-Path $taskControlled 'integration.log') -Pattern '--- SKIP:' -Quiet)) { throw 'Controlled prerequisite failed, skipped or did not execute every selected scenario' }
    }
    foreach ($taskEntry in $taskSelected) {
        $taskRecord = @{level=$taskEntry.level;scenario=$taskEntry.id;status='preparing';manifest=$null;evidence=$null;fixture_record=(Join-Path $taskOut ($taskEntry.id + '-fixture.json'))}
        $taskSummary.results += $taskRecord
        Save-Summary
        try {
            $taskManifest = & (Join-Path $PSScriptRoot 'new-test-fixture.ps1') -Scenario $taskEntry.id -Config $Config -Catalog $Catalog -FixtureParent $FixtureParent -RecordPath $taskRecord.fixture_record
            $taskRecord.manifest = [string]$taskManifest
            $taskRecord.evidence = Join-Path (Split-Path -Parent $taskManifest) 'evidence'
            $taskRecord.status = 'running'
            Save-Summary
            $taskRecord.evidence = & (Join-Path $PSScriptRoot 'test-native.ps1') -Manifest $taskManifest -TimeoutMinutes $taskEntry.timeout_minutes
            $taskRecord.status = 'passed'
        } catch {
            $taskRecord.status = 'failed'
            $taskRecord['error'] = $_.Exception.Message
            Write-Host "$($taskEntry.id): $($_.Exception.Message)" -ForegroundColor Red
        }
        Save-Summary
    }
    $taskSummary.status = if (@($taskSummary.results | Where-Object { $_.ContainsKey('status') -and $_.status -eq 'failed' }).Count) { 'failed' } else { 'passed' }
} catch {
    foreach ($taskFailed in @($taskSummary.results | Where-Object { $_.ContainsKey('status') -and $_.status -in @('preparing','running') })) { $taskFailed.status = 'failed' }
    $taskSummary.status = 'failed'
    $taskSummary['error'] = $_.Exception.Message
    Write-Host $_.Exception.Message -ForegroundColor Red
} finally {
    Pop-Location
    $taskSummary['finished_at'] = [DateTime]::UtcNow.ToString('o')
    Save-Summary
    Write-Host "Test report: $taskOut/summary.json"
}
if ($taskSummary.status -ne 'passed') { exit 1 }
