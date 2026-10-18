param(
    [ValidateSet('L1','Controlled','L2','L3','L4')][string[]]$Level = @('L1'),
    [string[]]$Scenario = @(),
    [string]$Config = '',
    [string]$FeedbackQualification = '',
    [string]$Catalog = '',
    [string]$OutDir = '',
    [string]$FixtureParent = 'C:\srt',
    [int]$Repeat = 1,
    [int]$MaxParallel = 1,
    [switch]$SkipAdminRequired
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'live-catalog.ps1')
. (Join-Path $PSScriptRoot 'test-scheduler.ps1')
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $Catalog) { $Catalog = Join-Path $taskRepo 'tests/catalog.json' }
if (-not $OutDir) { $OutDir = Join-Path $taskRepo ('.evidence/tests-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,6)) }
$taskOut = [IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Test report directory must be new' }
New-Item -ItemType Directory -Path $taskOut | Out-Null
$taskSummary = [ordered]@{schema='steerrunner-test-report/v1';source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));started_at=[DateTime]::UtcNow.ToString('o');requested_levels=$Level;status='running';results=@()}
$taskSummary['executed_case_count']=0
$taskSummary['passed_case_count']=0
$taskSummary['first_pass_rate']=0
function Save-Summary { $taskTemporary=Join-Path $taskOut 'summary.json.tmp';[IO.File]::WriteAllText($taskTemporary,($taskSummary|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false));[IO.File]::Move($taskTemporary,(Join-Path $taskOut 'summary.json'),$true) }
Save-Summary
Push-Location $taskRepo
try {
    if ($MaxParallel -lt 1 -or $MaxParallel -gt 8) { throw 'MaxParallel must be between 1 and 8' }
    $taskSummary['max_parallel'] = $MaxParallel
    if ($Repeat -lt 1 -or $Repeat -gt 10) { throw 'Repeat must be between 1 and 10' }
    if ($Repeat -ne 1 -and 'L2' -notin $Level -and 'L3' -notin $Level -and 'L4' -notin $Level) { throw 'Repeat requires a live level' }
    $taskCatalog = Get-Content -LiteralPath $Catalog -Raw | ConvertFrom-Json
    if ($taskCatalog.schema -ne 'steerrunner-tests/v1') { throw 'Unsupported catalog' }
    $taskIDs = @{}
    foreach ($taskEntry in $taskCatalog.scenarios) {
        if ($taskEntry.id -notmatch '^[a-z][a-z0-9.-]+$' -or $taskIDs.ContainsKey($taskEntry.id) -or $taskEntry.level -notin @('L2','L3','L4') -or $taskEntry.timeout_minutes -lt 1 -or $taskEntry.timeout_minutes -gt 60) { throw 'Invalid or duplicate live catalog entry' }
        if (($taskEntry.level -eq 'L2' -and $taskEntry.target -ne 'local-codex') -or ($taskEntry.target -eq 'orca-environment' -and $taskEntry.level -ne 'L3')) { throw 'Catalog execution target conflicts with test level' }
        if (($taskEntry.level -eq 'L4') -ne ($taskEntry.driver -eq 'controlled-integration') -or ($taskEntry.level -eq 'L4' -and $taskEntry.target -ne 'controlled')) { throw 'Controlled edge scenarios require L4 and the controlled target' }
        $taskIDs[$taskEntry.id] = $true
        $null = @(Get-SRLiveCases $taskEntry)
        foreach ($taskCase in @(Get-SRLiveCases $taskEntry)) { if ($taskEntry.level -eq 'L4' -and $taskCase.test -notmatch '^TestSREdge[A-Za-z]+$') { throw 'L4 must bind an exact registered controlled test' } }
    }
    $taskSelected = @($taskCatalog.scenarios | Where-Object { $_.level -in $Level -and ($Scenario.Count -eq 0 -or $_.id -in $Scenario) })
    if ($Scenario.Count -eq 0) {
        foreach ($taskRequired in @(Get-SRCaseRegistry | Where-Object { $_.level -in $Level })) {
            if ($taskRequired.id -notin @($taskSelected | ForEach-Object { $_.id })) { throw "Complete level catalog is missing registered scenario: $($taskRequired.id)" }
        }
    }
    if (@($Scenario | Select-Object -Unique).Count -ne $Scenario.Count) { throw 'Duplicate scenario selection' }
    foreach ($taskID in $Scenario) { if (@($taskSelected | Where-Object { $_.id -eq $taskID }).Count -ne 1) { throw "Scenario was not selected by the requested levels: $taskID" } }
    foreach ($taskLiveLevel in @('L2','L3','L4')) { if ($taskLiveLevel -in $Level -and @($taskSelected | Where-Object { $_.level -eq $taskLiveLevel }).Count -eq 0) { throw "No scenario matched $taskLiveLevel" } }
    if (@($taskSelected | Where-Object { $_.target -ne 'controlled' }).Count -gt 0 -and (-not $Config -or -not (Test-Path -LiteralPath $Config))) { throw 'Live levels require an explicit non-secret config; see tests/live-config.example.json' }
    if (@($taskSelected | Where-Object { $_.driver -eq 'development-audited' }).Count) {
        $taskAuditConfig=Get-Content -LiteralPath $Config -Raw | ConvertFrom-Json
        $taskAuditModel=$taskAuditConfig.PSObject.Properties['audit_model']
        foreach($taskAuditEntry in @($taskSelected|Where-Object {$_.driver -eq 'development-audited'})){
            $taskModelProperty=if($taskAuditEntry.target -eq 'orca-environment' -and $taskAuditConfig.PSObject.Properties['remote_model']){'remote_model'}else{'model'}
            $taskExecutionModel=$taskAuditConfig.PSObject.Properties[$taskModelProperty]
            if (-not $taskExecutionModel -or -not $taskAuditModel -or [string]::IsNullOrWhiteSpace([string]$taskExecutionModel.Value) -or [string]::IsNullOrWhiteSpace([string]$taskAuditModel.Value) -or $taskExecutionModel.Value -ceq $taskAuditModel.Value) { throw 'Audited scenario requires two explicit, different execution and audit_model values before any fixture is created' }
        }
    }
    $taskExpanded = @()
    foreach ($taskRound in 1..$Repeat) {
        foreach ($taskEntry in @($taskSelected | Sort-Object { if ($_.level -eq 'L4') { 1 } else { 0 } })) {
            foreach ($taskCase in @(Get-SRLiveCases $taskEntry)) {
                $taskExcluded = $SkipAdminRequired -and (Test-SRAdminCase $taskEntry.id $taskCase.id)
                $taskExpanded += @{entry=$taskEntry;case=$taskCase;repeat_index=$taskRound;excluded=$taskExcluded}
            }
        }
    }
    $taskSummary['selected_scenarios'] = @($taskSelected | ForEach-Object { $_.id })
    $taskSummary['repeat'] = $Repeat
    $taskSummary['planned_case_count'] = $taskExpanded.Count
    $taskSummary['authorized_exclusion_count'] = @($taskExpanded | Where-Object { $_.excluded }).Count
    $taskSummary['scope'] = if ($SkipAdminRequired) { 'admin-free-subset' } else { 'selected-complete' }
    Save-Summary
    if (@($taskExpanded | Where-Object { -not $_.excluded -and $_.entry.driver -eq 'execution-stability' }).Count) {
        $taskNeedsAdmin = @($taskExpanded | Where-Object { -not $_.excluded -and (Test-SRAdminCase $_.entry.id $_.case.id) }).Count -gt 0
        & (Join-Path $PSScriptRoot 'stability-preflight.ps1') -Config $Config -RequireAdmin:$taskNeedsAdmin
    }
    # Unit tests and controlled integration are prerequisites for every live batch.
    foreach ($taskKey in @('STEERRUNNER_ORCA_STATE_PROBE_ROOT','STEERRUNNER_ORCA_WORKSPACE_PROBE_ROOT','STEERRUNNER_ORCA_LOCAL_PIPE_EVIDENCE','STEERRUNNER_ORCA_IPC_PROBE_ROOT')) { if ([Environment]::GetEnvironmentVariable($taskKey)) { throw "Unexpected live opt-in in L1: $taskKey" } }
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
    $taskAllowedSkips = @('TestProcessHostCommandHelper','TestServiceStopWorkerHelper','TestOrcaLocalPipeNativeEvidence','TestOrcaLocalIPCNativeProbe','TestOrcaNativeStateLiveLifecycle','TestOrcaWorkspaceStartupLive','TestAuthenticationNativeToolBoundary')
    $taskUnexpected = @($taskSkips | Where-Object { $_.Test -notin $taskAllowedSkips })
    $taskSummary.results += @{level='L1';exit_code=$taskUnitCode;test_count=$taskPassed;skipped_tests=@($taskSkips | ForEach-Object { $_.Test });evidence=$taskOut;framework_evidence=$taskFramework;native_driver_compile_exit=$taskCompileCode;native_zero_match_guard=$taskZeroGuard}
    Save-Summary
    if ($taskUnitCode -ne 0 -or $taskPassed -eq 0 -or $taskUnexpected.Count -gt 0 -or $taskCompileCode -ne 0 -or -not $taskZeroGuard) { throw 'L1 failed, had unexpected skips, or live driver compile/zero-match guard failed' }
    if ('Controlled' -in $Level -or $taskSelected.Count -gt 0) {
        Write-Host 'Running controlled integration prerequisite'
        $taskControlled = Join-Path $taskOut 'controlled'
        $taskPattern = '^(' + (($taskCatalog.controlled_scenarios | ForEach-Object { [regex]::Escape($_) }) -join '|') + ')$'
        & (Join-Path $PSScriptRoot 'test-foundation.ps1') -OutDir $taskControlled -Pattern $taskPattern | Out-Host
        # The child owns a 15m aggregate test deadline. Do not declare timeout
        # before that contract ends; allow only result-file/cleanup latency.
        $taskUntil = [DateTime]::UtcNow.AddMinutes(15).AddSeconds(30)
        while (-not (Test-Path -LiteralPath (Join-Path $taskControlled 'result.json'))) {
            if ([DateTime]::UtcNow -gt $taskUntil) { throw 'Controlled harness timed out; inspect integration.log' }
            Start-Sleep -Seconds 2
        }
        $taskResult = Get-Content -LiteralPath (Join-Path $taskControlled 'result.json') -Raw | ConvertFrom-Json
        $taskSummary.results += @{level='Controlled';exit_code=$taskResult.exit_code;scenario_count=$taskResult.scenario_count;evidence=$taskControlled}
        Save-Summary
        if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne @($taskCatalog.controlled_scenarios).Count -or (Select-String -LiteralPath (Join-Path $taskControlled 'integration.log') -Pattern '--- SKIP:' -Quiet)) { throw 'Controlled prerequisite failed, skipped or did not execute every selected scenario' }
    }
    if (@($taskSelected | Where-Object { $_.id -eq 'local-codex.smoke' -and $_.level -eq 'L2' }).Count) {
        Write-Host 'Running L2 native Git boundary prerequisite (no model or publication)'
        $taskGitConfig = Get-Content -LiteralPath $Config -Raw | ConvertFrom-Json
        & go test -tags=live ./internal/execution/adapters/codex -run '^TestLinkedGitNativeCommit$' -count=1 -timeout=3m -v -args -sandbox-auth-native $taskGitConfig.codex_executable 1> (Join-Path $taskOut 'l2-git-boundary.log') 2> (Join-Path $taskOut 'l2-git-boundary-stderr.log')
        $taskGitCode = $LASTEXITCODE
        $taskSummary.results += @{level='L2-GitBoundary';exit_code=$taskGitCode;evidence=(Join-Path $taskOut 'l2-git-boundary.log')}
        Save-Summary
        if ($taskGitCode -ne 0) { throw 'L2 native Git commit or repository isolation failed' }
    }
    $taskPrerequisites = @($taskSummary.results)
    $taskResolvedConfig=if($Config){(Resolve-Path -LiteralPath $Config).ProviderPath}else{''}
    $taskResolvedProof=if($FeedbackQualification){(Resolve-Path -LiteralPath $FeedbackQualification).ProviderPath}else{''}
    Invoke-SRCaseBatch -Executions $taskExpanded -Output $taskOut -Config $taskResolvedConfig -Catalog ((Resolve-Path -LiteralPath $Catalog).ProviderPath) -FixtureParent $FixtureParent -MaxParallel $MaxParallel -FeedbackQualification $taskResolvedProof -Checkpoint {
        param($Records)
        $taskSummary.results = @($taskPrerequisites) + @($Records)
        Save-Summary
    }
    $taskSummary['executed_case_count'] = @($taskSummary.results | Where-Object { $_.ContainsKey('case_id') -and $_.status -ne 'excluded' }).Count
    $taskSummary['passed_case_count'] = @($taskSummary.results | Where-Object { $_.ContainsKey('case_id') -and $_.status -eq 'passed' }).Count
    $taskSummary['first_pass_rate'] = if ($taskSummary.executed_case_count) { $taskSummary.passed_case_count / $taskSummary.executed_case_count } else { 0 }
    $taskSummary.status = if (@($taskSummary.results | Where-Object { $_.ContainsKey('status') -and $_.status -eq 'failed' }).Count -or $taskSummary.executed_case_count -ne ($taskExpanded.Count - $taskSummary.authorized_exclusion_count)) { 'failed' } elseif ($taskSummary.authorized_exclusion_count) { 'passed_with_exclusions' } else { 'passed' }
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
if ($taskSummary.status -notin @('passed','passed_with_exclusions')) { exit 1 }
