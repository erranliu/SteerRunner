function Get-SRResultField($Value,[string]$Name) {
    if($null -ne $Value -and $Value.PSObject.Properties[$Name]) { return $Value.$Name }
    return $null
}
function Assert-SRRemoteAuditModels($Manifest,$View,$Proof) {
    if($Proof.passed -isnot [bool] -or -not $Proof.passed -or $Proof.run_id -cne $View.run_id -or $Proof.session -cne $View.execution.session_id -or -not $Proof.provider_session -or @($Proof.turns).Count -ne 3 -or @($View.audit.results).Count -ne 2){throw 'Remote audit proof lacks original execution and three native model contexts'}
    $taskSeen=@{}
    foreach($taskFact in $Proof.turns){
        if(-not $taskFact.command_id -or -not $taskFact.native_turn_id -or $taskSeen.ContainsKey($taskFact.native_turn_id) -or $taskFact.source -cne 'codex.app-server.original-turn.v1' -or $taskFact.session -cne $Proof.provider_session -or -not $taskFact.final_digest -or $taskFact.inspector_closed -isnot [bool] -or -not $taskFact.inspector_closed){throw 'Remote native model context is incomplete, duplicated or foreign'}
        $taskSeen[$taskFact.native_turn_id]=$true
    }
    foreach($taskAudit in $View.audit.results){
        $taskFacts=@($Proof.turns|Where-Object {$_.command_id -ceq $taskAudit.command_id -and $_.turn_id -ceq $taskAudit.turn_id -and $_.step -ceq $taskAudit.step})
        if($taskAudit.verdict -cne 'pass' -or $taskFacts.Count -ne 1 -or $taskFacts[0].model -cne $Manifest.audit_model.model -or $taskFacts[0].effort -cne $Manifest.audit_model.reasoning_effort){throw 'Remote native audit did not use the frozen audit model and effort'}
    }
    $taskPublication=@($Proof.turns|Where-Object {$_.command_id -ceq $View.execution.turn_id -and $_.step -in @('publish','publish.report')})
    if($taskPublication.Count -ne 1 -or $taskPublication[0].model -cne $Manifest.execution.settings.agent_launch.model -or $taskPublication[0].effort -cne $Manifest.execution.settings.agent_launch.reasoning_effort -or $taskPublication[0].model -ceq $Manifest.audit_model.model){throw 'Remote native publication did not restore the distinct execution model'}
}
function Assert-SRAuditModels($Manifest,$View,[string]$History) {
    $taskContexts=@{}
    $taskBound=$false
    $taskReader=[IO.File]::OpenText($History)
    try { while($null -ne ($taskLine=$taskReader.ReadLine())){
        $taskRow=$taskLine|ConvertFrom-Json
        if($taskRow.type -eq 'session_meta'){
            if($taskRow.payload.id -cne $View.execution.session_id -or ($taskRow.payload.cwd -replace '\\','/') -ine ($View.execution.workspace -replace '\\','/')){throw 'Audit native history belongs to another session/workspace'}
            $taskBound=$true
        }
        if($taskRow.type -eq 'turn_context'){
            $taskID=Get-SRResultField $taskRow.payload 'turn_id'
            if(-not $taskID -or $taskContexts.ContainsKey($taskID)){throw 'Audit native turn context is missing or duplicated'}
            $taskContexts[$taskID]=$taskRow.payload
        }
    }} finally {$taskReader.Dispose()}
    if(-not $taskBound -or @($View.audit.results).Count -ne 1){throw 'Audit model evidence lacks its bound session and single code audit'}
    $taskRecords=@()
    foreach($taskAudit in $View.audit.results){
        if($taskAudit.step -cne 'audit.code.execute' -or (Get-SRResultField $taskAudit 'failure') -or -not $taskAudit.source_result_id -or -not $taskAudit.result_id -or -not $taskAudit.pre_head -or $taskAudit.post_head -cne $View.pr.head_revision -or -not $taskContexts.ContainsKey($taskAudit.turn_id)){throw 'Audit lacks its source-bound native turn and verified post-head'}
        $taskContext=$taskContexts[$taskAudit.turn_id]
        if($taskContext.model -cne $Manifest.audit_model.model -or $taskContext.effort -cne $Manifest.audit_model.reasoning_effort){throw 'Native audit model/effort differs from the frozen selection'}
        $taskRecords+=@{step=$taskAudit.step;turn_id=$taskAudit.turn_id;model=$taskContext.model;effort=$taskContext.effort}
    }
    if(-not $taskContexts.ContainsKey($View.execution.turn_id)){throw 'Publication lacks its original native turn context'}
    $taskContext=$taskContexts[$View.execution.turn_id]
    if($taskContext.model -cne $Manifest.execution.settings.model -or $taskContext.effort -cne $Manifest.execution.settings.reasoning_effort -or $taskContext.model -ceq $Manifest.audit_model.model){throw 'Native publication did not return to the distinct execution model'}
    $taskRecords+=@{step='publish';turn_id=$View.execution.turn_id;model=$taskContext.model;effort=$taskContext.effort}
    return @{session=$View.execution.session_id;native_sha256=(Get-FileHash -LiteralPath $History -Algorithm SHA256).Hash.ToLowerInvariant();turns=$taskRecords;passed=$true}
}
function Get-SRNativeResources($Manifest,$Scenario,$View) {
    $taskResources=@(('root:'+$Manifest.root),('base:'+$Manifest.base_branch))
    $taskRuns=if($Scenario.PSObject.Properties['runs']){@($Scenario.runs)}else{@($View)}
    foreach($taskRun in $taskRuns){
        if(-not (Get-SRResultField $taskRun 'run_id')){throw 'Native resource snapshot lacks Run identity'}
        if(-not $Scenario.PSObject.Properties['runs'] -and $taskRun.run_id -cne $Scenario.run_id){throw 'Native resource snapshot belongs to another Run'}
        $taskResources+='run:'+$taskRun.run_id
        foreach($taskField in @('workspace','session_id','branch')){
            $taskValue=Get-SRResultField (Get-SRResultField $taskRun 'execution') $taskField
            if([string]::IsNullOrWhiteSpace([string]$taskValue)){throw "Native resource snapshot lacks $taskField"}
            $taskKind=if($taskField -eq 'session_id'){'session'}else{$taskField}
            $taskResources+=$taskKind+':'+$taskValue
        }
        $taskPR=Get-SRResultField (Get-SRResultField $taskRun 'pr') 'url'
        if(-not $Scenario.PSObject.Properties['runs'] -and (Get-SRResultField $Scenario 'pr_url') -and $taskPR -cne $Scenario.pr_url){throw 'Native resource snapshot lacks the verified PR identity'}
        if($taskPR){$taskResources+='pr:'+$taskPR}
    }
    return $taskResources
}
function Assert-SRNativeScenario($Manifest,$Result,$Scenario) {
    if ((Get-SRResultField $Result 'exit_code') -ne 0 -or (Get-SRResultField $Result 'scenario_count') -ne 1) { throw 'Live scenario did not execute exactly once and pass' }
    if ((Get-SRResultField $Scenario 'scenario') -cne $Manifest.scenario -or (Get-SRResultField $Scenario 'level') -cne $Manifest.level) { throw 'Live scenario identity differs from its manifest' }
    foreach($taskField in @('passed','run_stopped','service_stopped')) {
        $taskValue=Get-SRResultField $Scenario $taskField
        if ($taskValue -isnot [bool] -or -not $taskValue) { throw "Live result lacks confirmed $taskField" }
    }
    foreach($taskField in @('run_id','pr_url','original_head')) { if ([string]::IsNullOrWhiteSpace([string](Get-SRResultField $Scenario $taskField))) { throw "Live result lacks $taskField" } }
    $taskMerge=Get-SRResultField $Scenario 'merge_commit'
    switch($Manifest.driver) {
        'environment-check' { if($taskMerge){throw 'Environment check must close its verified PR without merging'} }
        'development-audited' { if ($taskMerge) { throw 'Audited scenario must leave its verified PR unmerged' } }
        'development-multirun' {
            $taskRuns=@(Get-SRResultField $Scenario 'runs')
            if($taskRuns.Count -ne 2){throw 'Multi-Run result must contain two Run resources'}
            foreach($taskRun in $taskRuns){if((Get-SRResultField $taskRun 'stop_status') -cne 'stopped' -or (Get-SRResultField $taskRun 'new_inputs_allowed') -isnot [bool] -or $taskRun.new_inputs_allowed){throw 'Multi-Run resources are not stopped/revoked'}}
            foreach($taskField in @('run_id')){if(@($taskRuns|ForEach-Object {Get-SRResultField $_ $taskField}|Where-Object {$_}|Select-Object -Unique).Count -ne 2){throw 'Multi-Run identities collide or are missing'}}
            foreach($taskField in @('session_id','workspace','branch')){if(@($taskRuns|ForEach-Object {Get-SRResultField (Get-SRResultField $_ 'execution') $taskField}|Where-Object {$_}|Select-Object -Unique).Count -ne 2){throw 'Multi-Run native resources collide or are missing'}}
            if(@($taskRuns|ForEach-Object {Get-SRResultField (Get-SRResultField $_ 'pr') 'url'}|Where-Object {$_}|Select-Object -Unique).Count -ne 2 -or $taskMerge){throw 'Multi-Run PR identities collide or were unexpectedly merged'}
        }
        {$_ -in @('development-smoke','development-feedback','result-acquisition')} {
            if(-not $taskMerge){throw 'Live smoke lacks its verified merge'}
            if($_ -eq 'development-feedback' -and (Get-SRResultField $Scenario 'feedback_rejected') -cne $true){throw 'Feedback boundary case lacks explicit rejection evidence'}
            if(Get-SRResultField $Scenario 'feedback_head'){throw 'workflow/2 must not create a feedback revision'}
        }
        default {throw 'Unsupported development result driver'}
    }
}

