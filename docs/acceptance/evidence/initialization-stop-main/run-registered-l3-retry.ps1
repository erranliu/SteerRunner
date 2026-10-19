$ErrorActionPreference='Stop'
$taskRepo='C:/Project/SteerRunner'
$taskPackage='C:/SteerlineDebug/dev-20261019-130825-dc5dc4a5-dirty'
. ($taskRepo+'/scripts/live-catalog.ps1')
. ($taskRepo+'/scripts/test-configuration.ps1')
. ($taskRepo+'/scripts/test-scheduler.ps1')
$taskPriorPath=$taskPackage+'/evidence/l3-initialization-stop/summary.json'
$taskPrior=Get-Content $taskPriorPath -Raw|ConvertFrom-Json
$taskRunner=$taskPackage+'/bin/steerrunner.exe'
$taskRunnerHash=(Get-FileHash $taskRunner).Hash.ToLowerInvariant()
$taskBuild=Get-Content ($taskRunner+'.build.json') -Raw|ConvertFrom-Json
$taskCommit=(& git -C $taskRepo rev-parse HEAD).Trim()
if($taskPrior.status -ne 'failed' -or $taskPrior.source_commit -cne $taskCommit -or $taskPrior.source_dirty -or (& git -C $taskRepo status --porcelain) -or $taskRunnerHash -cne $taskPrior.runner_sha256 -or $taskRunnerHash -cne $taskBuild.sha256.ToLowerInvariant() -or $taskBuild.source_commit -cne $taskCommit -or $taskBuild.source_dirty){throw 'Retry source/build differs from original qualified prerequisites'}
$taskL1=@($taskPrior.results|Where-Object level -eq 'L1')
$taskControlled=@($taskPrior.results|Where-Object level -eq 'Controlled')
$taskFailed=@($taskPrior.results|Where-Object level -eq 'L3')
if($taskL1.Count -ne 1 -or $taskL1[0].exit_code -ne 0 -or $taskL1[0].test_count -ne 394 -or -not $taskL1[0].terminal_extensions_passed -or -not $taskL1[0].native_zero_match_guard -or $taskControlled.Count -ne 1 -or $taskControlled[0].exit_code -ne 0 -or $taskControlled[0].scenario_count -ne 16 -or $taskFailed.Count -ne 2){throw 'Retry prerequisites incomplete'}
foreach($taskFailure in $taskFailed){if($taskFailure.status -ne 'failed' -or $taskFailure.manifest -or $taskFailure.error -notmatch 'steerrunner.exe.build.json'){throw 'Retry is not the metadata-only pre-Run preparation failure'}}
if((Get-FileHash ($taskControlled[0].evidence+'/steerrunner.exe')).Hash.ToLowerInvariant() -cne $taskRunnerHash){throw 'Controlled build differs'}
$taskOut=$taskPackage+'/evidence/l3-initialization-stop-retry-02'
New-Item -ItemType Directory -Path $taskOut|Out-Null
$taskConfig=$taskPackage+'/evidence/initialization-config.json'
$taskConfigHelper=$taskRepo+'/.evidence/initialization-main-build/test-config.exe'
$taskAssessment=Get-SRTestConfiguration -RunnerExe $taskRunner -Helper $taskConfigHelper -Config $taskConfig
if($taskAssessment.source_digest -cne $taskPrior.source_config_digest -or $taskAssessment.helper_sha256 -cne $taskPrior.helper_sha256 -or $taskAssessment.requirements_digest -cne $taskPrior.requirements_sha256){throw 'Retry configuration/helper/registry changed'}
$taskConfig=$taskPackage+'/evidence/l3-initialization-stop/frozen-config.json'
if((Get-FileHash $taskConfig).Hash.ToLowerInvariant() -cne $taskPrior.configuration_sha256){throw 'Frozen configuration bytes changed'}
$taskCatalog=$taskOut+'/frozen-catalog.json'
Copy-Item ($taskPackage+'/evidence/l3-initialization-stop/frozen-catalog.json') $taskCatalog
$taskEntry=(Get-Content $taskCatalog -Raw|ConvertFrom-Json).scenarios|Where-Object id -eq 'orca-remote.initialization-stop'
$taskExecutions=@()
foreach($taskCase in @(Get-SRLiveCases $taskEntry)){
 $taskAvailable=@($taskAssessment.cases|Where-Object {$_.scenario_id -ceq $taskEntry.id -and $_.case_id -ceq $taskCase.id})
 if($taskAvailable.Count -ne 1 -or $taskAvailable[0].availability -ne 'available'){throw 'Registered case unavailable'}
 $taskExecutions+=@{entry=$taskEntry;case=$taskCase;repeat_index=1;excluded=$false;unavailable=$false}
}
$script:taskRetrySummary=@{schema='steerrunner-registered-l3-retry/v1';source_commit=$taskCommit;source_dirty=$false;status='running';scope='registered L3 retry after restoring unchanged build receipt; exact newly qualified prerequisites retained';prerequisites=$taskPriorPath;prerequisites_sha256=(Get-FileHash $taskPriorPath).Hash.ToLowerInvariant();runner_sha256=$taskRunnerHash;runtime_helper_sha256='52a2261351290f49366a331119d2f72afa36b60d7ef7aff12063206053ae573a';max_parallel=2;complete_l3_level_passed=$false;results=@()}
Invoke-SRCaseBatch -Executions $taskExecutions -Output $taskOut -Config $taskConfig -Catalog $taskCatalog -FixtureParent ($taskPackage+'/cases') -MaxParallel 2 -RunnerExe $taskRunner -ConfigHelper $taskConfigHelper -ConfigurationSHA256 $taskPrior.configuration_sha256 -RunnerSHA256 $taskRunnerHash -HelperSHA256 $taskAssessment.helper_sha256 -RequirementsSHA256 $taskAssessment.requirements_digest -Checkpoint {
 param($Records)
 $script:taskRetrySummary.results=@($Records)
 [IO.File]::WriteAllText(($taskOut+'/summary.json'),($script:taskRetrySummary|ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
}
$script:taskRetrySummary.status=Get-SRBatchOutcome -Records $script:taskRetrySummary.results -SelectedScenarioCount 1 -AuthorizedExclusions 0
$script:taskRetrySummary['passed_case_count']=@($script:taskRetrySummary.results|Where-Object status -eq 'passed').Count
[IO.File]::WriteAllText(($taskOut+'/summary.json'),($script:taskRetrySummary|ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
if($script:taskRetrySummary.status -ne 'passed' -or $script:taskRetrySummary.passed_case_count -ne 2){throw 'Registered L3 retry failed'}
Write-Output ('Registered L3 retry passed: '+$taskOut+'/summary.json')
