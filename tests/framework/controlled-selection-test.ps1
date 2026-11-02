param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $Repository 'scripts/controlled-tests.ps1')
. (Join-Path $Repository 'scripts/test-prerequisite-contract.ps1')
$taskRoot=Join-Path $Evidence 'controlled-selection'
$null=New-Item -ItemType Directory -Path $taskRoot
$taskCatalog=Get-Content -LiteralPath (Join-Path $Repository 'tests/catalog.json') -Raw | ConvertFrom-Json
$taskWire=$taskCatalog | ConvertTo-Json -Depth 40
$taskChecks=[Collections.Generic.List[string]]::new()
function Reject([string]$Name,[scriptblock]$Action) {
    $rejected=$false
    try { & $Action | Out-Null } catch { $rejected=$true }
    if (-not $rejected) { throw "Expected rejection: $Name" }
    $taskChecks.Add($Name)
}
function JSON([string]$Path,$Value) { [IO.File]::WriteAllText($Path,($Value | ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false)) }
Assert-SRControlledSelectionCatalog $taskCatalog
$taskAll=Resolve-SRControlledSelection $taskCatalog @($taskCatalog.controlled_selection.groups.PSObject.Properties.Name)
if (($taskAll.tests -join ',') -cne ($taskCatalog.controlled_scenarios -join ',') -or $taskAll.tests.Count -ne 27) { throw 'Primary groups lost or reordered complete Controlled tests' }
foreach ($kind in @('groups','combinations')) {
    foreach ($definition in $taskCatalog.controlled_selection.$kind.PSObject.Properties) {
        $selection=if($kind -eq 'groups'){Resolve-SRControlledSelection $taskCatalog @($definition.Name)}else{Resolve-SRControlledSelection $taskCatalog @() @($definition.Name)}
        $matched=@($taskCatalog.controlled_scenarios | Where-Object {$_ -cmatch $selection.pattern})
        if (($matched -join ',') -cne ($selection.tests -join ',') -or 'TestUnregistered' -cmatch $selection.pattern) { throw 'Selection is not exact' }
    }
}
$taskChecks.Add('all-groups-and-combinations-exact')
$seal=Resolve-SRControlledSelection $taskCatalog @() @('execute-stop')
$both=Resolve-SRControlledSelection $taskCatalog @() @('execute-stop','runtime-files')
if (($seal.result_file_cases -join ',') -cne 'final-seal' -or $seal.pattern.Contains('/') -or $both.result_file_cases.Count -ne 2 -or $taskAll.result_file_cases.Count -ne 2) {throw 'ResultFile narrowing escaped its test or union lost full cases'}
Reject 'result-case-unknown' {Resolve-SRControlledSelection $taskCatalog @('files') @() @('missing')}
Reject 'result-case-duplicate' {Resolve-SRControlledSelection $taskCatalog @('files') @() @('final-seal','final-seal')}
Reject 'result-case-without-parent' {Resolve-SRControlledSelection $taskCatalog @('foundation') @() @('final-seal')}
$taskChecks.Add('result-case-union-and-unfiltered-pattern')
$acquisition=Resolve-SRControlledSelection $taskCatalog @() @('result-parser')
$acquisitionAll=Resolve-SRControlledSelection $taskCatalog @() @('result-plain','result-audit','result-parser')
$mixed=Resolve-SRControlledSelection $taskCatalog @() @('result-plain','execute-stop')
if($acquisition.cases['TestSRResultAcquisition'].Count -ne 3 -or $acquisitionAll.cases['TestSRResultAcquisition'].Count -ne 8 -or
    $taskAll.cases['TestSRResultAcquisition'].Count -ne 8 -or $mixed.cases['TestSRResultFile'].Count -ne 1 -or $mixed.pattern.Contains('/')){throw 'Per-parent cases lost default/union/mixed coverage'}
