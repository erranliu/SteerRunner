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
$taskSmokeManifest=Convert-ResultObject @{scenario='local-codex.smoke';level='L2';driver='development-smoke'}
$taskSmoke=$taskFull.Clone();$taskSmoke.scenario='local-codex.smoke';$taskSmoke.level='L2'
$taskRejected=$false;try{Assert-SRNativeScenario $taskSmokeManifest $taskResult (Convert-ResultObject $taskSmoke)}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Consolidated smoke omitted the feedback rejection assertion'}
$taskSmoke.feedback_rejected=$true
Assert-SRNativeScenario $taskSmokeManifest $taskResult (Convert-ResultObject $taskSmoke)
$taskParserManifest=Convert-ResultObject @{id='parser-fixture';root='parser-root';base_branch='parser-base';scenario='local-codex.result-acquisition';level='L3';driver='result-acquisition'}
$taskParser=Convert-ResultObject @{
 scenario='local-codex.result-acquisition';level='L3';passed=$true;business_outcome='parsed';analysis_invocations=2;permission_probe_calls=0
 cases=@(@{case='text-analysis';passed=$true},@{case='cancellation';passed=$true})
}
Assert-SRNativeScenario $taskParserManifest $taskResult $taskParser
foreach($taskMutation in @('missing-case','wrong-order','false-pass','wrong-outcome','published','permission-probe')){
 $taskBad=$taskParser|ConvertTo-Json -Depth 30|ConvertFrom-Json
 switch($taskMutation){
 'missing-case'{$taskBad.cases=@($taskBad.cases[0])}
 'wrong-order'{$taskBad.cases[0].case='cancellation'}
 'false-pass'{$taskBad.passed='true'}
 'wrong-outcome'{$taskBad.business_outcome='merged'}
 'published'{$taskBad|Add-Member -NotePropertyName pr_url -NotePropertyValue 'unexpected'}
 'permission-probe'{$taskBad.permission_probe_calls=1}
 }
 $taskRejected=$false;try{Assert-SRNativeScenario $taskParserManifest $taskResult $taskBad}catch{$taskRejected=$true}
 if(-not $taskRejected){throw "Parser result accepted invalid proof: $taskMutation"}
}
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
$taskRemoteManifest=Convert-ResultObject @{audit_model=@{model='auditor';reasoning_effort='high'};execution=@{target=@{kind='local'};settings=@{agent_launch=@{model='executor';reasoning_effort='high'}}}}
$taskRemoteView=Convert-ResultObject @{run_id='remote-run';execution=@{session_id='original-outer';turn_id='native-publish-command'};pr=@{head_revision='post'};audit=@{results=@(@{step='audit.code.execute';command_id='code-command';turn_id='native-code-command';source_result_id='implementation';result_id='audit';pre_head='pre';post_head='post'})}}
$taskRemoteTurns=@()
foreach($taskRemoteStep in @('audit.code.execute','publish.execute')){
    $taskRemoteCommand=if($taskRemoteStep -eq 'audit.code.execute'){'code-command'}else{'publish-command'}
    $taskRemoteModel=if($taskRemoteStep -eq 'audit.code.execute'){'auditor'}else{'executor'}
    $taskRemoteTurns+=@{step=$taskRemoteStep;command_id=$taskRemoteCommand;turn_id=('native-'+$taskRemoteCommand);native_turn_id=('native-'+$taskRemoteCommand);model=$taskRemoteModel;effort='high';source='orca.structured.original-model.v1';provider_session='original-native';source_digest=('a'*64)}
}
$taskRemoteProof=Convert-ResultObject @{passed=$true;run_id='remote-run';session='original-outer';protocol='orca_agent_session_v1';configuration_digest=('a'*64);turns=$taskRemoteTurns}
Assert-SRManagedOrcaAuditModels $taskRemoteManifest $taskRemoteView $taskRemoteProof
$taskRemoteProof.turns[1].step='publish.report'
Assert-SRManagedOrcaAuditModels $taskRemoteManifest $taskRemoteView $taskRemoteProof
$taskRemoteProof.turns[1].step='publish.execute'
foreach($taskMutation in @('foreign-run','foreign-outer','foreign-provider','no-configuration','wrong-protocol','duplicate-native','duplicate-command','foreign-source','missing-source','wrong-turn','wrong-step','wrong-audit','wrong-effort','wrong-publication','wrong-publication-effort','no-proof','missing-turn','extra-spec')){
    $taskBad=$taskRemoteProof|ConvertTo-Json -Depth 20|ConvertFrom-Json
    switch($taskMutation){
        'foreign-run'{$taskBad.run_id='foreign'}
        'foreign-outer'{$taskBad.session='foreign'}
        'foreign-provider'{$taskBad.turns[0].provider_session='foreign'}
        'no-configuration'{$taskBad.configuration_digest=''}
        'wrong-protocol'{$taskBad.protocol='orca_terminal_v3'}
        'duplicate-native'{$taskBad.turns[1].native_turn_id=$taskBad.turns[0].native_turn_id}
        'duplicate-command'{$taskBad.turns[1].command_id=$taskBad.turns[0].command_id;$taskBad.turns[1].turn_id=$taskBad.turns[0].turn_id}
        'foreign-source'{$taskBad.turns[0].source='foreign'}
        'missing-source'{$taskBad.turns[0].source_digest=''}
        'wrong-turn'{$taskBad.turns[0].turn_id='foreign'}
        'wrong-step'{$taskBad.turns[0].step='audit.spec'}
        'wrong-audit'{$taskBad.turns[0].model='executor'}
        'wrong-effort'{$taskBad.turns[0].effort='low'}
        'wrong-publication'{$taskBad.turns[1].model='auditor'}
        'wrong-publication-effort'{$taskBad.turns[1].effort='low'}
        'no-proof'{$taskBad.passed=$false}
        'missing-turn'{$taskBad.turns=@($taskBad.turns[0])}
        'extra-spec'{$taskBad.turns+=($taskRemoteTurns[0]|ConvertTo-Json|ConvertFrom-Json)}
    }
    $taskRejected=$false;try{Assert-SRManagedOrcaAuditModels $taskRemoteManifest $taskRemoteView $taskBad}catch{$taskRejected=$true}
    if(-not $taskRejected){throw "Remote audit accepted invalid native model evidence: $taskMutation"}
}
foreach($taskMutation in @('source_result_id','result_id','pre_head','post_head','failure','step','turn_id','command_id','extra-audit')){
    $taskBadView=$taskRemoteView|ConvertTo-Json -Depth 20|ConvertFrom-Json
    switch($taskMutation){
        'failure'{$taskBadView.audit.results[0]|Add-Member -NotePropertyName failure -NotePropertyValue 'unqualified'}
        'step'{$taskBadView.audit.results[0].step='audit.spec'}
        'extra-audit'{$taskBadView.audit.results+=($taskRemoteView.audit.results[0]|ConvertTo-Json|ConvertFrom-Json)}
        default{$taskBadView.audit.results[0].$taskMutation=''}
    }
    $taskRejected=$false;try{Assert-SRManagedOrcaAuditModels $taskRemoteManifest $taskBadView $taskRemoteProof}catch{$taskRejected=$true}
    if(-not $taskRejected){throw "Remote audit accepted invalid source binding: $taskMutation"}
}
Write-Output 'Optional merge field, required cleanup/feedback, independent multi-Run and remote native model evidence checks passed'

