param([Parameter(Mandatory=$true)][string]$Repository,[string]$Evidence='')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $Repository 'scripts/native-result.ps1')
function Convert-ResultObject($Value){return ($Value|ConvertTo-Json -Depth 30|ConvertFrom-Json)}
$taskResult=Convert-ResultObject @{exit_code=0;scenario_count=1}
$taskManifest=Convert-ResultObject @{scenario='local-codex.audited';level='L3';driver='development-audited'}
$taskBase=@{scenario='local-codex.audited';level='L3';passed=$true;run_stopped=$true;service_stopped=$true;run_id='first';pr_url='pr-first';original_head='first-head'}
Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskBase)
foreach($taskBadResult in @(@{exit_code=0;scenario_count=0},@{exit_code=1;scenario_count=1})){
    $taskRejected=$false;try{Assert-SRNativeScenario $taskManifest (Convert-ResultObject $taskBadResult) (Convert-ResultObject $taskBase)}catch{$taskRejected=$true}
    if(-not $taskRejected){throw 'Zero execution or failing native exit was accepted'}
}
$taskBad=$taskBase.Clone();$taskBad.passed='true'
$taskRejected=$false;try{Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskBad)}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'String pass flag was accepted as verified boolean'}
$taskBad=$taskBase.Clone();$taskBad.scenario='foreign'
$taskRejected=$false;try{Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskBad)}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Foreign scenario identity was accepted'}
foreach($taskField in @('passed','run_stopped','service_stopped','run_id','pr_url','original_head')){
    $taskBad=$taskBase.Clone();$taskBad.Remove($taskField)
    $taskRejected=$false;try{Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskBad)}catch{$taskRejected=$true}
    if(-not $taskRejected){throw "Missing result field accepted: $taskField"}
}
$taskBad=$taskBase.Clone();$taskBad.merge_commit='unexpected'
$taskRejected=$false;try{Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskBad)}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Audited PR merge was accepted'}
$taskManifest.driver='development-feedback'
$taskRejected=$false;try{Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskBase)}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Feedback case passed without feedback/merge'}
$taskFull=$taskBase.Clone();$taskFull.merge_commit='merge';$taskFull.feedback_head='updated'
Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskFull)
$taskManifest.driver='development-core';$taskFull.Remove('feedback_head')
Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskFull)
$taskManifest.driver='development-multirun'
$taskMulti=$taskBase.Clone();$taskMulti.runs=@(@{run_id='a';stop_status='stopped';new_inputs_allowed=$false;execution=@{session_id='s-a';workspace='w-a';branch='b-a'};pr=@{url='pr-a'}},@{run_id='b';stop_status='stopped';new_inputs_allowed=$false;execution=@{session_id='s-b';workspace='w-b';branch='b-b'};pr=@{url='pr-b'}})
Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskMulti)
$taskMulti.runs[1].execution.session_id='s-a'
$taskRejected=$false;try{Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskMulti)}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Multi-Run shared session was accepted'}
$taskResourceManifest=Convert-ResultObject @{root='fixture';base_branch='base'}
$taskResourceView=Convert-ResultObject @{run_id='first';execution=@{session_id='s';workspace='w';branch='b'};pr=@{url='pr-first'}}
$taskResources=@(Get-SRNativeResources $taskResourceManifest (Convert-ResultObject $taskBase) $taskResourceView)
foreach($taskResource in @('root:fixture','base:base','run:first','session:s','workspace:w','branch:b','pr:pr-first')){if($taskResource -notin $taskResources){throw "Missing native resource registration: $taskResource"}}
foreach($taskField in @('session_id','workspace','branch')){
    $taskBadView=$taskResourceView|ConvertTo-Json -Depth 20|ConvertFrom-Json
    $taskBadView.execution.PSObject.Properties.Remove($taskField)
    $taskRejected=$false;try{Get-SRNativeResources $taskResourceManifest (Convert-ResultObject $taskBase) $taskBadView|Out-Null}catch{$taskRejected=$true}
    if(-not $taskRejected){throw "Missing native identity accepted: $taskField"}
}
$taskResourceView.run_id='foreign'
$taskRejected=$false;try{Get-SRNativeResources $taskResourceManifest (Convert-ResultObject $taskBase) $taskResourceView|Out-Null}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Foreign native resource snapshot accepted'}
if($Evidence){
    $null=[IO.Directory]::CreateDirectory($Evidence)
    $taskHistory=Join-Path $Evidence 'audit-history.jsonl'
    $taskModelManifest=Convert-ResultObject @{audit_model=@{model='auditor';reasoning_effort='high'};execution=@{settings=@{model='executor';reasoning_effort='high'}}}
    $taskModelView=Convert-ResultObject @{execution=@{session_id='session';workspace='C:/fixture';turn_id='publish'};audit=@{results=@(@{step='audit.spec';turn_id='spec';verdict='pass'},@{step='audit.code';turn_id='code';verdict='pass'})}}
    $taskHistoryRows=@(@{type='session_meta';payload=@{id='session';cwd='C:/fixture'}},@{type='turn_context';payload=@{turn_id='spec';model='auditor';effort='high'}},@{type='turn_context';payload=@{turn_id='code';model='auditor';effort='high'}},@{type='turn_context';payload=@{turn_id='publish';model='executor';effort='high'}})
    function Write-AuditHistory { [IO.File]::WriteAllLines($taskHistory,@($taskHistoryRows|ForEach-Object {$_|ConvertTo-Json -Depth 10 -Compress})) }
    Write-AuditHistory
    Assert-SRAuditModels $taskModelManifest $taskModelView $taskHistory|Out-Null
    foreach($taskIndex in @(0,1,3)){
        $taskField=if($taskIndex -eq 0){'id'}else{'model'}
        $taskOriginal=$taskHistoryRows[$taskIndex].payload[$taskField];$taskHistoryRows[$taskIndex].payload[$taskField]='foreign'
        Write-AuditHistory
        $taskRejected=$false;try{Assert-SRAuditModels $taskModelManifest $taskModelView $taskHistory|Out-Null}catch{$taskRejected=$true}
        if(-not $taskRejected){throw 'Native audit accepted foreign session, wrong audit model or missing model restoration'}
        $taskHistoryRows[$taskIndex].payload[$taskField]=$taskOriginal
    }
}
Write-Output 'Optional merge field, required cleanup/feedback, and independent multi-Run result checks passed'
