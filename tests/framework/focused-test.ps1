param([Parameter(Mandatory)][string]$Repository,[Parameter(Mandatory)][string]$Evidence)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $Repository 'scripts/test-prerequisite-contract.ps1')
. (Join-Path $Repository 'scripts/test-configuration.ps1')
. (Join-Path $Repository 'scripts/test-focused.ps1')
$taskRoot=Join-Path $Evidence 'focused'
$null=New-Item -ItemType Directory -Path $taskRoot -ErrorAction Stop
function Put([string]$Path,[string]$Text) { [IO.File]::WriteAllText($Path,$Text,[Text.UTF8Encoding]::new($false)) }
function JSON([string]$Path,$Value) { Put $Path ($Value|ConvertTo-Json -Depth 60) }
$taskShell=(Get-Process -Id $PID).Path
$taskRunner=Join-Path $taskRoot 'runner.exe'
$taskHelper=Join-Path $taskRoot 'helper.exe'
$taskConfig=Join-Path $taskRoot 'config.json'
Put $taskRunner 'synthetic, never executed'
Put $taskHelper 'synthetic, never executed'
Put $taskConfig '{}'
$taskSource=Get-SRSourceIdentity $Repository @('cmd/steerrunner','internal','go.mod','go.sum','scripts/build.ps1','scripts/delivery-contract.ps1')
$taskBuild=@{schema='steerrunner-build/v1';component='runner';platform='windows/amd64';source_commit=$taskSource.source_commit;source_dirty=$true;source_digest=$taskSource.source_digest;sha256=(Get-SRFileDigest $taskRunner)}
JSON ($taskRunner+'.build.json') $taskBuild
$taskChecks=[Collections.Generic.List[string]]::new()

# Exercise the real CLI's rejection path; no model, Git fixture or stage double.
$taskInvalid=@(
    @{name='level';replace=@{Level='L4'}},
    @{name='mixed-level';replace=@{Level=@('L2','L3')}},
    @{name='scenario';replace=@{Scenario=@()}},
    @{name='wildcard';replace=@{Scenario='local-codex.*'}},
    @{name='all';replace=@{Scenario='all'}},
    @{name='unknown';replace=@{Scenario='missing.scenario'}},
    @{name='duplicate';replace=@{Scenario=@('local-codex.smoke','local-codex.smoke')}},
    @{name='config';replace=@{Config=''}},
    @{name='missing-config';replace=@{Config=(Join-Path $taskRoot 'missing.json')}},
    @{name='runner';replace=@{RunnerExe=''}},
    @{name='catalog';replace=@{Catalog=(Join-Path $Repository 'tests/catalog.json')}},
    @{name='availability';replace=@{ListAvailability=$true}},
    @{name='reuse';replace=@{PrerequisitesFrom=$taskRoot}}
)
foreach($taskCase in $taskInvalid) {
    $taskOutput=Join-Path $taskRoot ('invalid-'+$taskCase.name)
    # A child script preserves array parameters instead of flattening -File args.
    $taskArgs=@{Focused=$true;Level='L2';Scenario='local-codex.smoke';Config=$taskConfig;RunnerExe=$taskRunner;ConfigHelper=$taskHelper;OutDir=$taskOutput}
    foreach($taskKey in $taskCase.replace.Keys){$taskArgs[$taskKey]=$taskCase.replace[$taskKey]}
    $taskRequest=Join-Path $taskRoot 'arguments.json';JSON $taskRequest $taskArgs
    $taskLauncher=Join-Path $taskRoot 'invalid.ps1'
    Put $taskLauncher 'param($Entry,$Request); $p=Get-Content -LiteralPath $Request -Raw|ConvertFrom-Json -AsHashtable; & $Entry @p; exit $LASTEXITCODE'
    & $taskShell -NoProfile -File $taskLauncher -Entry (Join-Path $Repository 'scripts/test.ps1') -Request $taskRequest *> (Join-Path $taskRoot ($taskCase.name+'.log'))
    $taskCode=$LASTEXITCODE
    $taskSummary=Get-Content -LiteralPath (Join-Path $taskOutput 'summary.json') -Raw|ConvertFrom-Json
    if($taskCode -ne 2 -or $taskSummary.status -ne 'failed' -or $taskSummary.results.Count -ne 0 -or $taskSummary.prerequisite_execution_counts.native_preflight -ne 0) { throw "Focused invalid selection reached execution: $($taskCase.name)" }
    $taskChecks.Add('reject-'+$taskCase.name)
}
try { Assert-SRFocusedArguments @('L2') @('local-codex.smoke') $taskConfig $taskRunner $false $true $false ''; throw 'Expected explicit OutDir rejection' }
catch { if($_.Exception.Message -notlike 'Focused requires*'){throw} }
$taskChecks.Add('reject-implicit-output')
$taskBuild.source_digest='0'*64;JSON ($taskRunner+'.build.json') $taskBuild
try { Assert-SRFocusedRunner $Repository $taskRunner; throw 'Expected stale source rejection' }
catch { if($_.Exception.Message -notlike '*current production inputs*'){throw} }
$taskBuild.source_digest=$taskSource.source_digest;JSON ($taskRunner+'.build.json') $taskBuild
$taskChecks.Add('reject-stale-runner')