$taskProgressManifest=Convert-ResultObject @{scenario='local-codex.progress-file';level='L3';driver='progress-file'}
$taskProgress=@{scenario='local-codex.progress-file';level='L3';passed=$true;run_stopped=$true;service_stopped=$true;progress_verified=$true;run_id='a';runs=@(@{run_id='a';stop_status='stopped';new_inputs_allowed=$false;execution=@{session_id='s';workspace='w';branch='b'}})}
Assert-SRNativeScenario $taskProgressManifest $taskResult (Convert-ResultObject $taskProgress)
foreach($taskMutation in @('progress_verified','run_id','session_id','workspace','unexpected-pr')){
 $taskBad=$taskProgress|ConvertTo-Json -Depth 20|ConvertFrom-Json
 switch($taskMutation){
  'progress_verified'{$taskBad.progress_verified=$false}
  'run_id'{$taskBad.run_id='foreign'}
  'unexpected-pr'{$taskBad|Add-Member -NotePropertyName pr_url -NotePropertyValue 'unexpected'}
  default{$taskBad.runs[0].execution.$taskMutation=''}
 }
 $taskRejected=$false;try{Assert-SRNativeScenario $taskProgressManifest $taskResult $taskBad}catch{$taskRejected=$true}
 if(-not $taskRejected){throw "Progress result accepted invalid proof: $taskMutation"}
}
Write-Output 'Progress result proof, exact Run identity, native resources and no-publication guards passed'

$taskCandidateManifest=Convert-ResultObject @{driver='native-capability-runner';scenario='orca-local.native-active-stop';level='L3'}
$taskCandidate=Convert-ResultObject @{scenario=$taskCandidateManifest.scenario;level='L3';stage='completed';run_id='original';structured_session=('oas_'+('a'*32));stop_evidence_scope='orca_agent_session';passed=$true;run_stopped=$true;service_stopped=$true;offline_verified=$true;no_legacy_calls=$true;active_verified=$true}
Assert-SRNativeScenario $taskCandidateManifest $taskResult $taskCandidate
foreach($taskField in @('stop_evidence_scope','structured_session','offline_verified','active_verified','no_legacy_calls')){
 $taskBad=$taskCandidate|ConvertTo-Json|ConvertFrom-Json
 if($taskField -eq 'stop_evidence_scope'){$taskBad.$taskField='orca_terminal'}elseif($taskField -eq 'structured_session'){$taskBad.$taskField=''}else{$taskBad.$taskField=$false}
 $taskRejected=$false;try{Assert-SRNativeScenario $taskCandidateManifest $taskResult $taskBad}catch{$taskRejected=$true}
 if(-not $taskRejected){throw "Structured native candidate accepted incomplete proof: $taskField"}
}
Write-Host 'Structured native session scope and active-stop result guards passed'
