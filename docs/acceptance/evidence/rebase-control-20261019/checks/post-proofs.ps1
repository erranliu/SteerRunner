$ErrorActionPreference='Stop'
$taskUntil=[DateTime]::UtcNow.AddMinutes(65)
do {
 if([DateTime]::UtcNow -gt $taskUntil){throw 'Final evidence completion timeout'}
 $taskReady=Test-Path (Join-Path $PSScriptRoot 'probes.json')
 if($taskReady){
  $taskProbes=@(Get-Content (Join-Path $PSScriptRoot 'probes.json') -Raw|ConvertFrom-Json)
  if(@($taskProbes|Where-Object {$_.exit_code -ne 0}).Count){throw 'Native stop probe failed'}
  $taskReady=$taskProbes.Count -eq 2
 }
 if(-not $taskReady){Start-Sleep -Seconds 2}
} while(-not $taskReady)
$taskSummary=Get-Content (Join-Path $PSScriptRoot 'final-latest/summary.json') -Raw|ConvertFrom-Json
if($taskSummary.status -ne 'passed_with_exclusions' -or $taskSummary.passed_case_count -ne 3){throw 'Final batch did not pass'}
& python (Join-Path $PSScriptRoot 'verify-control.py')
if($LASTEXITCODE -ne 0){throw 'Fixed original control verification failed'}
$taskEvidence=@($taskSummary.results|Where-Object {$_.level -eq 'L3' -and $_.status -eq 'passed'}|ForEach-Object {$_.evidence})
& go run ./.evidence/rebase-control-20261019/export-exit @taskEvidence
if($LASTEXITCODE -ne 0){throw 'Original signed artifact export failed'}
$taskEvidence+=@($taskProbes|ForEach-Object {$_.evidence})
& (Join-Path $PSScriptRoot 'verify-proof.exe') @taskEvidence
if($LASTEXITCODE -ne 0){throw 'Original native signature verification failed'}
Write-Output 'All five original signatures and fixed control facts verified'
