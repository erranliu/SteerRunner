$ErrorActionPreference='Stop'
$taskUntil=[DateTime]::UtcNow.AddMinutes(60)
do {
 if([DateTime]::UtcNow -gt $taskUntil){throw 'Final native batch completion timeout'}
 $taskReport=Get-Content (Join-Path $PSScriptRoot 'final-latest/summary.json') -Raw|ConvertFrom-Json
 if($taskReport.status -eq 'running'){Start-Sleep -Seconds 2}
} while($taskReport.status -eq 'running')
if($taskReport.status -ne 'passed_with_exclusions' -or $taskReport.executed_case_count -ne 3 -or $taskReport.passed_case_count -ne 3){throw 'Final same-source prerequisites/native cases failed'}
if($taskReport.source_commit -ne (&git rev-parse HEAD).Trim() -or -not [string]::IsNullOrWhiteSpace((&git status --porcelain|Out-String))){throw 'Frozen final source changed'}
& (Join-Path $PSScriptRoot 'probes.ps1') > (Join-Path $PSScriptRoot 'probes-launch.log') 2>&1
exit $LASTEXITCODE
