function Get-SRResultField($Value,[string]$Name) {
    if($null -ne $Value -and $Value.PSObject.Properties[$Name]) { return $Value.$Name }
    return $null
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
    if(-not $taskBound -or @($View.audit.results).Count -ne 2){throw 'Audit model evidence lacks its bound session and two audits'}
    $taskRecords=@()
    foreach($taskAudit in $View.audit.results){
        if($taskAudit.verdict -cne 'pass' -or -not $taskContexts.ContainsKey($taskAudit.turn_id)){throw 'Audit lacks its passing native turn'}
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
        'development-audited' { if ($taskMerge) { throw 'Audited scenario must leave its verified PR unmerged' } }
        'development-multirun' {
            $taskRuns=@(Get-SRResultField $Scenario 'runs')
            if($taskRuns.Count -ne 2){throw 'Multi-Run result must contain two Run resources'}
            foreach($taskRun in $taskRuns){if((Get-SRResultField $taskRun 'stop_status') -cne 'stopped' -or (Get-SRResultField $taskRun 'new_inputs_allowed') -isnot [bool] -or $taskRun.new_inputs_allowed){throw 'Multi-Run resources are not stopped/revoked'}}
            foreach($taskField in @('run_id')){if(@($taskRuns|ForEach-Object {Get-SRResultField $_ $taskField}|Where-Object {$_}|Select-Object -Unique).Count -ne 2){throw 'Multi-Run identities collide or are missing'}}
            foreach($taskField in @('session_id','workspace','branch')){if(@($taskRuns|ForEach-Object {Get-SRResultField (Get-SRResultField $_ 'execution') $taskField}|Where-Object {$_}|Select-Object -Unique).Count -ne 2){throw 'Multi-Run native resources collide or are missing'}}
            if(@($taskRuns|ForEach-Object {Get-SRResultField (Get-SRResultField $_ 'pr') 'url'}|Where-Object {$_}|Select-Object -Unique).Count -ne 2 -or $taskMerge){throw 'Multi-Run PR identities collide or were unexpectedly merged'}
        }
        {$_ -in @('development-core','development-smoke','development-feedback')} {
            if(-not $taskMerge){throw 'Live smoke lacks its verified merge'}
            if($_ -ne 'development-core' -and -not (Get-SRResultField $Scenario 'feedback_head')){throw 'Full smoke/feedback case lacks its new feedback head'}
        }
        default {throw 'Unsupported development result driver'}
    }
}
