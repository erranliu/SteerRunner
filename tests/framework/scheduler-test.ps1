param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
. (Join-Path $Repository 'scripts/test-scheduler.ps1')
# Independent batches on this user/host coordinate through OS byte-range locks.
# The operating system releases a lease if its coordinator exits unexpectedly.
$taskLeaseDirectory=[IO.Path]::GetFullPath((Join-Path $Evidence 'scheduler-lock'))
$taskSharedLease=Open-SRTestLease $false -Directory $taskLeaseDirectory
if($null -eq $taskSharedLease){throw 'No shared scheduler lease available for fixture'}
$taskExclusiveLease=Open-SRTestLease $true -Directory $taskLeaseDirectory
if($null -ne $taskExclusiveLease){$taskExclusiveLease.Dispose();$taskSharedLease.Dispose();throw 'Exclusive lease overlapped shared lease'}
$taskSharedLease.Dispose()
$taskRoot=Join-Path $Evidence 'scheduler'
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskWorker=Join-Path $taskRoot 'worker.ps1'
$taskSource=@'
param([string]$Request)
$job=Get-Content -LiteralPath $Request -Raw | ConvertFrom-Json
if($job.entry.id -eq 'missing-result'){exit 7}
$record=$job.record | ConvertTo-Json -Depth 20 | ConvertFrom-Json -AsHashtable
$record.started_at=[DateTime]::UtcNow.ToString('o')
$record.status='running'
[IO.File]::WriteAllText($job.result_path,($record|ConvertTo-Json -Depth 20))
Start-Sleep -Milliseconds 1800
$record.finished_at=[DateTime]::UtcNow.ToString('o')
$record.status='passed'
$record.identity=@{runner_sha256='runner';fixture_sha256='driver'}
if($job.entry.id -eq 'changed-build'){$record.identity.runner_sha256='changed'}
$record.resources=@('run:'+$job.entry.id)
if($job.entry.id -like 'collision-*'){$record.resources=@('run:shared-original')}
if($job.entry.id -like 'session-*'){$record.resources=@(('run:'+$job.entry.id),'session:shared-session')}
if($job.entry.id -like 'workspace-*'){$record.resources=@(('run:'+$job.entry.id),'workspace:shared-workspace')}
[IO.File]::WriteAllText($job.result_path,($record|ConvertTo-Json -Depth 20))
'@
[IO.File]::WriteAllText($taskWorker,$taskSource,[Text.UTF8Encoding]::new($false))
function New-SchedulerCase([string]$ID,[string]$Level='L3') {
    return @{entry=@{id=$ID;level=$Level;driver='controlled-integration';timeout_minutes=1};case=@{id='default'};repeat_index=1;excluded=$false}
}
$taskExecutions=@((New-SchedulerCase 'first'),(New-SchedulerCase 'second'),(New-SchedulerCase 'exclusive' 'L4'),(New-SchedulerCase 'last'))
$script:taskSchedulerResults=@()
Invoke-SRCaseBatch -Executions $taskExecutions -Output $taskRoot -MaxParallel 2 -WorkerScript $taskWorker -LeaseDirectory $taskLeaseDirectory -Checkpoint {param($Records) $script:taskSchedulerResults=@($Records)}
if($script:taskSchedulerResults.Count -ne 4 -or @($script:taskSchedulerResults | Where-Object {$_.status -ne 'passed'}).Count){throw 'Scheduler lost or failed a case'}
$taskRecords=$script:taskSchedulerResults
$taskOverlap={param($Left,$Right) ([DateTime]$Left.started_at -lt [DateTime]$Right.finished_at -and [DateTime]$Right.started_at -lt [DateTime]$Left.finished_at)}
if(-not (& $taskOverlap $taskRecords[0] $taskRecords[1])){throw 'Independent L3 workers did not overlap'}
foreach($taskRecord in @($taskRecords[0],$taskRecords[1],$taskRecords[3])){if(& $taskOverlap $taskRecord $taskRecords[2]){throw 'Exclusive L4 worker overlapped another case'}}
$taskFailures=Join-Path $taskRoot 'failures';New-Item -ItemType Directory -Path $taskFailures | Out-Null
Invoke-SRCaseBatch -Executions @((New-SchedulerCase 'good'),(New-SchedulerCase 'missing-result'),(New-SchedulerCase 'changed-build')) -Output $taskFailures -MaxParallel 2 -WorkerScript $taskWorker -LeaseDirectory $taskLeaseDirectory -Checkpoint {param($Records) $script:taskSchedulerResults=@($Records)}
if(@($script:taskSchedulerResults | Where-Object {$_.status -eq 'failed'}).Count -ne 2 -or @($script:taskSchedulerResults | Where-Object {$_.status -eq 'passed'}).Count -ne 1){throw 'Worker loss or changed build was accepted'}
$taskCollisions=Join-Path $taskRoot 'collisions';New-Item -ItemType Directory -Path $taskCollisions | Out-Null
Invoke-SRCaseBatch -Executions @((New-SchedulerCase 'collision-a'),(New-SchedulerCase 'collision-b')) -Output $taskCollisions -MaxParallel 2 -WorkerScript $taskWorker -LeaseDirectory $taskLeaseDirectory -Checkpoint {param($Records) $script:taskSchedulerResults=@($Records)}
if(@($script:taskSchedulerResults | Where-Object {$_.status -eq 'failed' -and $_.error -like '*resources collide*'}).Count -ne 1){throw 'Shared original Run identity was accepted across cases'}
foreach($taskKind in @('session','workspace')){
    $taskCollisionRoot=Join-Path $taskRoot $taskKind;New-Item -ItemType Directory -Path $taskCollisionRoot | Out-Null
    Invoke-SRCaseBatch -Executions @((New-SchedulerCase ($taskKind+'-a')),(New-SchedulerCase ($taskKind+'-b'))) -Output $taskCollisionRoot -MaxParallel 2 -WorkerScript $taskWorker -LeaseDirectory $taskLeaseDirectory -Checkpoint {param($Records) $script:taskSchedulerResults=@($Records)}
    if(@($script:taskSchedulerResults | Where-Object {$_.status -eq 'failed' -and $_.error -like '*resources collide*'}).Count -ne 1){throw "Shared native $taskKind was accepted across cases"}
}
[IO.File]::WriteAllText((Join-Path $taskRoot 'verified.json'),(@{parallel_case_count=2;exclusive_case_count=1;worker_loss_rejected=$true;changed_build_rejected=$true;resource_collision_rejected=$true;records=$taskRecords}|ConvertTo-Json -Depth 30))
Write-Output 'Bounded L3 overlap, exclusive L4, worker loss and frozen build checks passed'
