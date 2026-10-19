$ErrorActionPreference='Stop'
$taskRepo='C:/Project/SteerRunner'
$taskPackage='C:/SteerlineDebug/dev-20261019-154407-be8f3b54'
. ($taskRepo+'/scripts/live-catalog.ps1')
. ($taskRepo+'/scripts/test-configuration.ps1')
. ($taskRepo+'/scripts/test-scheduler.ps1')
$taskSnapshot=Get-Content ($taskPackage+'/evidence/native-source-snapshot.json') -Raw|ConvertFrom-Json
if((& git -C $taskRepo rev-parse HEAD).Trim() -cne $taskSnapshot.test_source_commit -or (& git -C $taskRepo status --porcelain)){throw 'Frozen native test source changed'}
foreach($taskFile in $taskSnapshot.files.PSObject.Properties){if((Get-FileHash (Join-Path $taskRepo $taskFile.Name)).Hash.ToLowerInvariant() -cne $taskFile.Value){throw ('Frozen file changed: '+$taskFile.Name)}}
$taskL1=@(Get-Content ($taskPackage+'/evidence/initialization-native-components-serial.jsonl')|ForEach-Object {$_|ConvertFrom-Json})
$taskPassed=@($taskL1|Where-Object {$_.Action -eq 'pass' -and $_.PSObject.Properties['Test'] -and $_.Test -notmatch '/'})
if($taskPassed.Count -ne 126 -or @($taskL1|Where-Object Action -eq 'fail').Count){throw 'Native component qualification incomplete'}
$taskFramework=Get-Content ($taskPackage+'/evidence/native-framework-02/result.json') -Raw|ConvertFrom-Json
if(@($taskFramework.tests|Where-Object {-not $_.passed}).Count -or @($taskFramework.tests).Count -ne 17 -or $taskFramework.exit_code -ne 0 -or $taskFramework.test_count -ne 17){throw 'Framework qualification incomplete'}
$taskControlled=Get-Content ($taskPackage+'/evidence/native-controlled/result.json') -Raw|ConvertFrom-Json
$taskRunner=$taskPackage+'/bin/steerrunner.exe'
$taskHash=(Get-FileHash $taskRunner).Hash.ToLowerInvariant()
$taskBuild=Get-Content ($taskRunner+'.build.json') -Raw|ConvertFrom-Json
if($taskControlled.exit_code -ne 0 -or $taskControlled.scenario_count -ne 3 -or (Get-FileHash $taskControlled.exe).Hash.ToLowerInvariant() -cne $taskHash -or $taskBuild.sha256.ToLowerInvariant() -cne $taskHash -or $taskBuild.source_commit -cne $taskSnapshot.caller_source_commit -or $taskBuild.source_dirty){throw 'Native caller/Controlled build qualification differs'}
& git -C $taskRepo diff --quiet $taskSnapshot.caller_source_commit -- '*.go'
if($LASTEXITCODE -ne 0){throw 'Caller Go source differs from qualified native implementation'}
$taskConfigHelper=$taskRepo+'/.evidence/initialization-native-final-build/test-config.exe'
$taskAssessment=Get-SRTestConfiguration -RunnerExe $taskRunner -Helper $taskConfigHelper -Config ($taskPackage+'/evidence/native-config.json')
$taskOut=$taskPackage+'/evidence/native-initialization-l3'
New-Item -ItemType Directory -Path $taskOut|Out-Null
$taskConfig=$taskOut+'/frozen-config.json'
[IO.File]::WriteAllText($taskConfig,($taskAssessment.configuration|ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
$taskCatalog=$taskOut+'/frozen-catalog.json'
Copy-Item ($taskRepo+'/tests/catalog.json') $taskCatalog
$taskEntry=(Get-Content $taskCatalog -Raw|ConvertFrom-Json).scenarios|Where-Object id -eq 'orca-remote.initialization-stop'
$taskExecutions=@()
foreach($taskCase in @(Get-SRLiveCases $taskEntry)){
 $taskAvailable=@($taskAssessment.cases|Where-Object {$_.scenario_id -ceq $taskEntry.id -and $_.case_id -ceq $taskCase.id})
 if($taskAvailable.Count -ne 1 -or $taskAvailable[0].availability -ne 'available'){throw ('Native case unavailable: '+$taskCase.id)}
 $taskExecutions+=@{entry=$taskEntry;case=$taskCase;repeat_index=1;excluded=$false;unavailable=$false}
}
$script:taskNativeSummary=@{schema='steerrunner-native-initialization-increment/v1';source_commit=$taskSnapshot.caller_source_commit;source_dirty=$false;test_source_commit=$taskSnapshot.test_source_commit;test_source_dirty=$false;integrated_main=$taskSnapshot.integrated_main;qualification_scope='native rebase increment: affected components/framework/three Controlled scenarios and registered L3 cases';l1_component_count=126;controlled_scenario_count=3;runner_sha256=$taskHash;max_parallel=2;status='running';results=@();complete_l1_passed=$false;complete_controlled_passed=$false;complete_l3_level_passed=$false}
Invoke-SRCaseBatch -Executions $taskExecutions -Output $taskOut -Config $taskConfig -Catalog $taskCatalog -FixtureParent ($taskPackage+'/cases') -MaxParallel 2 -RunnerExe $taskRunner -ConfigHelper $taskConfigHelper -ConfigurationSHA256 (Get-FileHash $taskConfig).Hash.ToLowerInvariant() -RunnerSHA256 $taskHash -HelperSHA256 $taskAssessment.helper_sha256 -RequirementsSHA256 $taskAssessment.requirements_digest -Checkpoint {
 param($Records)
 $script:taskNativeSummary.results=@($Records)
 [IO.File]::WriteAllText(($taskOut+'/summary.json'),($script:taskNativeSummary|ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
}
$script:taskNativeSummary.status=Get-SRBatchOutcome -Records $script:taskNativeSummary.results -SelectedScenarioCount 1 -AuthorizedExclusions 0
$script:taskNativeSummary['passed_case_count']=@($script:taskNativeSummary.results|Where-Object status -eq 'passed').Count
[IO.File]::WriteAllText(($taskOut+'/summary.json'),($script:taskNativeSummary|ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
if($script:taskNativeSummary.status -ne 'passed' -or $script:taskNativeSummary.passed_case_count -ne 2){throw 'Native initialization L3 increment failed'}
Write-Output ('Native initialization L3 passed: '+$taskOut+'/summary.json')
