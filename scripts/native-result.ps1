function Get-SRResultField($Value,[string]$Name) {
    if($null -ne $Value -and $Value.PSObject.Properties[$Name]) { return $Value.$Name }
    return $null
}
function Assert-SRManagedOrcaAuditModels($Manifest,$View,$Proof) {
    if($Manifest.execution.target.kind -cne 'local'){throw 'Managed audit proof is local only; native remote auditing is unsupported'}
    if($Proof.passed -isnot [bool] -or -not $Proof.passed -or $Proof.run_id -cne $View.run_id -or $Proof.session -cne $View.execution.session_id -or -not $Proof.provider_session -or $Proof.configuration_digest -cnotmatch '^[0-9a-f]{64}$' -or @($Proof.turns).Count -ne 2 -or @($View.audit.results).Count -ne 1){throw 'Remote audit proof lacks its frozen original execution, single code audit and publication'}
    $taskSeen=@{}; $taskCommands=@{}; $taskRequests=@{}
    foreach($taskFact in $Proof.turns){
        if(-not $taskFact.command_id -or $taskFact.turn_id -cne $taskFact.command_id -or $taskCommands.ContainsKey($taskFact.command_id) -or -not $taskFact.native_turn_id -or $taskSeen.ContainsKey($taskFact.native_turn_id) -or $taskFact.source -cne 'codex.app-server.original-turn.v1' -or $taskFact.session -cne $Proof.provider_session -or -not $taskFact.final_digest -or $taskFact.reader_writers_absent -isnot [bool] -or -not $taskFact.reader_writers_absent -or -not $taskFact.control_request_id -or $taskRequests.ContainsKey($taskFact.control_request_id)){throw 'Remote native model context is incomplete, duplicated or foreign'}
        $taskSeen[$taskFact.native_turn_id]=$true; $taskCommands[$taskFact.command_id]=$true; $taskRequests[$taskFact.control_request_id]=$true
    }
    $taskAudit=$View.audit.results[0]
    if($taskAudit.step -cne 'audit.code.execute' -or (Get-SRResultField $taskAudit 'failure') -or -not $taskAudit.source_result_id -or -not $taskAudit.result_id -or -not $taskAudit.pre_head -or -not $taskAudit.post_head -or $taskAudit.post_head -cne $View.pr.head_revision){throw 'Remote code audit lacks its source-bound result and verified post-head'}
    $taskFacts=@($Proof.turns|Where-Object {$_.command_id -ceq $taskAudit.command_id -and $_.turn_id -ceq $taskAudit.turn_id -and $_.step -ceq $taskAudit.step})
    if($taskFacts.Count -ne 1 -or $taskFacts[0].model -cne $Manifest.audit_model.model -or $taskFacts[0].effort -cne $Manifest.audit_model.reasoning_effort){throw 'Remote native audit did not use the frozen audit model and effort'}
    $taskPublication=@($Proof.turns|Where-Object {$_.command_id -ceq $View.execution.turn_id -and $_.step -in @('publish.execute','publish.report')})
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
    if ((Get-SRResultField $Manifest 'driver') -eq 'result-acquisition') {
        foreach ($taskCall in @($Scenario.calls)) {
            $taskResources+='run:'+$taskCall.run_id
            if ($taskCall.session_id) { $taskResources+='session:'+$taskCall.session_id }
        }
        return $taskResources
    }
    $taskRuns=if($Scenario.PSObject.Properties['runs']){@($Scenario.runs)}else{@($View)}
    foreach($taskRun in $taskRuns){
        if(-not (Get-SRResultField $taskRun 'run_id')){throw 'Native resource snapshot lacks Run identity'}
        if(-not $Scenario.PSObject.Properties['runs'] -and $taskRun.run_id -cne $Scenario.run_id){throw 'Native resource snapshot belongs to another Run'}
        $taskResources+='run:'+$taskRun.run_id
        if((Get-SRResultField $Manifest 'driver') -eq 'cli-wait-retry' -and -not (Get-SRResultField (Get-SRResultField $taskRun 'execution') 'workspace')) {
            if($taskRun.stop_status -cne 'stopped' -or $taskRun.new_inputs_allowed -or $taskRun.execution.turn_started){throw 'Canceled CLI wait retained dispatch authority'}
            continue
        }
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
    if ($Manifest.driver -eq 'result-acquisition') {
        Assert-SRParserScenario $Manifest $Scenario
        return
    }
    foreach($taskField in @('passed','run_stopped','service_stopped')) {
        $taskValue=Get-SRResultField $Scenario $taskField
        if ($taskValue -isnot [bool] -or -not $taskValue) { throw "Live result lacks confirmed $taskField" }
    }
    if ($Manifest.driver -eq 'cli-wait-retry') {
        $taskRuns=@(Get-SRResultField $Scenario 'runs')
        foreach($taskField in @('wait_verified','cancel_verified','native_audit')) {if((Get-SRResultField $Scenario $taskField) -cne $true){throw 'CLI wait native proof is incomplete'}}
        if($taskRuns.Count -ne 2 -or $taskRuns[0].run_id -cne $Scenario.run_id -or $taskRuns[0].run_id -ceq $taskRuns[1].run_id -or -not $taskRuns[0].execution.workspace -or $taskRuns[1].execution.workspace){throw 'CLI wait Run resources differ from original recovery/cancellation'}
        foreach($taskRun in $taskRuns){if($taskRun.stop_status -cne 'stopped' -or $taskRun.new_inputs_allowed -or $taskRun.execution_wait){throw 'CLI wait resources remain active'}}
        $taskEvidence=Join-Path $Manifest.root 'evidence'
        $taskQueue=Get-Content -LiteralPath (Join-Path $taskEvidence 'wait-queue-0.json') -Raw|ConvertFrom-Json
        $taskLater=Get-Content -LiteralPath (Join-Path $taskEvidence 'wait-after-old-limit.json') -Raw|ConvertFrom-Json
        $taskRecovered=Get-Content -LiteralPath (Join-Path $taskEvidence 'wait-recovered.json') -Raw|ConvertFrom-Json
        $taskCanceled=Get-Content -LiteralPath (Join-Path $taskEvidence 'wait-canceled.json') -Raw|ConvertFrom-Json
        $taskAudit=Get-Content -LiteralPath (Join-Path $taskEvidence 'wait-native-audit.json') -Raw|ConvertFrom-Json
        $taskOffline=(Get-Content -LiteralPath (Join-Path $taskEvidence 'wait-offline-stop.json') -Raw|ConvertFrom-Json).data
        if($taskQueue.execution_wait.calls_started -ne 0 -or $taskLater.execution_wait.calls_started -ne 0 -or $taskLater.execution_wait.stage -cne 'queue' -or $taskQueue.revision -ne $taskLater.revision -or $taskRecovered.operations[0].operation_id -cne $taskQueue.execution_wait.operation_id -or $taskRecovered.execution.attempt_id -cne $taskQueue.execution.attempt_id -or $taskRecovered.execution_wait){throw 'CLI wait did not retain original operation and zero-call queue'}
        if($taskCanceled.run_id -cne $taskRuns[1].run_id -or $taskCanceled.stop_status -cne 'stopped' -or $taskCanceled.execution.turn_started -or $taskCanceled.execution.workspace -or $taskAudit.workspace_creations -ne 1 -or $taskAudit.preparation_queries -ne 1 -or $taskAudit.mutation_intents -ne $taskAudit.mutation_receipts -or $taskAudit.passed -cne $true -or $taskOffline.resources_settled -cne $true){throw 'CLI wait cancellation, independent native journal or resource settlement failed'}
        return
    }
    if ($Manifest.driver -eq 'progress-file') {
        $taskRuns=@(Get-SRResultField $Scenario 'runs')
        $taskExpected=if($Manifest.scenario -eq 'local-codex.progress-multirun'){2}else{1}
        if($taskRuns.Count -ne $taskExpected -or (Get-SRResultField $Scenario 'progress_verified') -cne $true -or (Get-SRResultField $Scenario 'pr_url') -or (Get-SRResultField $Scenario 'merge_commit')){throw 'Progress proof is incomplete or published unexpectedly'}
        foreach($taskRun in $taskRuns){if($taskRun.stop_status -cne 'stopped' -or $taskRun.new_inputs_allowed){throw 'Progress native resource is not stopped/revoked'}}
        if(@($taskRuns|ForEach-Object {$_.run_id}|Select-Object -Unique).Count -ne $taskExpected){throw 'Progress Run identities collide'}
        if((Get-SRResultField $Scenario 'run_id') -cne $taskRuns[0].run_id){throw 'Progress result Run identity conflicts'}
        foreach($taskField in @('session_id','workspace','branch')){if(@($taskRuns|ForEach-Object {Get-SRResultField (Get-SRResultField $_ 'execution') $taskField}|Where-Object {$_}|Select-Object -Unique).Count -ne $taskExpected){throw 'Progress native resources collide or are missing'}}
        return
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
        {$_ -in @('development-smoke','development-feedback')} {
            if(-not $taskMerge){throw 'Live smoke lacks its verified merge'}
            if(($_ -eq 'development-feedback' -or $Manifest.scenario -eq 'local-codex.smoke' -or $Manifest.scenario -eq 'orca-remote.full-access') -and (Get-SRResultField $Scenario 'feedback_rejected') -cne $true){throw 'Feedback boundary case lacks explicit rejection evidence'}
            if(Get-SRResultField $Scenario 'feedback_head'){throw 'workflow/2 must not create a feedback revision'}
        }
        default {throw 'Unsupported development result driver'}
    }
}

function Assert-SRParserScenario($Manifest,$Scenario) {
    if ($Manifest.scenario -cne 'local-codex.result-acquisition' -or $Manifest.level -cne 'L3' -or $Scenario.business_outcome -cne 'parsed' -or $Scenario.production_parser_calls -ne 3 -or $Scenario.permission_probe_calls -ne 1) { throw 'Parser result differs from its standalone L3 contract' }
    foreach ($taskFlag in @('passed','parser_stopped')) {
        $taskValue=Get-SRResultField $Scenario $taskFlag
        if ($taskValue -isnot [bool] -or -not $taskValue) { throw 'Parser resources or assertions are unverified' }
    }
    foreach ($taskField in @('pr_url','merge_commit','run_id','runs')) { if (Get-SRResultField $Scenario $taskField) { throw 'Parser-only result contains development resources' } }
    $taskCalls=@(Get-SRResultField $Scenario 'calls')
    $taskCases=@(Get-SRResultField $Scenario 'cases')
    if ($taskCalls.Count -ne 3 -or ($taskCalls.label -join ',') -cne 'parser-default,parser-explicit,parser-cancel' -or $taskCases.Count -ne 2 -or ($taskCases.case -join ',') -cne 'parser-default,parser-boundaries') { throw 'Parser calls or cases are missing, reordered or duplicated' }
    $taskSessions=@{}
    foreach ($taskCall in $taskCalls) {
        if ($taskCall.run_id -cne ('parser-'+$Manifest.id+'-'+$taskCall.label) -or $taskCall.stopped -isnot [bool] -or -not $taskCall.stopped) { throw 'Parser original call identity or settlement differs' }
        if ($taskCall.session_id) {
            if ($taskSessions.ContainsKey($taskCall.session_id)) { throw 'Parser native sessions collide' }
            $taskSessions[$taskCall.session_id]=$true
        } elseif ($taskCall.label -cne 'parser-cancel') { throw 'Completed parser lacks its original session' }
    }
    foreach ($taskCase in $taskCases) { if ($taskCase.passed -isnot [bool] -or -not $taskCase.passed) { throw 'Parser case did not pass' } }
    if ($taskCases[0].session -cne $taskCalls[0].session_id -or $taskCases[1].session -cne $taskCalls[1].session_id -or $taskCases[0].tool_events -ne 0 -or $taskCases[1].induced_output_status -notin @('completed','policy_violation')) { throw 'Parser semantic or source evidence is incomplete' }
    foreach ($taskFlag in @('canary_unchanged','read_only_command_denied','active_cancel_stopped')) { if ((Get-SRResultField $taskCases[1] $taskFlag) -cne $true) { throw 'Parser boundary evidence is incomplete' } }
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
