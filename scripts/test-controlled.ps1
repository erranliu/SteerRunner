param(
    [string[]]$Group=@(),
    [string[]]$Combination=@(),
    [switch]$List,
    [string]$FromEvidence='',
    [string]$OutDir='',
    [string]$RunnerExe='',
    [string[]]$ResultFileCase=@(),
    [string[]]$Case=@(),
    [ValidateRange(1,60)][int]$TimeoutMinutes=25
)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'controlled-tests.ps1')
. (Join-Path $PSScriptRoot 'test-prerequisite-contract.ps1')
$taskRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskCatalogPath=Join-Path $taskRoot 'tests/catalog.json'
$taskCatalog=Get-Content -LiteralPath $taskCatalogPath -Raw | ConvertFrom-Json
Assert-SRControlledSelectionCatalog $taskCatalog
if ($List) {
    if ($Group.Count -or $Combination.Count -or $FromEvidence -or $OutDir -or $RunnerExe -or $ResultFileCase.Count -or $Case.Count) { throw 'List cannot be combined with selection, evidence or output arguments' }
    $taskCatalog.controlled_selection | ConvertTo-Json -Depth 10
    exit 0
}
$taskSelection=$null
if ($FromEvidence) {
    if ($Group.Count -or $Combination.Count -or $RunnerExe -or $ResultFileCase.Count -or $Case.Count) { throw 'FromEvidence only imports timings; selection and RunnerExe are unsupported' }
    $FromEvidence=(Resolve-Path -LiteralPath $FromEvidence).Path
} else {
    $taskSelection=Resolve-SRControlledSelection $taskCatalog $Group $Combination $ResultFileCase $Case
    Assert-SRNoLiveOptIn
}
if (-not $OutDir) { $OutDir=Join-Path $taskRoot ('.evidence/controlled-selection-'+[Guid]::NewGuid().ToString('N').Substring(0,10)) }
$taskOut=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Controlled selection output directory must be new' }
if ($FromEvidence -and $taskOut.StartsWith($FromEvidence.TrimEnd('\','/')+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) { throw 'Timing import output must be outside the original evidence directory' }
$null=New-Item -ItemType Directory -Path $taskOut
$taskClock=[Diagnostics.Stopwatch]::StartNew()
$taskSummary=@{schema='steerrunner-controlled-selection/v1';validation_scope='controlled-selection';complete_controlled_passed=$false;qualification_eligible=$false;
    mode=$(if($FromEvidence){'timing-import'}else{'execution'});status='running';selection=$taskSelection;catalog_sha256=(Get-FileHash -LiteralPath $taskCatalogPath -Algorithm SHA256).Hash.ToLowerInvariant();
    started_at=[DateTime]::UtcNow.ToString('o')}
$taskExit=1
try {
    $taskExpected=@()
    if ($FromEvidence) { $taskEvidence=$FromEvidence } else {
        $taskEvidence=Join-Path $taskOut 'controlled'
        $taskExpected=@($taskSelection.tests)
        $taskArguments=@{OutDir=$taskEvidence;Pattern=$taskSelection.pattern;TimeoutMinutes=$TimeoutMinutes}
        $taskPaths=@(foreach($taskParent in $taskSelection.cases.Keys){foreach($taskID in $taskSelection.cases[$taskParent]){"$taskParent/$taskID"}})
        if ($taskPaths.Count) { $taskArguments['Case']=$taskPaths }
        if ($RunnerExe) { $taskArguments['RunnerExe']=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($RunnerExe) }
        & (Join-Path $PSScriptRoot 'test-foundation.ps1') @taskArguments | Out-Host
        $null=Wait-SRControlledResult $taskEvidence ([DateTime]::UtcNow.AddMinutes($TimeoutMinutes).AddSeconds(30))
    }
    $taskExpectedCases=if($taskSelection){$taskSelection.cases}else{@{}}
    $taskTimings=Get-SRControlledTimingReport $taskCatalog $taskEvidence $taskExpected @() $taskExpectedCases
    Write-SRPrerequisiteJSON (Join-Path $taskOut 'timings.json') $taskTimings
    $taskSummary['evidence']=$taskEvidence
    $taskSummary['timing_report']=Join-Path $taskOut 'timings.json'
    $taskSummary['observed_top_level_seconds']=$taskTimings.observed_top_level_seconds
    if ($taskTimings.status -ne 'passed') { throw 'Controlled results are incomplete, failed, skipped or differ from the exact selection; inspect timings.json' }
    $taskSummary.status='passed';$taskExit=0
} catch {
    $taskSummary.status='failed';$taskSummary['error']=$_.Exception.Message
    Write-Host $_.Exception.Message -ForegroundColor Red
} finally {
    $taskSummary['duration_seconds']=$taskClock.Elapsed.TotalSeconds
    $taskSummary['finished_at']=[DateTime]::UtcNow.ToString('o')
    Write-SRPrerequisiteJSON (Join-Path $taskOut 'summary.json') $taskSummary
    Write-Host "Controlled selection report: $taskOut/summary.json"
}
exit $taskExit
