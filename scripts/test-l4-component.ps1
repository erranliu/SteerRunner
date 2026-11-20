param(
    [Parameter(Mandatory=$true)][string]$OutDir,
    [Parameter(Mandatory=$true)][ValidateSet('./internal/application')][string]$Package,
    [Parameter(Mandatory=$true)][ValidateSet('TestRunNamePrefixFreeze','TestDisplayRunsIgnoreRolledBackChanges')][string]$Test,
    [string]$PrerequisitesFrom='',
    [ValidateRange(1,60)][int]$TimeoutMinutes=15
)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'test-prerequisite-contract.ps1')
$taskRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskOut=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'L4 component evidence directory must be new' }
$null=New-Item -ItemType Directory -Path $taskOut
$taskReport=@{validation_scope='l4-component';status='failed';package=$Package;test=$Test;tags=@('l4');started_at=[DateTime]::UtcNow.ToString('o')}
Push-Location $taskRoot
try {
    Assert-SRNoLiveOptIn
    $taskStart=Get-SRPrerequisiteSnapshot $taskRoot
    if (-not $taskStart.supported) { throw 'L4 component source identity is unsupported' }
    if ($PrerequisitesFrom) {
        $taskProof=Read-SRPrerequisites $taskRoot $PrerequisitesFrom
        if ($taskProof.snapshot.fingerprint -cne $taskStart.fingerprint) { throw 'L4 component does not match its batch prerequisites' }
    }
    & go test -tags=l4 $Package -run ('^'+[regex]::Escape($Test)+'$') -count=1 "-timeout=$($TimeoutMinutes)m" -json 1> (Join-Path $taskOut 'go-test.jsonl') 2> (Join-Path $taskOut 'stderr.log')
    $taskReport['exit_code']=$LASTEXITCODE
    $taskEvents=@(Get-Content -LiteralPath (Join-Path $taskOut 'go-test.jsonl') | ForEach-Object {$_|ConvertFrom-Json})
    $taskPassed=@($taskEvents | Where-Object {$_.Action -eq 'pass' -and $_.PSObject.Properties['Test'] -and $_.Test -cnotmatch '/'})
    $taskRan=@($taskEvents | Where-Object {$_.Action -eq 'run' -and $_.PSObject.Properties['Test'] -and $_.Test -cnotmatch '/'})
    $taskReport['passed_count']=$taskPassed.Count
    if ($taskReport.exit_code -ne 0 -or $taskPassed.Count -ne 1 -or $taskPassed[0].Test -cne $Test -or $taskRan.Count -ne 1 -or $taskRan[0].Test -cne $Test -or @($taskEvents | Where-Object {$_.Action -in @('fail','skip')}).Count) { throw 'L4 component failed, skipped or did not execute the exact test once' }
    $taskFinish=Get-SRPrerequisiteSnapshot $taskRoot
    if (-not $taskFinish.supported -or $taskStart.fingerprint -cne $taskFinish.fingerprint) { throw 'L4 component inputs changed during execution' }
    Write-SRPrerequisiteJSON (Join-Path $taskOut 'source-identity.json') @{source_commit=$taskStart.source_commit;source_dirty=$taskStart.source_dirty;input_start=$taskStart;input_finish=$taskFinish;package=$Package;test=$Test;tags=@('l4')}
    $taskReport.status='passed'
} catch {
    $taskReport['error']=$_.Exception.Message
    throw
} finally {
    Pop-Location
    $taskReport['finished_at']=[DateTime]::UtcNow.ToString('o')
    Write-SRPrerequisiteJSON (Join-Path $taskOut 'result.json') $taskReport
}
