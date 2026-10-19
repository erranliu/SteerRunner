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
$taskFull=$taskBase.Clone();$taskFull.merge_commit='merge';$taskFull.feedback_rejected=$true
Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskFull)
$taskManifest.driver='development-smoke';$taskFull.Remove('feedback_rejected')
Assert-SRNativeScenario $taskManifest $taskResult (Convert-ResultObject $taskFull)
$taskManifest.driver='result-acquisition'
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
$taskEnvManifest=Convert-ResultObject @{scenario='environment-check.local-sandbox';level='L3';driver='environment-check';base_revision='base';execution=@{settings=@{access_mode='sandbox'}}}
$taskEnvScenario=$taskBase.Clone();$taskEnvScenario.scenario=$taskEnvManifest.scenario
$taskEnvItems=@(1..12|ForEach-Object {@{required=$true;status='passed';evidence=@(@{verified=$true;source='native_tool_and_owner_verification';check_id='first';session_id='session';turn_id='turn';nonce='nonce';operation_id='operation';tool_call_id='tool'})}})
$taskEnv=Convert-ResultObject @{ok=$true;data=@{check=@{check_id='first';status='completed';result='passed';cleanup_status='completed';input=@{execution=@{settings=@{access_mode='sandbox'}}};actual=@{access_mode='sandbox';approval_policy='never';session_id='session';turn_id='turn';nonce='nonce'};resources=@{base_sha='base';head_sha='first-head';pr_url='pr-first';pr_closed=$true;base_deleted=$true;head_deleted=$true;writer_stopped=$true;local_removed=$true};items=$taskEnvItems}}}
Assert-SRNativeScenario $taskEnvManifest $taskResult (Convert-ResultObject $taskEnvScenario)
Assert-SREnvironmentScenario $taskEnvManifest (Convert-ResultObject $taskEnvScenario) $taskEnv
foreach($taskMutation in @('mode','cleanup','evidence')){
    $taskBadEnvelope=$taskEnv|ConvertTo-Json -Depth 30|ConvertFrom-Json
    switch($taskMutation){'mode'{$taskBadEnvelope.data.check.actual.access_mode='full_access'};'cleanup'{$taskBadEnvelope.data.check.resources.pr_closed=$false};'evidence'{$taskBadEnvelope.data.check.items[0].evidence[0].session_id='foreign'}}
    $taskRejected=$false;try{Assert-SREnvironmentScenario $taskEnvManifest (Convert-ResultObject $taskEnvScenario) $taskBadEnvelope}catch{$taskRejected=$true}
    if(-not $taskRejected){throw "Environment Check merge lost its $taskMutation validation"}
}
if($Evidence){
    $null=[IO.Directory]::CreateDirectory($Evidence)
    $taskHistory=Join-Path $Evidence 'audit-history.jsonl'
    $taskModelManifest=Convert-ResultObject @{audit_model=@{model='auditor';reasoning_effort='high'};execution=@{settings=@{model='executor';reasoning_effort='high'}}}
    $taskModelView=Convert-ResultObject @{execution=@{session_id='session';workspace='C:/fixture';turn_id='publish'};pr=@{head_revision='post'};audit=@{results=@(@{step='audit.code.execute';turn_id='code';source_result_id='implementation';result_id='audit';pre_head='pre';post_head='post'})}}
    $taskHistoryRows=@(@{type='session_meta';payload=@{id='session';cwd='C:/fixture'}},@{type='turn_context';payload=@{turn_id='code';model='auditor';effort='high'}},@{type='turn_context';payload=@{turn_id='publish';model='executor';effort='high'}})
    function Write-AuditHistory { [IO.File]::WriteAllLines($taskHistory,@($taskHistoryRows|ForEach-Object {$_|ConvertTo-Json -Depth 10 -Compress})) }
    Write-AuditHistory
    foreach($taskSourceField in @('source_result_id','result_id','pre_head','post_head')) {
        $taskSource=$taskModelView.audit.results[0].$taskSourceField
        $taskModelView.audit.results[0].$taskSourceField=''
        $taskRejected=$false;try{Assert-SRAuditModels $taskModelManifest $taskModelView $taskHistory|Out-Null}catch{$taskRejected=$true}
        if(-not $taskRejected){throw "Audit accepted missing source binding: $taskSourceField"}
        $taskModelView.audit.results[0].$taskSourceField=$taskSource
    }
    Assert-SRAuditModels $taskModelManifest $taskModelView $taskHistory|Out-Null
    foreach($taskIndex in @(0,1,2)){
        $taskField=if($taskIndex -eq 0){'id'}else{'model'}
        $taskOriginal=$taskHistoryRows[$taskIndex].payload[$taskField];$taskHistoryRows[$taskIndex].payload[$taskField]='foreign'
        Write-AuditHistory
        $taskRejected=$false;try{Assert-SRAuditModels $taskModelManifest $taskModelView $taskHistory|Out-Null}catch{$taskRejected=$true}
        if(-not $taskRejected){throw 'Native audit accepted foreign session, wrong audit model or missing model restoration'}
        $taskHistoryRows[$taskIndex].payload[$taskField]=$taskOriginal
    }
}
$taskRemoteManifest=Convert-ResultObject @{audit_model=@{model='auditor';reasoning_effort='high'};execution=@{settings=@{agent_launch=@{model='executor';reasoning_effort='high'}}}}
$taskRemoteView=Convert-ResultObject @{run_id='remote-run';execution=@{session_id='original-outer';turn_id='publish-command'};audit=@{results=@(@{step='audit.spec';command_id='spec-command';turn_id='spec-command';verdict='pass'},@{step='audit.code';command_id='code-command';turn_id='code-command';verdict='pass'})}}
$taskRemoteTurns=@()
foreach($taskRemoteStep in @('audit.spec','audit.code','publish.report')){
    $taskRemoteCommand=switch($taskRemoteStep){'audit.spec'{'spec-command'};'audit.code'{'code-command'};default{'publish-command'}}
    $taskRemoteModel=if($taskRemoteStep -eq 'publish.report'){'executor'}else{'auditor'}
    $taskRemoteTurns+=@{step=$taskRemoteStep;command_id=$taskRemoteCommand;turn_id=$taskRemoteCommand;native_turn_id=('native-'+$taskRemoteCommand);model=$taskRemoteModel;effort='high';source='codex.app-server.original-turn.v1';session='original-native';final_digest='native-final';inspector_closed=$true}
}
$taskRemoteProof=Convert-ResultObject @{passed=$true;run_id='remote-run';session='original-outer';provider_session='original-native';turns=$taskRemoteTurns}
Assert-SRRemoteAuditModels $taskRemoteManifest $taskRemoteView $taskRemoteProof
foreach($taskMutation in @('foreign-run','foreign-provider','unclosed','duplicate-native','wrong-audit','wrong-publication','no-proof')){
    $taskBad=$taskRemoteProof|ConvertTo-Json -Depth 20|ConvertFrom-Json
    switch($taskMutation){
        'foreign-run'{$taskBad.run_id='foreign'}
        'foreign-provider'{$taskBad.turns[0].session='foreign'}
        'unclosed'{$taskBad.turns[0].inspector_closed=$false}
        'duplicate-native'{$taskBad.turns[1].native_turn_id=$taskBad.turns[0].native_turn_id}
        'wrong-audit'{$taskBad.turns[0].model='executor'}
        'wrong-publication'{$taskBad.turns[2].model='auditor'}
        'no-proof'{$taskBad.passed=$false}
    }
    $taskRejected=$false;try{Assert-SRRemoteAuditModels $taskRemoteManifest $taskRemoteView $taskBad}catch{$taskRejected=$true}
    if(-not $taskRejected){throw "Remote audit accepted invalid native model evidence: $taskMutation"}
}
Write-Output 'Optional merge field, required cleanup/feedback, independent multi-Run and remote native model evidence checks passed'