# Process-local doubles exercise the actual entry dispatcher; production has no
# injection switches. Case/scheduler/native implementation is not duplicated.
$taskEntryText=Get-Content -LiteralPath (Join-Path $Repository 'scripts/test.ps1') -Raw
foreach($taskLibrary in @('live-catalog.ps1','test-scheduler.ps1','test-configuration.ps1','test-prerequisite-contract.ps1','test-focused.ps1')) {
    $taskEntryText=$taskEntryText.Replace(". (Join-Path `$PSScriptRoot '$taskLibrary')",'')
}
$taskEntryText=$taskEntryText.Replace("`$taskRepo = [IO.Path]::GetFullPath((Join-Path `$PSScriptRoot '..'))",("`$taskRepo = '"+$Repository.Replace("'","''")+"'"))
$taskEntry=Join-Path $taskRoot 'entry.ps1';Put $taskEntry $taskEntryText
$taskWrapper=Join-Path $taskRoot 'dispatch.ps1'
Put $taskWrapper @'
param($Repository,$Entry,$Output,$Runner,$Helper,$Config,$Mode)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
foreach($library in @('live-catalog.ps1','test-configuration.ps1','test-prerequisite-contract.ps1','test-focused.ps1')){. (Join-Path $Repository ('scripts/'+$library))}
function Mark($Output,$Name){[IO.File]::AppendAllText((Join-Path $Output 'calls.txt'),$Name+"`n")}
function Get-SRTestConfiguration {
    param($RunnerExe,$Helper,$Config)
    @{baseline=@();cases=@(Get-SRCaseRegistry | ForEach-Object {$s=$_;foreach($c in $s.cases){@{scenario_id=$s.id;case_id=$c.id;availability='available'}}});configuration=@{synthetic=$true};runner_exe=$RunnerExe;runner_sha256=(Get-SRFileDigest $RunnerExe);helper_sha256=(Get-SRFileDigest $Helper);requirements_digest='fixture';source_digest='fixture';resource_digest='fixture'}
}
function Invoke-SRL1Core($Root,$Output,$Catalog,$DefaultCatalog){Mark $Output 'L1';@{passed=$true;record=@{level='L1';exit_code=0};stage=@{name='l1-core';kind='test-double'}}}
function Invoke-SRControlledStage($Root,$Output,$Catalog,$DefaultCatalog){Mark $Output 'Controlled';@{passed=$true;record=@{level='Controlled';exit_code=0};stage=@{name='controlled';kind='test-double'}}}
function Invoke-SRNativePreflight($Root,$Output,[switch]$RequireStable){
    Mark $Output 'NativePreflight'
    $driver=Join-Path $Output 'native.test.exe';[IO.File]::WriteAllText($driver,'synthetic driver')
    @{compile_exit=$(if($Mode -eq 'compile-fail'){1}else{0});zero_match_guard=($Mode -ne 'zero-match');duration_seconds=0;driver_path=$driver;driver_sha256=(Get-SRFileDigest $driver);input_finish=@{fingerprint='fixture'}}
}
function Read-SRPrerequisites($Root,$Source){
    @{runner_exe=$Runner;manifest_path=($Runner+'.build.json');manifest_sha256=(Get-SRFileDigest ($Runner+'.build.json'));manifest=@{stages=@{'l1-core'=@{record=@{level='L1';exit_code=0};finished_at='fixture';duration_seconds=0};controlled=@{record=@{level='Controlled';exit_code=0};finished_at='fixture';duration_seconds=0}}}}
}
function Get-SRPrerequisiteSnapshot($Root,[switch]$Native){@{supported=$true;fingerprint='fixture'}}
function Invoke-SRCaseBatch {
    param($Executions,$Output,$Checkpoint,$NativeDriver='',$NativeDriverSHA256='')
    Mark $Output 'Cases'
    if($Mode -ne 'default' -and (-not $NativeDriver -or (Get-SRFileDigest $NativeDriver) -cne $NativeDriverSHA256)){throw 'Guarded driver was not passed to the case batch'}
    $records=@()
    if($Mode -ne 'empty'){
        foreach($e in $Executions){$records+=@{level=$e.entry.level;scenario=$e.entry.id;case_id=$e.case.id;repeat_index=$e.repeat_index;status=$(if($Mode -eq 'cleanup-fail'){'failed'}else{'passed'});timings=@{preparation_seconds=0;native_and_cleanup_seconds=0}}}
    }
    & $Checkpoint $records
    if($Mode -eq 'changed-driver'){[IO.File]::AppendAllText($NativeDriver,'changed')}
}
$argsMap=@{Level='L2';Scenario='local-codex.smoke';Config=$Config;RunnerExe=$Runner;ConfigHelper=$Helper;OutDir=$Output}
if($Mode -eq 'reuse'){$argsMap['PrerequisitesFrom']=Split-Path -Parent $Config}
elseif($Mode -ne 'default'){$argsMap['Focused']=$true}
if($Mode -eq 'L3'){$argsMap.Level='L3';$argsMap.Scenario='orca-local.codex'}
& $Entry @argsMap
exit $LASTEXITCODE
'@
foreach($taskMode in @('L2','L3','default','reuse','compile-fail','zero-match','cleanup-fail','empty','changed-driver')) {
    $taskOutput=Join-Path $taskRoot ('dispatch-'+$taskMode)
    & $taskShell -NoProfile -File $taskWrapper -Repository $Repository -Entry $taskEntry -Output $taskOutput -Runner $taskRunner -Helper $taskHelper -Config $taskConfig -Mode $taskMode *> (Join-Path $taskRoot ('dispatch-'+$taskMode+'.log'))
    $taskCode=$LASTEXITCODE
    $taskSummary=Get-Content -LiteralPath (Join-Path $taskOutput 'summary.json') -Raw|ConvertFrom-Json
    $taskCalls=(Get-Content -LiteralPath (Join-Path $taskOutput 'calls.txt')) -join ','
    $taskSuccess=$taskMode -in @('L2','L3','default','reuse')
    $taskExpectedCalls=if($taskMode -eq 'default'){'L1,NativePreflight,Controlled,Cases'}elseif($taskMode -in @('compile-fail','zero-match')){'NativePreflight'}else{'NativePreflight,Cases'}
    if($taskCalls -cne $taskExpectedCalls -or (($taskCode -eq 0) -ne $taskSuccess) -or (($taskSummary.status -eq 'passed') -ne $taskSuccess)){throw "Entry dispatch/result mismatch: $taskMode ($taskCalls); inspect dispatch log"}
    if($taskMode -notin @('default','reuse')) {
        if($taskSummary.validation_scope -ne 'focused' -or $taskSummary.prerequisite_execution_counts.l1_core -ne 0 -or $taskSummary.prerequisite_execution_counts.controlled -ne 0 -or $taskSummary.baseline.l2_verified -or @($taskSummary.levels.PSObject.Properties | Where-Object {$_.Value.complete_level_passed}).Count){throw "Focused gained complete qualification: $taskMode"}
    } elseif(-not $taskSummary.levels.L2.complete_level_passed -or -not $taskSummary.baseline.l2_verified -or $taskSummary.validation_scope -ne 'complete'){throw "Default/reuse qualification regressed: $taskMode"}
    $taskChecks.Add('dispatch-'+$taskMode)
}

