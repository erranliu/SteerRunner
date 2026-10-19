$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskRoot=$PSScriptRoot
$taskSource=(&git rev-parse HEAD).Trim()
if($taskSource -ne '212ebee3abeae798c05fe53ede19f8b2a5e0f0f3'){throw 'Unexpected final production source'}
$taskControlled=Join-Path $taskRoot 'controlled-r5-25'
$taskUntil=[DateTime]::UtcNow.AddMinutes(26)
while(-not (Test-Path (Join-Path $taskControlled 'result.json'))){if([DateTime]::UtcNow -gt $taskUntil){throw 'Controlled25 result unavailable'};Start-Sleep -Seconds 2}
$taskControlResult=Get-Content (Join-Path $taskControlled 'result.json') -Raw|ConvertFrom-Json
if($taskControlResult.exit_code -ne 0 -or $taskControlResult.scenario_count -ne 16 -or (Select-String (Join-Path $taskControlled 'integration.log') -Pattern '--- SKIP:' -Quiet)){throw 'Full frozen Controlled16 not qualified'}
$taskPrior=Get-Content (Join-Path $taskRoot 'native-r5/summary.json') -Raw|ConvertFrom-Json
$taskL1=@($taskPrior.results|Where-Object {$_.level -eq 'L1'})
if($taskL1.Count -ne 1 -or $taskL1[0].exit_code -ne 0 -or $taskPrior.source_commit -ne $taskSource -or -not [string]::IsNullOrWhiteSpace((&git status --porcelain|Out-String))){throw 'Same-source full prerequisites not valid'}
$taskL1Map=@{};foreach($taskProperty in $taskL1[0].PSObject.Properties){$taskL1Map[$taskProperty.Name]=$taskProperty.Value}
$taskPrerequisites=@($taskL1Map,@{level='Controlled';exit_code=0;scenario_count=16;evidence=$taskControlled;source_commit=$taskSource;aggregate_timeout_minutes=25;frozen_binaries_from=(Join-Path $taskRoot 'native-r5/controlled')})
. ./scripts/live-catalog.ps1
. ./scripts/test-scheduler.ps1
$taskOut=Join-Path $taskRoot 'native-final'
if(Test-Path $taskOut){throw 'Fresh final native output required'}
New-Item -ItemType Directory -Path $taskOut|Out-Null
$taskCatalog=Get-Content ./tests/catalog.json -Raw|ConvertFrom-Json
$taskEntries=@($taskCatalog.scenarios|Where-Object {$_.id -in @('orca-remote.service-restart','orca-remote.stop-revocation')})
$taskCases=@();foreach($taskEntry in $taskEntries){foreach($taskCase in @(Get-SRLiveCases $taskEntry)){$taskCases+=@{entry=$taskEntry;case=$taskCase;repeat_index=1;excluded=(Test-SRAdminCase $taskEntry.id $taskCase.id)}}}
$taskReport=[ordered]@{schema='steerrunner-test-report/v1';source_commit=$taskSource;source_dirty=$false;started_at=[DateTime]::UtcNow.ToString('o');status='running';scope='admin-free-subset';selected_scenarios=@($taskEntries|ForEach-Object {$_.id});planned_case_count=7;authorized_exclusion_count=4;max_parallel=2;results=$taskPrerequisites;original_prerequisite_report=(Join-Path $taskRoot 'native-r5/summary.json');controlled_aggregate_rerun=$taskControlled;executed_case_count=0;passed_case_count=0}
function Save-FinalControlReport {$taskReport|ConvertTo-Json -Depth 24|Set-Content -Encoding utf8NoBOM (Join-Path $taskOut 'summary.json')}
Save-FinalControlReport
try {
 & ./scripts/stability-preflight.ps1 -Config (Join-Path $taskRoot 'live-config-r5.json')
 Invoke-SRCaseBatch -Executions $taskCases -Output $taskOut -Config (Join-Path $taskRoot 'live-config-r5.json') -Catalog ((Resolve-Path ./tests/catalog.json).ProviderPath) -FixtureParent 'C:\srt' -MaxParallel 2 -Checkpoint {param($Records) $taskReport.results=@($taskPrerequisites)+@($Records);Save-FinalControlReport}
 $taskReport.executed_case_count=@($taskReport.results|Where-Object {$_.ContainsKey('case_id') -and $_.status -ne 'excluded'}).Count
 $taskReport.passed_case_count=@($taskReport.results|Where-Object {$_.ContainsKey('case_id') -and $_.status -eq 'passed'}).Count
 $taskReport.status=if($taskReport.executed_case_count -eq 3 -and $taskReport.passed_case_count -eq 3){'passed_with_exclusions'}else{'failed'}
} catch {$taskReport.status='failed';$taskReport['error']=$_.Exception.Message;Write-Host $_.Exception.Message} finally {$taskReport['finished_at']=[DateTime]::UtcNow.ToString('o');Save-FinalControlReport}
if($taskReport.status -ne 'passed_with_exclusions'){exit 1}
