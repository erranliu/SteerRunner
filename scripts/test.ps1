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
    [switch]$SkipAdminRequired,
    [switch]$ListAvailability,
    [string]$RunnerExe = '',
    [string]$ConfigHelper = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'live-catalog.ps1')
. (Join-Path $PSScriptRoot 'test-scheduler.ps1')
. (Join-Path $PSScriptRoot 'test-configuration.ps1')
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
foreach ($taskParameter in @('Config','Catalog','RunnerExe','ConfigHelper','FeedbackQualification','FixtureParent')) {
    $taskValue = Get-Variable -Name $taskParameter -ValueOnly
    if ($taskValue) { Set-Variable -Name $taskParameter -Value $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($taskValue) }
}
if (-not $Catalog) { $Catalog = Join-Path $taskRepo 'tests/catalog.json' }
if (-not $OutDir -and -not $ListAvailability) { $OutDir = Join-Path $taskRepo ('.evidence/tests-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,6)) }
$taskOut = if($OutDir){[IO.Path]::GetFullPath($OutDir)}else{''}
if ($taskOut) {
    if (Test-Path -LiteralPath $taskOut) { throw 'Test report directory must be new' }
    New-Item -ItemType Directory -Path $taskOut | Out-Null
}
$taskSummary = [ordered]@{schema='steerrunner-test-report/v1';source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));started_at=[DateTime]::UtcNow.ToString('o');requested_levels=$Level;status='running';results=@()}
$taskSummary['executed_case_count']=0
$taskSummary['passed_case_count']=0
$taskSummary['first_pass_rate']=0
function Save-Summary { if(-not $taskOut){return}; $taskTemporary=Join-Path $taskOut 'summary.json.tmp';[IO.File]::WriteAllText($taskTemporary,($taskSummary|ConvertTo-Json -Depth 70),[Text.UTF8Encoding]::new($false));[IO.File]::Move($taskTemporary,(Join-Path $taskOut 'summary.json'),$true) }
$taskExitCode = 2
$taskExpanded = @()
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
        if (($taskEntry.level -eq 'L2' -and $taskEntry.target -ne 'local-codex') -or ($taskEntry.target -in @('orca-environment','orca-local') -and $taskEntry.level -ne 'L3')) { throw 'Catalog execution target conflicts with test level' }
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
    if ($ListAvailability -and @($Level | Where-Object {$_ -notin @('L2','L3')}).Count) { throw 'Availability only supports L2/L3 selections' }
    $taskAssessment = $null
    $taskLiveSelected = @($taskSelected | Where-Object {$_.target -ne 'controlled'})
    if ($taskLiveSelected.Count) {
        if (-not $RunnerExe) {
            $RunnerExe = if($ListAvailability){Join-Path $taskRepo 'bin/steerrunner.exe'}else{Join-Path $taskOut 'bin/steerrunner.exe'}
            if (-not $ListAvailability) { & (Join-Path $PSScriptRoot 'build.ps1') -OutFile $RunnerExe | Out-Host }
        }
        if (-not $ConfigHelper) {
            $ConfigHelper = if($ListAvailability){Join-Path $taskRepo 'bin/test-config.exe'}else{Join-Path $taskOut 'bin/test-config.exe'}
            if (-not $ListAvailability) { & go build -trimpath -o $ConfigHelper ./tests/helpers/native; if($LASTEXITCODE -ne 0){throw 'Test configuration helper build failed'} }
        }
        $taskAssessment = Get-SRTestConfiguration -RunnerExe $RunnerExe -Helper $ConfigHelper -Config $Config
        $taskSummary['baseline'] = @{configuration_issues=@($taskAssessment.baseline);l2_verified=$false}
        $taskSummary['availability'] = @($taskAssessment.cases | Where-Object {$_.scenario_id -in @($taskSelected.id)})
        $taskSummary['runner_exe'] = $taskAssessment.runner_exe
        $taskSummary['runner_sha256'] = $taskAssessment.runner_sha256
        $taskSummary['resource_digest'] = $taskAssessment.resource_digest
        $taskSummary['source_config_digest'] = $taskAssessment.source_digest
        $taskSummary['requirements_sha256'] = $taskAssessment.requirements_digest
        $taskSummary['helper_sha256'] = $taskAssessment.helper_sha256
        if ($ListAvailability) {
            $taskSummary.status='availability'
            Save-Summary
            $taskSummary | ConvertTo-Json -Depth 70
            exit 0
        }
        $Config = Join-Path $taskOut 'frozen-config.json'
        [IO.File]::WriteAllText($Config,($taskAssessment.configuration | ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
        $taskSummary['configuration_sha256'] = (Get-FileHash -LiteralPath $Config -Algorithm SHA256).Hash.ToLowerInvariant()
        # Workers consume the batch catalog rather than a mutable caller file.
        $Catalog = Join-Path $taskOut 'frozen-catalog.json'
        [IO.File]::WriteAllText($Catalog,($taskCatalog | ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
    }
    $taskExpanded = @()
    foreach ($taskRound in 1..$Repeat) {
        foreach ($taskEntry in @($taskSelected | Sort-Object { if ($_.level -eq 'L4') { 1 } else { 0 } })) {
            foreach ($taskCase in @(Get-SRLiveCases $taskEntry)) {
                $taskExcluded = $SkipAdminRequired -and (Test-SRAdminCase $taskEntry.id $taskCase.id)
                $taskAvailability = if($taskEntry.target -eq 'controlled'){$null}else{@($taskSummary.availability | Where-Object {$_.scenario_id -ceq $taskEntry.id -and $_.case_id -ceq $taskCase.id})[0]}
                $taskExpanded += @{entry=$taskEntry;case=$taskCase;repeat_index=$taskRound;excluded=$taskExcluded;unavailable=($null -ne $taskAvailability -and $taskAvailability.availability -ne 'available');availability=$taskAvailability}
            }
        }
    }
    $taskSummary['selected_scenarios'] = @($taskSelected | ForEach-Object { $_.id })
    $taskSummary['repeat'] = $Repeat
    $taskSummary['planned_case_count'] = $taskExpanded.Count
    $taskSummary['authorized_exclusion_count'] = @($taskExpanded | Where-Object { $_.excluded }).Count
    $taskSummary['scope'] = if ($SkipAdminRequired) { 'admin-free-subset' } else { 'selected-complete' }
    $taskSummary.results = @(Merge-SRCaseRecords -Executions $taskExpanded -Records @())
    Save-Summary
    $taskExitCode = 1
    if ($taskLiveSelected.Count -and @($taskExpanded | Where-Object {-not $_.excluded -and -not $_.unavailable}).Count -eq 0) {
        $taskSummary.results=@($taskExpanded | ForEach-Object {@{level=$_.entry.level;scenario=$_.entry.id;case_id=$_.case.id;repeat_index=$_.repeat_index;status=$(if($_.excluded){'excluded'}else{'unavailable'});availability=$_.availability}})
        $taskSummary.status='incomplete'
        $taskSummary['unavailable_case_count']=@($taskSummary.results | Where-Object {$_.status -eq 'unavailable'}).Count
        Set-SRLevelCoverage -Summary $taskSummary -Registered @(Get-SRCaseRegistry) -Records $taskSummary.results -Levels $Level
        Save-Summary
        exit 1
    }
    # Unit tests and controlled integration are prerequisites for every live batch.
    foreach ($taskKey in @('STEERRUNNER_ORCA_STATE_PROBE_ROOT','STEERRUNNER_ORCA_WORKSPACE_PROBE_ROOT','STEERRUNNER_ORCA_LOCAL_PIPE_EVIDENCE','STEERRUNNER_ORCA_IPC_PROBE_ROOT','STEERRUNNER_ORCA_STOP_PROBE_ROOT')) { if ([Environment]::GetEnvironmentVariable($taskKey)) { throw "Unexpected live opt-in in L1: $taskKey" } }
    Write-Host 'Running L1'
    $taskFramework = Join-Path $taskOut 'framework'
    & (Join-Path $PSScriptRoot 'test-framework.ps1') -Evidence $taskFramework
    $taskCoverage = Join-Path $taskOut 'coverage.out'
    & go test @($taskCatalog.l1_packages) -count=1 -timeout=5m -json "-coverprofile=$taskCoverage" 1> (Join-Path $taskOut 'l1.jsonl') 2> (Join-Path $taskOut 'l1-stderr.log')
    $taskUnitCode = $LASTEXITCODE
    # The two formal native extensions are part of L1, including on machines
    # without model credentials or a running Orca instance.
    $taskExtensionLog = Join-Path $taskOut 'terminal-extensions.tap'
    & node --test --test-reporter=tap internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs internal/execution/adapters/orcatransport/extensions/dsh-managed.test.mjs 1> $taskExtensionLog 2> (Join-Path $taskOut 'terminal-extensions-stderr.log')
    $taskExtensionCode = $LASTEXITCODE
    $taskExtensionResult = Get-Content -LiteralPath $taskExtensionLog -Raw
    $taskExtensionCount = if ($taskExtensionResult -match '(?m)^# tests (\d+)\s*$') { [int]$Matches[1] } else { 0 }
    $taskExtensionsPassed = $taskExtensionCode -eq 0 -and $taskExtensionCount -gt 0 -and $taskExtensionResult -match '(?m)^# fail 0\s*$' -and $taskExtensionResult -match '(?m)^# skipped 0\s*$'
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
    $taskSummary.results += @{level='L1';exit_code=$taskUnitCode;test_count=$taskPassed;skipped_tests=@($taskSkips | ForEach-Object { $_.Test });evidence=$taskOut;framework_evidence=$taskFramework;native_driver_compile_exit=$taskCompileCode;native_zero_match_guard=$taskZeroGuard;terminal_extension_exit=$taskExtensionCode;terminal_extension_test_count=$taskExtensionCount;terminal_extensions_passed=$taskExtensionsPassed}
    Save-Summary
    if ($taskUnitCode -ne 0 -or $taskPassed -eq 0 -or $taskUnexpected.Count -gt 0 -or $taskCompileCode -ne 0 -or -not $taskZeroGuard -or -not $taskExtensionsPassed) { throw 'L1 failed, had unexpected skips, or native extension/driver guards failed' }
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
    if (@($taskExpanded | Where-Object { $_.entry.id -eq 'local-codex.smoke' -and -not $_.unavailable -and -not $_.excluded }).Count) {
        Write-Host 'Running L2 native Git boundary prerequisite (no model or publication)'
        $taskGitConfig = Get-Content -LiteralPath $Config -Raw | ConvertFrom-Json
        & go test -tags=live ./internal/execution/adapters/codex -run '^TestLinkedGitNativeCommit$' -count=1 -timeout=3m -v -args -sandbox-auth-native $taskGitConfig.configurations.'local-codex'.execution.settings.executable 1> (Join-Path $taskOut 'l2-git-boundary.log') 2> (Join-Path $taskOut 'l2-git-boundary-stderr.log')
        $taskGitCode = $LASTEXITCODE
        $taskSummary.results += @{level='L2-GitBoundary';exit_code=$taskGitCode;evidence=(Join-Path $taskOut 'l2-git-boundary.log')}
        Save-Summary
        if ($taskGitCode -ne 0) { throw 'L2 native Git commit or repository isolation failed' }
    }
    $taskPrerequisites = @($taskSummary.results | Where-Object {-not $_.ContainsKey('case_id')})
    $taskResolvedConfig=if($Config){(Resolve-Path -LiteralPath $Config).ProviderPath}else{''}
    $taskResolvedProof=if($FeedbackQualification){(Resolve-Path -LiteralPath $FeedbackQualification).ProviderPath}else{''}
    $taskConfigSHA=if($taskAssessment){$taskSummary.configuration_sha256}else{''}
    $taskRunnerSHA=if($taskAssessment){$taskSummary.runner_sha256}else{''}
    $taskHelperSHA=if($taskAssessment){$taskSummary.helper_sha256}else{''}
    $taskRequirementsSHA=if($taskAssessment){$taskSummary.requirements_sha256}else{''}
    Invoke-SRCaseBatch -Executions $taskExpanded -Output $taskOut -Config $taskResolvedConfig -Catalog ((Resolve-Path -LiteralPath $Catalog).ProviderPath) -FixtureParent $FixtureParent -MaxParallel $MaxParallel -FeedbackQualification $taskResolvedProof -RunnerExe $RunnerExe -ConfigHelper $ConfigHelper -ConfigurationSHA256 $taskConfigSHA -RunnerSHA256 $taskRunnerSHA -HelperSHA256 $taskHelperSHA -RequirementsSHA256 $taskRequirementsSHA -Checkpoint {
        param($Records)
        $taskSummary.results = @($taskPrerequisites) + @(Merge-SRCaseRecords -Executions $taskExpanded -Records $Records)
        Save-Summary
    }
    $taskSummary.status = Get-SRBatchOutcome -Records $taskSummary.results -SelectedScenarioCount $taskSelected.Count -AuthorizedExclusions $taskSummary.authorized_exclusion_count
} catch {
    foreach ($taskFailed in @($taskSummary.results | Where-Object { $_.ContainsKey('status') -and $_.status -in @('preparing','running') })) { $taskFailed.status = 'failed' }
    $taskSummary.status = 'failed'
    $taskSummary['error'] = $_.Exception.Message
    Write-Host $_.Exception.Message -ForegroundColor Red
} finally {
    Pop-Location
    if ($taskExpanded.Count) {
        $taskSummary['executed_case_count'] = @($taskSummary.results | Where-Object { $_.ContainsKey('case_id') -and $_.status -in @('preparing','running','passed','failed') }).Count
        $taskSummary['passed_case_count'] = @($taskSummary.results | Where-Object { $_.ContainsKey('case_id') -and $_.status -eq 'passed' }).Count
        $taskSummary['first_pass_rate'] = if ($taskSummary.executed_case_count) { $taskSummary.passed_case_count / $taskSummary.executed_case_count } else { 0 }
        $taskSummary['unavailable_case_count'] = @($taskSummary.results | Where-Object { $_.ContainsKey('status') -and $_.status -eq 'unavailable' }).Count
        Set-SRLevelCoverage -Summary $taskSummary -Registered @(Get-SRCaseRegistry) -Records @($taskSummary.results | Where-Object {$_.ContainsKey('case_id')}) -Levels $Level
        if ($taskAssessment -and $taskSummary.levels.ContainsKey('L2')) { $taskSummary.baseline.l2_verified = $taskSummary.levels.L2.complete_level_passed }
    }
    $taskSummary['finished_at'] = [DateTime]::UtcNow.ToString('o')
    Save-Summary
    if ($taskOut -and -not $ListAvailability) { Write-Host "Test report: $taskOut/summary.json" }
}
if ($taskSummary.status -notin @('passed','passed_with_exclusions')) { exit $taskExitCode }
