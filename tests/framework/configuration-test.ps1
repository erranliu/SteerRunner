param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
. (Join-Path $Repository 'scripts/test-configuration.ps1')
. (Join-Path $Repository 'scripts/test-scheduler.ps1')
$taskRoot=Join-Path $Evidence 'configuration'
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskRunner=Join-Path $taskRoot 'runner.exe'
$taskHelper=Join-Path $taskRoot 'test-config.exe'
& (Join-Path $Repository 'scripts/build.ps1') -OutFile $taskRunner | Out-Null
Push-Location $Repository
try { & go build -trimpath -o $taskHelper ./tests/helpers/native; if($LASTEXITCODE -ne 0){throw 'Configuration helper build failed'} } finally {Pop-Location}
$taskConfig=Get-Content -LiteralPath (Join-Path $Repository 'tests/test-config.example.json') -Raw | ConvertFrom-Json -AsHashtable
$taskConfig.configurations.'local-codex'.execution.settings.executable=Join-Path $taskRoot 'uninstalled-codex.exe'
$taskPath=Join-Path $taskRoot 'config.json'
[IO.File]::WriteAllText($taskPath,($taskConfig|ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
$taskAssessment=Get-SRTestConfiguration -RunnerExe $taskRunner -Helper $taskHelper -Config $taskPath
Push-Location $taskRoot
try {
    $taskRelative=Get-SRTestConfiguration -RunnerExe $taskRunner -Helper $taskHelper -Config 'config.json'
    if($taskRelative.source_digest -cne $taskAssessment.source_digest){throw 'Relative Config did not resolve against the caller PowerShell location'}
} finally { Pop-Location }
if(@($taskAssessment.cases|Where-Object {$_.scenario_id -eq 'local-codex.feedback' -and $_.availability -eq 'available'}).Count -ne 1){throw 'Static configuration availability incorrectly probed the native tool'}
if(@($taskAssessment.cases|Where-Object {$_.scenario_id -eq 'orca-local.pi' -and $_.availability -eq 'unavailable'}).Count -ne 1){throw 'Missing optional agent configuration became available'}
$taskShell=(Get-Process -Id $PID).Path
$taskMissing=Join-Path $taskRoot 'not-configured.json'
$taskMissingOut=Join-Path $taskRoot 'missing-batch'
& $taskShell -NoProfile -File (Join-Path $Repository 'scripts/test.ps1') -Level L3 -Config $taskMissing -RunnerExe $taskRunner -ConfigHelper $taskHelper -OutDir $taskMissingOut *> (Join-Path $taskRoot 'missing.log')
if($LASTEXITCODE -ne 1){throw 'Missing requested L3 configurations were not incomplete'}
$taskSummary=Get-Content -LiteralPath (Join-Path $taskMissingOut 'summary.json') -Raw|ConvertFrom-Json
if($taskSummary.status -ne 'incomplete' -or $taskSummary.executed_case_count -ne 0 -or $taskSummary.results.Count -ne 27 -or $taskSummary.levels.L3.requested.Count -ne 27 -or $taskSummary.levels.L3.executed.Count -ne 0 -or $taskSummary.levels.L3.complete_level_passed){throw 'Missing case coverage was silently reduced or counted as passed'}
if(Test-Path -LiteralPath (Join-Path $taskMissingOut 'framework')){throw 'Unconfigured batch started execution prerequisites'}
$taskList=& $taskShell -NoProfile -File (Join-Path $Repository 'scripts/test.ps1') -Level L3 -Config $taskMissing -RunnerExe $taskRunner -ConfigHelper $taskHelper -ListAvailability
if($LASTEXITCODE -ne 0 -or ($taskList|Out-String|ConvertFrom-Json).status -ne 'availability'){throw 'Missing configuration inventory failed or mixed stdout'}
$taskOld=Join-Path $taskRoot 'old.json'
[IO.File]::WriteAllText($taskOld,'{"schema":"steerrunner-live-config/v1"}')
& $taskShell -NoProfile -File (Join-Path $Repository 'scripts/test.ps1') -Level L3 -Config $taskOld -RunnerExe $taskRunner -ConfigHelper $taskHelper -ListAvailability *> (Join-Path $taskRoot 'old.log')
if($LASTEXITCODE -ne 2){throw 'Legacy test format was silently converted'}
$taskScheduler=Join-Path $taskRoot 'scheduler'
New-Item -ItemType Directory -Path $taskScheduler | Out-Null
$taskWorker=Join-Path $taskRoot 'worker.ps1'
$taskBody=@'
param([string]$Request)
$job=Get-Content -LiteralPath $Request -Raw|ConvertFrom-Json -AsHashtable
if($job.entry.id -eq 'unconfigured'){throw 'Unconfigured case was dispatched'}
$record=$job.record
$record.status='passed';$record.identity=@{runner_sha256='same-build';driver_sha256='same-driver'};$record.resources=@('run:configured')
[IO.File]::WriteAllText($job.result_path,($record|ConvertTo-Json -Depth 30))
'@
[IO.File]::WriteAllText($taskWorker,$taskBody,[Text.UTF8Encoding]::new($false))
$taskExecutions=@(
 @{entry=@{id='unconfigured';level='L3';driver='development-smoke';timeout_minutes=1};case=@{id='default'};repeat_index=1;excluded=$false;unavailable=$true;availability=@{availability='unavailable';reasons=@('missing_configuration')}},
 @{entry=@{id='configured';level='L3';driver='development-smoke';timeout_minutes=1};case=@{id='default'};repeat_index=1;excluded=$false;unavailable=$false}
)
$script:taskRecords=@()
Invoke-SRCaseBatch -Executions $taskExecutions -Output $taskScheduler -MaxParallel 2 -WorkerScript $taskWorker -LeaseDirectory (Join-Path $taskRoot 'locks') -Checkpoint {param($records)$script:taskRecords=@($records)}
if($script:taskRecords.Count -ne 2 -or $script:taskRecords[0].status -ne 'unavailable' -or $script:taskRecords[1].status -ne 'passed'){throw 'Missing configuration blocked its independent configured peer'}
$taskCoverage=[ordered]@{}
Set-SRLevelCoverage -Summary $taskCoverage -Registered @(@{id='configured';level='L3';cases=@(@{id='default'})},@{id='unconfigured';level='L3';cases=@(@{id='default'})}) -Records $script:taskRecords -Levels @('L3')
if($taskCoverage.levels.L3.complete_level_passed){throw 'A subset became complete L3 coverage'}
foreach($taskOutcome in @(
    @{records=@(@{case_id='a';status='failed'},@{case_id='b';status='unavailable'});selected=2;excluded=0;expected='failed'},
    @{records=@(@{case_id='a';status='passed'},@{case_id='b';status='unavailable'});selected=2;excluded=0;expected='incomplete'},
    @{records=@(@{case_id='a';status='excluded'});selected=1;excluded=1;expected='incomplete'},
    @{records=@(@{case_id='a';status='passed'},@{case_id='b';status='queued'});selected=2;excluded=0;expected='incomplete'},
    @{records=@(@{case_id='a';status='passed'},@{case_id='b';status='excluded'});selected=2;excluded=1;expected='passed_with_exclusions'},
    @{records=@();selected=0;excluded=0;expected='passed'}
)) {
    if((Get-SRBatchOutcome -Records $taskOutcome.records -SelectedScenarioCount $taskOutcome.selected -AuthorizedExclusions $taskOutcome.excluded) -cne $taskOutcome.expected){throw 'Batch outcome priority drifted'}
}
$taskChanged=Get-Content -LiteralPath (Join-Path $taskScheduler 'configured-default-r1-request.json') -Raw | ConvertFrom-Json -AsHashtable
$taskChanged.config=$taskPath
$taskChanged['configuration_sha256']='0'*64
$taskChanged.result_path=Join-Path $taskRoot 'changed-snapshot-result.json'
$taskChanged.record.fixture_record=Join-Path $taskRoot 'changed-snapshot-fixture.json'
$taskChangedPath=Join-Path $taskRoot 'changed-snapshot-request.json'
[IO.File]::WriteAllText($taskChangedPath,($taskChanged|ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
& $taskShell -NoProfile -File (Join-Path $Repository 'scripts/run-test-case.ps1') -Request $taskChangedPath *> (Join-Path $taskRoot 'changed-snapshot.log')
$taskChangedResult=Get-Content -LiteralPath $taskChanged.result_path -Raw | ConvertFrom-Json
if($LASTEXITCODE -ne 1 -or $taskChangedResult.error -notmatch 'Frozen batch artifact changed' -or (Test-Path -LiteralPath $taskChanged.record.fixture_record)){throw 'Changed frozen snapshot reached fixture preparation'}
# The fixture must compare what it actually parsed, even after worker preflight.
foreach ($taskIdentity in @('ConfigurationSHA256','HelperSHA256','RequirementsSHA256')) {
    $taskRejected=$false
    try {
        $taskExpected=@{}; $taskExpected[$taskIdentity]='0'*64
        & (Join-Path $Repository 'scripts/new-test-fixture.ps1') -Scenario 'local-codex.feedback' -Config $taskPath -RunnerExe $taskRunner -ConfigHelper $taskHelper @taskExpected | Out-Null
    } catch { $taskRejected=$_.Exception.Message -match 'Frozen batch artifact changed during assessment' }
    if(-not $taskRejected){throw ('Fixture ignored batch identity after parsing: '+$taskIdentity)}
}
$taskStale=$taskAssessment | ConvertTo-Json -Depth 60 | ConvertFrom-Json -AsHashtable
$taskStale.requirements_digest='0'*64
[IO.File]::WriteAllText((Join-Path $taskRoot 'stale-assessment.json'),($taskStale | ConvertTo-Json -Depth 60))
$taskStaleHelper=Join-Path $taskRoot 'stale-helper.ps1'
[IO.File]::WriteAllText($taskStaleHelper,'$input | Out-Null; [IO.File]::ReadAllText((Join-Path $PSScriptRoot ''stale-assessment.json''))')
$taskRejected=$false
try { Get-SRTestConfiguration -RunnerExe $taskRunner -Helper $taskStaleHelper -Config $taskPath | Out-Null } catch { $taskRejected=$_.Exception.Message -match 'case requirements do not match' }
if(-not $taskRejected){throw 'An old helper silently assessed a different case registry'}
# A real entry-point prerequisite failure must preserve the whole selected scope.
$taskPreviousOptIn=[Environment]::GetEnvironmentVariable('STEERRUNNER_ORCA_STATE_PROBE_ROOT')
$taskPrerequisiteOut=Join-Path $taskRoot 'prerequisite-failure'
try {
    $env:STEERRUNNER_ORCA_STATE_PROBE_ROOT='configuration-framework-sentinel'
    & $taskShell -NoProfile -File (Join-Path $Repository 'scripts/test.ps1') -Level L3 -Config $taskPath -RunnerExe $taskRunner -ConfigHelper $taskHelper -OutDir $taskPrerequisiteOut *> (Join-Path $taskRoot 'prerequisite-failure.log')
    $taskPrerequisiteExit=$LASTEXITCODE
} finally { [Environment]::SetEnvironmentVariable('STEERRUNNER_ORCA_STATE_PROBE_ROOT',$taskPreviousOptIn) }
$taskPrerequisite=Get-Content -LiteralPath (Join-Path $taskPrerequisiteOut 'summary.json') -Raw | ConvertFrom-Json
if($taskPrerequisiteExit -ne 1 -or $taskPrerequisite.status -ne 'failed' -or $taskPrerequisite.error -notmatch 'Unexpected live opt-in' -or $taskPrerequisite.results.Count -ne 27 -or $taskPrerequisite.levels.L3.requested.Count -ne 27 -or $taskPrerequisite.levels.L3.executed.Count -ne 0 -or $taskPrerequisite.unavailable_case_count -ne 24 -or $taskPrerequisite.baseline.l2_verified -or $taskPrerequisite.levels.L3.complete_level_passed){throw 'Prerequisite failure lost requested, unavailable or unexecuted case facts'}
if(-not (Test-Path -LiteralPath (Join-Path $taskPrerequisiteOut 'frozen-catalog.json'))){throw 'Batch catalog was not frozen'}
# L2's native preparation is a case dependency, never a global L3 prerequisite.
$taskBoundaryWorker=Join-Path $taskRoot 'boundary-worker.ps1'
$taskBoundaryBody=@'
param([string]$Request)
$job=Get-Content -LiteralPath $Request -Raw | ConvertFrom-Json -AsHashtable
if($job.entry.id -eq 'local-codex.smoke') { & $job.entry.native_worker -Request $Request; exit $LASTEXITCODE }
$record=$job.record
$record.status='passed';$record.identity=@{runner_sha256='same-build';driver_sha256='same-driver'}
[IO.File]::WriteAllText($job.result_path,($record|ConvertTo-Json -Depth 30))
'@
[IO.File]::WriteAllText($taskBoundaryWorker,$taskBoundaryBody,[Text.UTF8Encoding]::new($false))
$taskBoundaryOut=Join-Path $taskRoot 'boundary-isolation'
New-Item -ItemType Directory -Path $taskBoundaryOut | Out-Null
$taskBoundaryExecutions=@(
    @{entry=@{id='local-codex.smoke';level='L2';driver='development-smoke';timeout_minutes=1;native_worker=(Join-Path $Repository 'scripts/run-test-case.ps1')};case=@{id='default'};repeat_index=1;excluded=$false;unavailable=$false},
    @{entry=@{id='configured';level='L3';driver='development-smoke';timeout_minutes=1};case=@{id='default'};repeat_index=1;excluded=$false;unavailable=$false}
)
$script:taskBoundaryRecords=@()
Invoke-SRCaseBatch -Executions $taskBoundaryExecutions -Output $taskBoundaryOut -Config $taskPath -RunnerExe $taskRunner -ConfigHelper $taskHelper -ConfigurationSHA256 $taskAssessment.source_digest -MaxParallel 2 -WorkerScript $taskBoundaryWorker -LeaseDirectory (Join-Path $taskRoot 'locks') -Checkpoint {param($records)$script:taskBoundaryRecords=@($records)}
$taskBoundaryFailed=@($script:taskBoundaryRecords | Where-Object {$_.scenario -eq 'local-codex.smoke'})[0]
if($taskBoundaryFailed.status -ne 'failed' -or $taskBoundaryFailed.error -notmatch 'L2 native Git' -or $taskBoundaryFailed.git_boundary.exit_code -eq 0 -or (Test-Path -LiteralPath $taskBoundaryFailed.fixture_record) -or @($script:taskBoundaryRecords | Where-Object {$_.scenario -eq 'configured' -and $_.status -eq 'passed'}).Count -ne 1){throw 'L2 preparation failure blocked its independent L3 peer or was reclassified as unavailable'}
[IO.File]::WriteAllText((Join-Path $taskRoot 'verified.json'),(@{configuration_contract='passed';missing_batch='incomplete';missing_count=27;independent_dispatch='passed';complete_level_passed=$false}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
Write-Output 'Strict config, static availability, legacy rejection, incomplete scope and independent dispatch passed'