# Even every registered case passing cannot turn focused into qualification.
$taskExecution=@{entry=@{id='one';level='L2'};case=@{id='default'};repeat_index=1}
$taskRecords=@(@{level='L2';scenario='one';case_id='default';repeat_index=1;status='passed'})
$taskSummary=@{validation_scope='focused';status='passed';baseline=@{l2_verified=$true;l3_verified=$true}}
Set-SRLevelCoverage $taskSummary @(@{id='one';level='L2';cases=@(@{id='default'})}) $taskRecords @('L2')
Set-SRQualificationCoverage $taskSummary @($taskExecution) $taskRecords
if($taskSummary.levels.L2.complete_level_passed -or $taskSummary.baseline.l2_verified -or $taskSummary.baseline.l3_verified){throw 'Focused qualification was resurrected'}
$taskChecks.Add('all-cases-still-focused')

# The actual worker accepts a verified L3 driver, then rejects changed frozen
# config before any fixture/native effects. Partial/L4 driver guards stay in
# the existing prerequisite contract suite.
$taskJob=@{entry=@{id='orca-local.codex';level='L3';driver='development'};record=@{status='queued';started_at=$null;finished_at=$null};result_path=(Join-Path $taskRoot 'l3-worker-result.json');native_driver=$taskRunner;native_driver_sha256=(Get-SRFileDigest $taskRunner);config=$taskConfig;configuration_sha256=('0'*64)}
$taskRequest=Join-Path $taskRoot 'l3-worker.json';JSON $taskRequest $taskJob
& $taskShell -NoProfile -File (Join-Path $Repository 'scripts/run-test-case.ps1') -Request $taskRequest *> (Join-Path $taskRoot 'l3-worker.log')
$taskResult=Get-Content -LiteralPath $taskJob.result_path -Raw|ConvertFrom-Json
if($LASTEXITCODE -ne 1 -or $taskResult.error -notlike 'Frozen batch artifact changed: configuration_sha256*'){throw 'Verified L3 driver did not reach the frozen configuration check'}
$taskChecks.Add('l3-driver-before-fixture')
JSON (Join-Path $taskRoot 'verified.json') @{checks=$taskChecks.ToArray();check_count=$taskChecks.Count;live_execution=$false;native_qualification=$false}
Write-Host "Focused contract checks passed: $($taskChecks.Count)"
