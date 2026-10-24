param([Parameter(Mandatory)][string]$Repository,[Parameter(Mandatory)][string]$Evidence,[string]$Verifier='')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
if (-not $Verifier) { $Verifier=Join-Path $Repository 'scripts/verify-progress-parallel.ps1' }
$taskShell=(Get-Process -Id $PID).Path
$taskIDs=@('local-codex.progress-file','orca-local.progress-file','orca-remote.progress-file','local-codex.progress-multirun','local-codex.progress-recovery')
$taskStart=[DateTimeOffset]::UtcNow.AddSeconds(-125).ToString('o')
$taskEnd=[DateTimeOffset]::UtcNow.ToString('o')
$taskPassed=@()
foreach($taskMode in @('valid','missing-field','resource-collision','no-passing-cases','zero-selection')) {
    $taskRoot=Join-Path $Evidence $taskMode
    New-Item -ItemType Directory -Path $taskRoot -ErrorAction Stop|Out-Null
    $taskCases=@()
    foreach($taskID in $taskIDs) {
        $taskCaseDir=Join-Path $taskRoot $taskID
        New-Item -ItemType Directory -Path $taskCaseDir|Out-Null
        $taskWindow=@{contract='steerrunner-progress-observation-window/v2';start=$taskStart;end=$taskEnd;run_ids=@($taskID);collection_healthy_at_end=$true;collection_gap_budget_seconds=20;collection_gap_samples=0}
        if($taskMode -eq 'missing-field' -and $taskID -eq $taskIDs[0]) { $taskWindow.Remove('collection_gap_samples') }
        [IO.File]::WriteAllText((Join-Path $taskCaseDir 'progress-001-observation-window.json'),($taskWindow|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
        $taskResource=if($taskMode -eq 'resource-collision'){'shared-resource'}else{$taskID+'-resource'}
        $taskStatus=if($taskMode -eq 'no-passing-cases'){'queued'}else{'passed'}
        if($taskMode -ne 'zero-selection') { $taskCases+=@{scenario=$taskID;status=$taskStatus;evidence=$taskCaseDir;resources=@($taskResource);identity=@{runner_sha256=('a'*64);driver_sha256=('b'*64);source_commit=('c'*40)}} }
    }
    [IO.File]::WriteAllText((Join-Path $taskRoot 'summary.json'),(@{status='passed';max_parallel=5;results=$taskCases}|ConvertTo-Json -Depth 10),[Text.UTF8Encoding]::new($false))
    & $taskShell -NoProfile -File $Verifier -Report $taskRoot *> (Join-Path $taskRoot 'verification.log')
    $taskCode=$LASTEXITCODE
    $taskResultPath=Join-Path $taskRoot 'progress-parallel-verification.json'
    if(-not (Test-Path -LiteralPath $taskResultPath)) { throw "Verifier lost its failure artifact: $taskMode" }
    $taskResult=Get-Content -LiteralPath $taskResultPath -Raw|ConvertFrom-Json
    if($taskMode -eq 'valid') {
        if($taskCode -ne 0 -or -not $taskResult.passed -or $taskResult.native_observation_overlaps.Count -ne 10) { throw 'Valid independent overlap fixture was rejected' }
    } else {
        if($taskCode -ne 1 -or $taskResult.passed -or $taskResult.issues.Count -eq 0) { throw "Invalid overlap evidence was accepted: $taskMode" }
        if($taskMode -eq 'missing-field' -and -not ($taskResult.issues -match 'bounded collection recovery')) { throw 'Missing observation fields lost their diagnostic' }
        if($taskMode -eq 'resource-collision' -and $taskResult.resource_conflicts.Count -eq 0) { throw 'Resource collision disappeared' }
    }
    $taskPassed+=$taskMode
}
[IO.File]::WriteAllText((Join-Path $Evidence 'result.json'),(@{controlled=$true;live_execution=$false;passed=$taskPassed}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
Write-Output 'Progress parallel verifier preserves missing-field failures and rejects resource collisions.'