Reject 'case-path-without-parent' {Resolve-SRControlledSelection $taskCatalog @('foundation') @() @() @('TestSRResultAcquisition/plain-marker')}
Reject 'case-path-unknown-parent' {ConvertTo-SRControlledCasePlan @('TestForeign/plain-marker')}
Reject 'case-path-wildcard' {ConvertTo-SRControlledCasePlan @('TestSRResultAcquisition/*')}
Reject 'case-path-duplicate' {ConvertTo-SRControlledCasePlan @('TestSRResultAcquisition/plain-marker','TestSRResultAcquisition/plain-marker')}
Reject 'legacy-generic-conflict' {Resolve-SRControlledSelection $taskCatalog @('files') @() @('final-seal') @('TestSRResultFile/final-seal')}
$taskChecks.Add('per-parent-case-default-union-and-isolation')
$restartParent='TestSRRestartPreservesAuthorization'
$restartIDs=@('graceful','kill','creation_receipt','pending_stop')
$restartAll=Resolve-SRControlledSelection $taskCatalog @() @('restart-continuation','restart-create-receipt','restart-pending-stop')
$restartWide=Resolve-SRControlledSelection $taskCatalog @() @('restart-pending-stop','execute-stop')
$restartMixed=Resolve-SRControlledSelection $taskCatalog @() @('restart-pending-stop','runtime-files','result-plain')
foreach($plan in @($taskAll,$seal,$restartAll,$restartWide)) {
    if(($plan.cases[$restartParent] -join ',') -cne ($restartIDs -join ',')){throw 'Restart default/execute-stop/union lost one of its four cases'}
}
if(($restartMixed.cases[$restartParent] -join ',') -cne 'pending_stop' -or $restartMixed.cases['TestSRResultFile'].Count -ne 2 -or
    ($restartMixed.cases['TestSRResultAcquisition'] -join ',') -cne 'plain-marker' -or $restartMixed.pattern.Contains('/')){throw 'Restart selection leaked across parents'}