function Assert-SREnvironmentScenario($taskNative,$taskScenario,$taskCheckEnvelope) {
    $taskMerge=Get-SRResultField $taskScenario 'merge_commit'
 $taskCheck = $taskCheckEnvelope.data.check
 if (-not $taskCheckEnvelope.ok -or $taskCheck.check_id -ne $taskScenario.run_id -or $taskCheck.status -ne 'completed' -or $taskCheck.result -ne 'passed' -or $taskCheck.cleanup_status -ne 'completed' -or $taskCheck.input.execution.settings.access_mode -ne $taskNative.execution.settings.access_mode -or $taskCheck.actual.access_mode -ne $taskNative.execution.settings.access_mode -or $taskCheck.actual.approval_policy -ne 'never' -or -not $taskCheck.actual.session_id -or -not $taskCheck.actual.turn_id -or $taskCheck.resources.base_sha -ne $taskNative.base_revision -or $taskCheck.resources.head_sha -ne $taskScenario.original_head -or $taskCheck.resources.pr_url -ne $taskScenario.pr_url) { throw 'Original complete Check context/result does not match the frozen native case' }
 foreach ($taskField in @('pr_closed','base_deleted','head_deleted','writer_stopped','local_removed')) { if (-not $taskCheck.resources.$taskField) { throw "Unsettled original Check resource: $taskField" } }
 if (@($taskCheck.items).Count -ne 12) { throw 'Original native manifest is incomplete' }
 foreach ($taskItem in $taskCheck.items) {
  if (-not $taskItem.required -or $taskItem.status -ne 'passed' -or @($taskItem.evidence).Count -eq 0) { throw 'Original native item lacks passed required evidence' }
  foreach ($taskFact in $taskItem.evidence) { if (-not $taskFact.verified -or $taskFact.source -ne 'native_tool_and_owner_verification' -or $taskFact.check_id -ne $taskCheck.check_id -or $taskFact.session_id -ne $taskCheck.actual.session_id -or $taskFact.turn_id -ne $taskCheck.actual.turn_id -or $taskFact.nonce -ne $taskCheck.actual.nonce -or -not $taskFact.operation_id -or -not $taskFact.tool_call_id) { throw 'Original native evidence identity conflicts with its Check' } }
 }

 if (-not $taskScenario.pr_url -or -not $taskScenario.original_head -or $taskMerge) { throw 'Environment check must verify and close its owned PR without merging' }

}
