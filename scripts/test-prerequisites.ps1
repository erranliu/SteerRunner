param(
    [ValidateSet('L1','Controlled','All')][string]$Stage='All',
    [string]$OutDir=''
)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'test-prerequisite-contract.ps1')
$taskRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if(-not $OutDir){$OutDir=Join-Path $taskRoot ('.evidence/prerequisites-'+[Guid]::NewGuid().ToString('N').Substring(0,10))}
$taskOut=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if(Test-Path -LiteralPath $taskOut){throw 'Prerequisite output directory must be new'}
$null=New-Item -ItemType Directory -Path $taskOut
$taskReport=@{schema='steerrunner-prerequisite-execution/v1';requested_stage=$Stage;status='running';started_at=[DateTime]::UtcNow.ToString('o');stages=@()}
$taskExit=1
Push-Location $taskRoot
try {
    Assert-SRNoLiveOptIn
    $taskCatalog=Get-Content -LiteralPath (Join-Path $taskRoot 'tests/catalog.json') -Raw | ConvertFrom-Json
    if($Stage -in @('L1','All')) {
        $taskCore=Invoke-SRL1Core $taskRoot $taskOut $taskCatalog
        $taskReport.stages+=@($taskCore.stage)
        $taskPreflight=Invoke-SRNativePreflight $taskRoot $taskOut
        $taskReport.stages+=@($taskPreflight)
        if(-not $taskCore.passed -or $taskPreflight.compile_exit -ne 0 -or -not $taskPreflight.zero_match_guard){throw 'L1 failed, had unexpected skips, or native extension/driver guards failed'}
    }
    if($Stage -in @('Controlled','All')) {
        $taskControlled=Invoke-SRControlledStage $taskRoot $taskOut $taskCatalog
        $taskReport.stages+=@($taskControlled.stage)
        if(-not $taskControlled.passed){throw 'Controlled prerequisite failed, skipped or did not execute every selected scenario'}
    }
    $taskReport.status='passed';$taskExit=0
} catch { $taskReport.status='failed';$taskReport['error']=$_.Exception.Message;Write-Host $_.Exception.Message -ForegroundColor Red }
finally {
    Pop-Location
    $taskReport['finished_at']=[DateTime]::UtcNow.ToString('o')
    Write-SRPrerequisiteJSON (Join-Path $taskOut 'stages.json') $taskReport
    Write-Host "Prerequisite report: $taskOut/stages.json"
}
exit $taskExit