$restartOverride=Resolve-SRControlledSelection $taskCatalog @() @('execute-stop') @() @("$restartParent/creation_receipt")
if(($restartOverride.cases[$restartParent] -join ',') -cne 'creation_receipt' -or ($restartOverride.result_file_cases -join ',') -cne 'final-seal'){throw 'Explicit Restart override changed ResultFile selection'}
Reject 'restart-unknown-case' {ConvertTo-SRControlledCasePlan @("$restartParent/unknown")}
Reject 'restart-duplicate-case' {ConvertTo-SRControlledCasePlan @("$restartParent/graceful","$restartParent/graceful")}
Reject 'restart-foreign-parent-case' {ConvertTo-SRControlledCasePlan @('TestSRResultAcquisition/graceful')}
$taskChecks.Add('restart-four-default-execute-stop-union-and-parent-isolation')
Reject 'empty' {Resolve-SRControlledSelection $taskCatalog}
Reject 'mixed-kinds' {Resolve-SRControlledSelection $taskCatalog @('files') @('execute-stop')}
Reject 'unknown' {Resolve-SRControlledSelection $taskCatalog @('missing')}
Reject 'wildcard' {Resolve-SRControlledSelection $taskCatalog @('*')}
Reject 'case-sensitive' {Resolve-SRControlledSelection $taskCatalog @('Files')}
Reject 'duplicate-id' {Resolve-SRControlledSelection $taskCatalog @('files','files')}
foreach ($mode in @('missing-owner','duplicate-owner','unknown-test','duplicate-combination-test','empty-combination')) {
    $catalog=$taskWire | ConvertFrom-Json
    switch ($mode) {
        'missing-owner' {$catalog.controlled_selection.groups.files=@($catalog.controlled_selection.groups.files | Select-Object -Skip 1)}
        'duplicate-owner' {$catalog.controlled_selection.groups.files+=@('TestSRStandalonePackage')}
        'unknown-test' {$catalog.controlled_selection.combinations.'execute-stop'+=@('TestUnknown')}
        'duplicate-combination-test' {$catalog.controlled_selection.combinations.'execute-stop'+=@('TestSRResultFile')}
        'empty-combination' {$catalog.controlled_selection.combinations.'execute-stop'=@()}
    }
    Reject $mode {Assert-SRControlledSelectionCatalog $catalog}
}
$taskTiming=Join-Path $taskRoot 'timing-source'
$null=New-Item -ItemType Directory -Path $taskTiming
$taskLog="=== RUN   TestSRResultFile`n=== RUN   TestSRResultFile/child`n    --- PASS: TestSRResultFile/child (2.50s)`n--- PASS: TestSRResultFile (3.25s)`n"
[IO.File]::WriteAllText((Join-Path $taskTiming 'integration.log'),$taskLog)
JSON (Join-Path $taskTiming 'result.json') @{exit_code=0;scenario_count=1}
$timings=Get-SRControlledTimingReport $taskCatalog $taskTiming @('TestSRResultFile')
if ($timings.status -ne 'passed' -or $timings.observed_top_level_seconds -ne 3.25 -or $timings.tests.Count -ne 1 -or $timings.complete_controlled_passed -or $timings.qualification_eligible -or @($timings.groups | Where-Object {$_.id -eq 'files'})[0].observed_seconds -ne 3.25) { throw 'Timing double-counted children or granted qualification' }
$taskChecks.Add('top-level-only-group-cost')
foreach ($mode in @('zero','skip','fail','unfinished','duplicate','unknown','unexpected','missing','count','exit','missing-result')) {
    $log=$taskLog; $expected=@('TestSRResultFile');$result=@{exit_code=0;scenario_count=1}
    switch ($mode) {
        'zero' {$log='';$result.scenario_count=0}
        'skip' {$log=$log.Replace('--- PASS: TestSRResultFile (','--- SKIP: TestSRResultFile (')}
        'fail' {$log=$log.Replace('--- PASS: TestSRResultFile (','--- FAIL: TestSRResultFile (')}
        'unfinished' {$log="=== RUN   TestSRResultFile`n"}
        'duplicate' {$log+=$taskLog}
        'unknown' {$log=$log.Replace('TestSRResultFile','TestForeign')}
        'unexpected' {$expected=@('TestSRProgressFile')}
        'missing' {$expected+=@('TestSRProgressFile')}
        'count' {$result.scenario_count=2}
        'exit' {$result.exit_code=1}
    }
    [IO.File]::WriteAllText((Join-Path $taskTiming 'integration.log'),$log)
    JSON (Join-Path $taskTiming 'result.json') $result
    if ($mode -eq 'missing-result') {Remove-Item -LiteralPath (Join-Path $taskTiming 'result.json')}
    $timings=Get-SRControlledTimingReport $taskCatalog $taskTiming $expected
    if ($timings.status -eq 'passed' -or $timings.qualification_eligible -or $timings.complete_controlled_passed) {throw "Invalid timing passed: $mode"}
    if ($mode -eq 'unfinished' -and $null -ne $timings.tests[0].duration_seconds) {throw 'Unfinished test acquired a zero duration'}
    $taskChecks.Add('timing-'+$mode)
}
# Run the real selection entry against an isolated, deterministic foundation peer.
$taskMock=Join-Path $taskRoot 'repository'
$null=New-Item -ItemType Directory -Path (Join-Path $taskMock 'scripts'),(Join-Path $taskMock 'tests')
foreach ($name in @('test-controlled.ps1','controlled-tests.ps1','test-prerequisite-contract.ps1','delivery-contract.ps1')) {Copy-Item -LiteralPath (Join-Path $Repository ('scripts/'+$name)) -Destination (Join-Path $taskMock ('scripts/'+$name))}
$mockCatalog=$taskWire | ConvertFrom-Json
$mockCatalog.controlled_selection.combinations | Add-Member -NotePropertyName 'all-mock' -NotePropertyValue @($mockCatalog.controlled_scenarios)
JSON (Join-Path $taskMock 'tests/catalog.json') $mockCatalog
[IO.File]::WriteAllText((Join-Path $taskMock 'scripts/test-foundation.ps1'),@'
param($OutDir,$Pattern,$TimeoutMinutes,$RunnerExe,[string[]]$ResultFileCase=@(),[string[]]$Case=@())
. (Join-Path $PSScriptRoot 'controlled-tests.ps1')
$null=New-Item -ItemType Directory -Path $OutDir
$catalog=Get-Content (Join-Path $PSScriptRoot '../tests/catalog.json') -Raw|ConvertFrom-Json
$selected=@($catalog.controlled_scenarios|Where-Object{$_ -cmatch $Pattern})
$log=($selected|ForEach-Object{"=== RUN   $_`n--- PASS: $_ (1.00s)"}) -join "`n"
$result=@{exit_code=0;scenario_count=$selected.Count}
$plan=ConvertTo-SRControlledCasePlan $Case
if($ResultFileCase.Count){$plan['TestSRResultFile']=$ResultFileCase}
$result['controlled_cases']=@{}
$definitions=Get-SRControlledCaseDefinitions
foreach($parent in $definitions.Keys) {
    if($selected -cnotcontains $parent){continue}
    $ids=if($plan.ContainsKey($parent)){@($plan[$parent])}else{@($definitions[$parent])}
    $receipts=@($ids|ForEach-Object{@{id=$_;passed=$true;duration_seconds=0.5}})
    $result.controlled_cases[$parent]=$receipts
    if($parent -ceq 'TestSRResultFile'){$result['result_file_cases']=$receipts}
    foreach($id in $ids) {
        $log+="`n=== RUN   $parent/$id`n"
        if($parent -ceq 'TestSRResultFile'){foreach($mask in 0..3){$log+="=== RUN   $parent/$id/enabled-$mask`n    --- PASS: $parent/$id/enabled-$mask (0.10s)`n"}}
        $log+="    --- PASS: $parent/$id (0.50s)`n"
    }
}
[IO.File]::WriteAllText((Join-Path $OutDir 'integration.log'),$log)
$result|ConvertTo-Json -Depth 5|Set-Content (Join-Path $OutDir 'result.json')
@{pid=2147483646}|ConvertTo-Json|Set-Content (Join-Path $OutDir 'source-identity.json')
'@)
$taskShell=(Get-Process -Id $PID).Path
foreach ($id in @('execute-stop','all-mock','acceptance-control','result-plain','result-audit','result-parser','restart-continuation','restart-create-receipt','restart-pending-stop')) {
    $output=Join-Path $taskRoot $id
    & $taskShell -NoProfile -File (Join-Path $taskMock 'scripts/test-controlled.ps1') -Combination $id -OutDir $output *> (Join-Path $taskRoot ($id+'.log'))
    if ($LASTEXITCODE -ne 0) {throw "Selection entry failed: $id"}
    $summary=Get-Content (Join-Path $output 'summary.json') -Raw|ConvertFrom-Json
    if ($summary.status -ne 'passed' -or $summary.validation_scope -ne 'controlled-selection' -or $summary.complete_controlled_passed -or $summary.qualification_eligible -or (Test-Path (Join-Path $output 'prerequisites.json')) -or (Test-Path (Join-Path $output 'controlled/prerequisites.json'))) {throw 'Selected execution gained complete qualification'}
    Reject ('not-reusable-'+$id) {Read-SRPrerequisites $Repository $output}
}
$taskChecks.Add('all-27-selected-still-no-qualification')
$taskCaseSource=Join-Path $taskRoot 'execute-stop/controlled'
$caseProof=Get-SRResultFileCaseEvidence $taskCaseSource @('final-seal')
if($caseProof.required_paths.Count -ne 5){throw 'Enable variants were not required'}
Reject 'partial-cases-not-full' {Get-SRResultFileCaseEvidence $taskCaseSource @(Get-SRResultFileCaseIDs)}
$caseRoot=Join-Path $taskRoot 'case-negatives'
$null=New-Item -ItemType Directory -Path $caseRoot
$caseLog=[IO.File]::ReadAllText((Join-Path $taskCaseSource 'integration.log'))
$caseResult=[IO.File]::ReadAllText((Join-Path $taskCaseSource 'result.json'))
foreach($mode in @('parent-only','missing-leaf','skipped-leaf','duplicate-leaf','foreign-leaf','receipt-only','receipt-missing','receipt-failed','receipt-duration','receipt-duration-bool')) {
    $log=$caseLog;$result=$caseResult|ConvertFrom-Json
    switch($mode) {
        'parent-only' {$log="=== RUN   TestSRResultFile`n--- PASS: TestSRResultFile (1.00s)"}
        'missing-leaf' {$log=($log -split "`n" | Where-Object {$_ -notlike '*enabled-3*'}) -join "`n"}
        'skipped-leaf' {$log=$log.Replace('--- PASS: TestSRResultFile/final-seal/enabled-2','--- SKIP: TestSRResultFile/final-seal/enabled-2')}
        'duplicate-leaf' {$log+="`n=== RUN   TestSRResultFile/final-seal/enabled-0`n"}
        'foreign-leaf' {$log=$log.Replace('enabled-0','enabled-foreign')}
        'receipt-only' {$log=''}
        'receipt-missing' {$result.PSObject.Properties.Remove('result_file_cases')}
        'receipt-failed' {$result.result_file_cases[0].passed=$false}
        'receipt-duration' {$result.result_file_cases[0].duration_seconds=-1}
        'receipt-duration-bool' {$result.result_file_cases[0].duration_seconds=$true}
    }
    [IO.File]::WriteAllText((Join-Path $caseRoot 'integration.log'),$log)
    JSON (Join-Path $caseRoot 'result.json') $result
    Reject ('case-proof-'+$mode) {Get-SRResultFileCaseEvidence $caseRoot @('final-seal')}
}
$taskChecks.Add('actual-result-case-and-four-enable-variants')
$taskFullSubset=Join-Path $taskRoot 'full-count-partial-cases'
$null=New-Item -ItemType Directory -Path $taskFullSubset
$taskFullSource=Join-Path $taskRoot 'all-mock/controlled'
foreach($name in @('integration.log','result.json','source-identity.json')) {Copy-Item -LiteralPath (Join-Path $taskFullSource $name) -Destination (Join-Path $taskFullSubset $name)}
$partialResult=Get-Content (Join-Path $taskFullSubset 'result.json') -Raw|ConvertFrom-Json
$partialResult.result_file_cases=@($partialResult.result_file_cases | Where-Object {$_.id -eq 'final-seal'})
JSON (Join-Path $taskFullSubset 'result.json') $partialResult
$partialLog=@(Get-Content (Join-Path $taskFullSubset 'integration.log') | Where-Object {$_ -notlike '*TestSRResultFile/optional-file-failure-business-loop*'}) -join "`n"
[IO.File]::WriteAllText((Join-Path $taskFullSubset 'integration.log'),$partialLog)
$caseGateRejected=$false
try {Assert-SRControlledEvidence $taskFullSubset @($taskCatalog.controlled_scenarios) | Out-Null}
catch {$caseGateRejected=$_.Exception.Message -like '*Actual ResultFile cases differ*' -or $_.Exception.Message -like '*ResultFile legacy and per-parent receipts differ*'}
if(-not $caseGateRejected){throw 'Complete 27-test count bypassed the actual ResultFile case gate'}
$taskChecks.Add('full-27-count-cannot-hide-partial-result-cases')
$acqSource=Join-Path $taskRoot 'result-parser/controlled'
$acqProof=Get-SRControlledCaseEvidence $acqSource 'TestSRResultAcquisition' @('parser-unsupported','parser-complete','parser-tool')
if($acqProof.required_paths.Count -ne 3){throw 'Parser subset was not verified by actual paths'}
Reject 'parser-subset-not-full-eight' {Get-SRControlledCaseEvidence $acqSource 'TestSRResultAcquisition' @((Get-SRControlledCaseDefinitions)['TestSRResultAcquisition'])}
$acqNegative=Join-Path $taskRoot 'acquisition-negatives'
$null=New-Item -ItemType Directory -Path $acqNegative
$acqLog=[IO.File]::ReadAllText((Join-Path $acqSource 'integration.log'))
$acqResult=[IO.File]::ReadAllText((Join-Path $acqSource 'result.json'))
foreach($mode in @('parent-only','missing-case','skipped-case','foreign-case','duplicate-case','missing-receipt','wrong-parent','false-receipt','string-duration')) {
    $log=$acqLog;$result=$acqResult|ConvertFrom-Json
    switch($mode) {
        'parent-only' {$log="=== RUN   TestSRResultAcquisition`n--- PASS: TestSRResultAcquisition (1.00s)"}
        'missing-case' {$log=($log -split "`n" | Where-Object {$_ -notlike '*TestSRResultAcquisition/parser-tool*'}) -join "`n"}
        'skipped-case' {$log=$log.Replace('--- PASS: TestSRResultAcquisition/parser-tool','--- SKIP: TestSRResultAcquisition/parser-tool')}
        'foreign-case' {$log=$log.Replace('parser-tool','audit-zero')}
        'duplicate-case' {$log+="`n=== RUN   TestSRResultAcquisition/parser-tool`n"}
        'missing-receipt' {$result.PSObject.Properties.Remove('controlled_cases')}
        'wrong-parent' {$result.controlled_cases.PSObject.Properties.Remove('TestSRResultAcquisition')}
        'false-receipt' {$result.controlled_cases.TestSRResultAcquisition[0].passed=$false}
        'string-duration' {$result.controlled_cases.TestSRResultAcquisition[0].duration_seconds='0.5'}
    }
    [IO.File]::WriteAllText((Join-Path $acqNegative 'integration.log'),$log)
    JSON (Join-Path $acqNegative 'result.json') $result
    Reject ('acquisition-proof-'+$mode) {Get-SRControlledCaseEvidence $acqNegative 'TestSRResultAcquisition' @('parser-unsupported','parser-complete','parser-tool')}
}
$acqFullNegative=Join-Path $taskRoot 'full-count-partial-acquisition'
$null=New-Item -ItemType Directory -Path $acqFullNegative
foreach($name in @('integration.log','result.json','source-identity.json')){Copy-Item -LiteralPath (Join-Path $taskFullSource $name) -Destination (Join-Path $acqFullNegative $name)}
$partial=Get-Content (Join-Path $acqFullNegative 'result.json') -Raw|ConvertFrom-Json
$partial.controlled_cases.TestSRResultAcquisition=@($partial.controlled_cases.TestSRResultAcquisition | Where-Object {$_.id -like 'parser-*'})
JSON (Join-Path $acqFullNegative 'result.json') $partial
$acqGateRejected=$false
try{Assert-SRControlledEvidence $acqFullNegative @($taskCatalog.controlled_scenarios) | Out-Null}catch{$acqGateRejected=$_.Exception.Message -like '*Actual ResultAcquisition cases differ*'}
if(-not $acqGateRejected){throw 'Full parent count hid incomplete acquisition cases'}
$taskChecks.Add('full-27-count-requires-all-eight-acquisition-cases')
$restartSource=Join-Path $taskRoot 'restart-continuation/controlled'
$restartProof=Get-SRControlledCaseEvidence $restartSource $restartParent @('graceful','kill')
if($restartProof.required_paths.Count -ne 2){throw 'Restart continuation lost actual two-case proof'}
Reject 'restart-subset-not-default-four' {Get-SRControlledCaseEvidence $restartSource $restartParent $restartIDs}
$restartNegative=Join-Path $taskRoot 'restart-negatives'
$null=New-Item -ItemType Directory -Path $restartNegative
$restartLog=[IO.File]::ReadAllText((Join-Path $restartSource 'integration.log'))
$restartResult=[IO.File]::ReadAllText((Join-Path $restartSource 'result.json'))
foreach($mode in @('parent-only','missing-case','skipped-case','foreign-case','duplicate-case','missing-receipt','wrong-parent','false-receipt','string-duration')) {
    $log=$restartLog;$result=$restartResult|ConvertFrom-Json
    switch($mode) {
        'parent-only' {$log="=== RUN   $restartParent`n--- PASS: $restartParent (1.00s)"}
        'missing-case' {$log=($log -split "`n" | Where-Object {$_ -notlike "*$restartParent/kill*"}) -join "`n"}
        'skipped-case' {$log=$log.Replace("--- PASS: $restartParent/kill","--- SKIP: $restartParent/kill")}
        'foreign-case' {$log=$log.Replace('/kill','/pending_stop')}
        'duplicate-case' {$log+="`n=== RUN   $restartParent/kill`n"}
        'missing-receipt' {$result.PSObject.Properties.Remove('controlled_cases')}
        'wrong-parent' {$result.controlled_cases.PSObject.Properties.Remove($restartParent)}
        'false-receipt' {$result.controlled_cases.$restartParent[0].passed=$false}
        'string-duration' {$result.controlled_cases.$restartParent[0].duration_seconds='0.5'}
    }
    [IO.File]::WriteAllText((Join-Path $restartNegative 'integration.log'),$log)
    JSON (Join-Path $restartNegative 'result.json') $result
    Reject ('restart-proof-'+$mode) {Get-SRControlledCaseEvidence $restartNegative $restartParent @('graceful','kill')}
}
# Check the complete gate both in the full 27 and when Restart is the only
# expected supported parent, so the case guard cannot depend on ResultFile/Acquisition.
foreach($scope in @('full-27','restart-only')) {
    $negative=Join-Path $taskRoot ('partial-restart-'+$scope)
    $null=New-Item -ItemType Directory -Path $negative
    $sourceDirectory=if($scope -eq 'full-27'){$taskFullSource}else{$restartSource}
    foreach($name in @('integration.log','result.json','source-identity.json')){Copy-Item -LiteralPath (Join-Path $sourceDirectory $name) -Destination (Join-Path $negative $name)}
    $partial=Get-Content (Join-Path $negative 'result.json') -Raw|ConvertFrom-Json
    $partial.controlled_cases.$restartParent=@($partial.controlled_cases.$restartParent | Where-Object {$_.id -eq 'graceful'})
    JSON (Join-Path $negative 'result.json') $partial
    $expected=if($scope -eq 'full-27'){@($taskCatalog.controlled_scenarios)}else{@($restartParent)}
    $rejected=$false
    try{Assert-SRControlledEvidence $negative $expected | Out-Null}catch{$rejected=$_.Exception.Message -like '*Actual RestartPreservesAuthorization cases differ*'}
    if(-not $rejected){throw "Complete $scope count bypassed Restart case proof"}
    $taskChecks.Add('complete-case-gate-'+$scope+'-requires-four-restart-cases')
}
$source=Join-Path $taskRoot 'execute-stop/controlled'
$before=(Get-FileHash (Join-Path $source 'integration.log')).Hash
$import=Join-Path $taskRoot 'import'
& $taskShell -NoProfile -File (Join-Path $taskMock 'scripts/test-controlled.ps1') -FromEvidence $source -OutDir $import *> (Join-Path $taskRoot 'import.log')
if ($LASTEXITCODE -ne 0 -or (Get-FileHash (Join-Path $source 'integration.log')).Hash -cne $before) {throw 'Timing import failed or rewrote source'}
$summary=Get-Content (Join-Path $import 'summary.json') -Raw|ConvertFrom-Json
if ($summary.mode -ne 'timing-import' -or $summary.qualification_eligible) {throw 'Timing import gained qualification'}
$taskChecks.Add('read-only-timing-import')
# Exercise the existing full-stage function: diagnostics must not add a gate,
# and a selected subset must still fail its unchanged complete-count gate.
[IO.File]::WriteAllText((Join-Path $taskMock 'scripts/stage-check.ps1'),@'
param($Output)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'test-prerequisite-contract.ps1')
function Get-SRPrerequisiteSnapshotOrUnavailable { @{supported=$true} }
function Save-SRPrerequisiteStage { param($Root,$Output,$Name,$Start,$Record) @{status='passed'} }
$root=Split-Path $PSScriptRoot -Parent
$catalog=Get-Content (Join-Path $root 'tests/catalog.json') -Raw|ConvertFrom-Json
foreach($mode in @('full','diagnostic-unavailable','subset')) {
    $out=Join-Path $Output $mode
    $null=New-Item -ItemType Directory -Path $out
    if($mode -eq 'diagnostic-unavailable'){$catalog.PSObject.Properties.Remove('controlled_selection')}
    if($mode -eq 'subset'){$catalog.controlled_scenarios+=@('TestRequiredButNotExecuted')}
    $stage=Invoke-SRControlledStage $root $out $catalog
    if($mode -eq 'subset') {
        if($stage.passed){throw 'Subset obtained complete-stage success'}
    } elseif(-not $stage.passed) {throw 'Diagnostics changed full-stage success'}
    if($mode -eq 'full' -and -not(Test-Path $stage.record.timing_report)){throw 'Full stage lost timing attachment'}
    if($mode -eq 'diagnostic-unavailable' -and -not $stage.record.ContainsKey('timing_error')){throw 'Diagnostic failure was not recorded'}
}
'@)
& $taskShell -NoProfile -File (Join-Path $taskMock 'scripts/stage-check.ps1') -Output (Join-Path $taskRoot 'stages') *> (Join-Path $taskRoot 'stages.log')
if ($LASTEXITCODE -ne 0) {throw 'Complete-stage qualification or additive timing behavior changed'}
$taskChecks.Add('complete-stage-gates-unchanged')
JSON (Join-Path $taskRoot 'verified.json') @{passed=$true;checks=@($taskChecks.ToArray())}
Write-Output "Controlled selection/timing contracts passed: $($taskChecks.Count)"
