param(
    [Parameter(Mandatory)][string]$TargetTemplate,
    [Parameter(Mandatory)][string]$OutDir,
    [string]$RepositoryID = '903fb315-7d5e-4246-8913-816e1d3bdf03',
    [string]$BaseRevision = 'a161f31f2ea7537d95af0b0fde977329bf91dc12'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
. (Join-Path $PSScriptRoot 'orca-cli-lock.ps1')
$taskOut = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Comparison output must be new; no attempted case may be replayed' }
$taskTemplate = Get-Content -LiteralPath $TargetTemplate -Raw | ConvertFrom-Json
if ($BaseRevision -notmatch '^[0-9a-f]{40}$' -or -not $taskTemplate.Runtime -or -not $taskTemplate.Environment) { throw 'Frozen target/base required' }
New-Item -ItemType Directory -Path $taskOut | Out-Null
Copy-Item -LiteralPath $TargetTemplate -Destination (Join-Path $taskOut 'target-template.json')
$taskSourcePaths = @('internal','go.mod','go.sum','scripts/test-native-input.ps1','scripts/test-native-input-comparison.ps1','scripts/delivery-contract.ps1','scripts/orca-cli-lock.ps1')
$taskSource = Get-SRSourceIdentity $taskRepo $taskSourcePaths
$taskSummary = [ordered]@{schema='native-followup-comparison/v1';scope='test EXE production Lifecycle; no shipped Runner publication qualification';started_at=[DateTime]::UtcNow.ToString('o');status='preparing';source=$taskSource;runtime=$taskTemplate.Runtime;environment=$taskTemplate.Environment;base_revision=$BaseRevision;planned_per_group=10;rows=@();active_case=$null}
$taskNonce = [Guid]::NewGuid().ToString('N').Substring(0,8)
$taskExe = Join-Path $taskOut 'native-input.test.exe'
$taskKeys = @('STEERRUNNER_ORCA_STATE_PROBE_ROOT','STEERRUNNER_ORCA_STATE_PROBE_MODE','STEERRUNNER_ORCA_STATE_PROBE_ENVIRONMENT','STEERRUNNER_ORCA_FOLLOWUP_SCALARS')
$taskPrevious = @{}
foreach($taskKey in $taskKeys){$taskPrevious[$taskKey]=[Environment]::GetEnvironmentVariable($taskKey)}
function Save-Comparison {
    $taskTemporary = Join-Path $taskOut 'progress.tmp.json'
    [IO.File]::WriteAllText($taskTemporary,($taskSummary|ConvertTo-Json -Depth 20),[Text.UTF8Encoding]::new($false))
    [IO.File]::Move($taskTemporary,(Join-Path $taskOut 'result.json'),$true)
}
function Invoke-ComparisonOrca([string]$Directory,[string]$Name,[string[]]$Arguments) {
    $taskBegin=[DateTime]::UtcNow.ToString('o')
    $taskResponse=Invoke-SRLockedOrca -Executable $taskTemplate.CLI -Arguments @($Arguments+@('--json','--environment',$taskTemplate.Environment))
    $taskText=$taskResponse.output -join "`n"
    [IO.File]::WriteAllText((Join-Path $Directory ($Name+'.stdout.json')),$taskText,[Text.UTF8Encoding]::new($false))
    $taskValue=$taskText|ConvertFrom-Json
    [IO.File]::WriteAllText((Join-Path $Directory ($Name+'.call.json')),(@{started_at=$taskBegin;finished_at=[DateTime]::UtcNow.ToString('o');args=$Arguments;exit_code=$taskResponse.exit_code}|ConvertTo-Json -Depth 8),[Text.UTF8Encoding]::new($false))
    if ($taskValue.ok -and $taskValue._meta.runtimeId -ne $taskTemplate.Runtime){throw 'Original runtime changed; no replacement selection'}
    return $taskValue
}
function Get-EmptyComparisonCensus([string]$Directory,[string]$Name,[string]$Worktree) {
    $taskCensus=Invoke-ComparisonOrca $Directory $Name @('terminal','list','--worktree',('id:'+$Worktree))
    if (-not $taskCensus.ok -or $null -eq $taskCensus.result.totalCount -or $taskCensus.result.totalCount -ne 0 -or $taskCensus.result.truncated -ne $false -or $taskCensus.result.hostScope.hostIds.Count -lt 1 -or $taskCensus.result.hostScope.omittedHostIds.Count -ne 0 -or $taskCensus.result.terminals.Count -ne 0) { throw 'Complete empty owned native census not verified; stop comparison' }
    return $taskCensus
}
Save-Comparison
Push-Location $taskRepo
try {
    & go test -c -o $taskExe ./internal/execution/adapters/orcatransport 1> (Join-Path $taskOut 'compile.log') 2> (Join-Path $taskOut 'compile-stderr.log')
    if($LASTEXITCODE -ne 0){throw 'Comparison test EXE compilation failed'}
    $taskSummary['test_exe_sha256']=Get-SRFileDigest $taskExe
    $taskSummary['orca_cli_sha256']=Get-SRFileDigest $taskTemplate.CLI
    $taskSummary.status='running'
    # Ten balanced pairs, sequential execution. Odd pairs short/long, even long/short.
    foreach($taskPair in 1..10) {
        $taskOrder=if($taskPair%2 -eq 1){@(800,3427)}else{@(3427,800)}
        foreach($taskSize in $taskOrder) {
            $taskOrdinal=$taskSummary.rows.Count+1
            $taskLabel=('{0:D2}-{1}-{2:D2}' -f $taskOrdinal,$taskSize,$taskPair)
            $taskCase=Join-Path $taskOut $taskLabel
            New-Item -ItemType Directory -Path $taskCase | Out-Null
            $taskRow=[ordered]@{ordinal=$taskOrdinal;pair=$taskPair;scalars=$taskSize;root=$taskLabel;started_at=[DateTime]::UtcNow.ToString('o');passed=$false;cleanup_verified=$false;test_seconds=0;verified_turns=0;stage='creating'}
            $taskSummary.active_case=$taskRow
            Save-Comparison
            $taskName='sr-cmp-'+$taskNonce+'-'+$taskLabel
            $taskCreated=Invoke-ComparisonOrca $taskCase 'worktree-create' @('worktree','create','--repo',('id:'+$RepositoryID),'--name',$taskName,'--base-branch',$BaseRevision,'--no-parent','--setup','skip')
            if(-not $taskCreated.ok -or -not $taskCreated.result.worktree.id){throw 'Original creation not verified; never repeat create'}
            $taskWorktree=$taskCreated.result.worktree
            $taskRow['worktree']=$taskWorktree.id
            $taskFull=[IO.Path]::GetFullPath($taskWorktree.path).Replace('\','/')
            $taskAllowed='C:/Users/Administrator/orca/workspaces/SteerlineTest/'+$taskName
            if($taskFull -cne $taskAllowed -or $taskWorktree.head -ne $BaseRevision){throw 'New worktree path/base differs from fixed intent'}
            $taskNative=$null
            try {
                $taskList=Invoke-ComparisonOrca $taskCase 'initial-census' @('terminal','list','--worktree',('id:'+$taskWorktree.id))
                if(-not $taskList.ok -or $taskList.result.truncated -ne $false -or $taskList.result.hostScope.omittedHostIds.Count -ne 0 -or $taskList.result.totalCount -gt 1){throw 'Initial worktree inventory not controlled'}
                if($taskList.result.totalCount -eq 1){
                    $taskShell=$taskList.result.terminals[0]
                    $taskShown=$null
                    foreach($taskRead in 1..20){
                        $taskShown=Invoke-ComparisonOrca $taskCase ('shell-show-'+$taskRead) @('terminal','show','--terminal',$taskShell.handle)
                        if($taskShown.ok -and $taskShown.result.terminal.preview -like ('PS *'+$taskName+'>')){break}
                        Start-Sleep -Milliseconds 250
                    }
                    if(-not $taskShown.ok -or $taskShown.result.terminal.preview -notlike ('PS *'+$taskName+'>') -or $taskShown.result.terminal.incarnationId -ne $taskShell.incarnationId){throw 'Default unused shell not verified'}
                    $taskClosed=Invoke-ComparisonOrca $taskCase 'shell-close' @('terminal','close','--terminal',$taskShell.handle)
                    if(-not $taskClosed.ok -or -not $taskClosed.result.close.ptyKilled){throw 'Default shell close not confirmed'}
                }
                $null=Get-EmptyComparisonCensus $taskCase 'before-test-census' $taskWorktree.id
                $taskProbe=Join-Path $taskCase 'probe'
                New-Item -ItemType Directory -Path $taskProbe | Out-Null
                $taskTarget=$taskTemplate|ConvertTo-Json -Depth 12|ConvertFrom-Json
                $taskTarget.Worktree='id:'+$taskWorktree.id
                [IO.File]::WriteAllText((Join-Path $taskProbe 'target.json'),($taskTarget|ConvertTo-Json -Depth 12),[Text.UTF8Encoding]::new($false))
                [Environment]::SetEnvironmentVariable($taskKeys[0],$taskProbe)
                [Environment]::SetEnvironmentVariable($taskKeys[1],'followup-immediate')
                [Environment]::SetEnvironmentVariable($taskKeys[2],[string]$taskTemplate.Environment)
                [Environment]::SetEnvironmentVariable($taskKeys[3],[string]$taskSize)
                $taskRow.stage='test-running'
                Save-Comparison
                $taskTimer=[Diagnostics.Stopwatch]::StartNew()
                & $taskExe '-test.run=^TestOrcaNativeStateLiveLifecycle$' '-test.count=1' '-test.timeout=6m' '-test.v' 1> (Join-Path $taskProbe 'test.log') 2> (Join-Path $taskProbe 'test-stderr.log')
                $taskExit=$LASTEXITCODE
                $taskTimer.Stop()
                $taskRow['exit_code']=$taskExit
                $taskRow['test_seconds']=$taskTimer.Elapsed.TotalSeconds
                $taskLog=Get-Content (Join-Path $taskProbe 'test.log') -Raw
                if(Test-Path (Join-Path $taskProbe 'native-result.json')){$taskNative=Get-Content (Join-Path $taskProbe 'native-result.json') -Raw|ConvertFrom-Json}
                $taskRow.stage='test-completed'
                if($null -ne $taskNative){
                    if($taskNative.PSObject.Properties.Name -contains 'verified_turns'){$taskRow['verified_turns']=@($taskNative.verified_turns).Count}
                    $taskRow['paste_calls']=$taskNative.paste_calls
                    $taskRow['enter_calls']=$taskNative.enter_calls
                    if($taskRow.verified_turns -eq 2){
                        $taskSecond=$taskNative.verified_turns[1]
                        $taskRow['bytes']=$taskSecond.input_bytes
                        $taskRow['input_sha256']=$taskSecond.input_sha256
                        $taskRow.passed=($taskExit -eq 0 -and $taskLog -match '(?m)^--- PASS: TestOrcaNativeStateLiveLifecycle ' -and $taskLog -notmatch '(?m)^--- SKIP:' -and $taskNative.completed -and $taskNative.cleaned_up -and $taskNative.paste_calls -eq 2 -and $taskNative.enter_calls -eq 2 -and $taskSecond.input_scalars -eq $taskSize -and $taskSecond.original_input.Text -ceq ([IO.File]::ReadAllText((Join-Path $taskProbe 'input-2.txt'))))
                    }
                }
                if(-not $taskRow.passed){$taskRow['failure_log']=$taskLog}
            } catch {
                $taskRow['error']=$_.Exception.Message
            } finally {
                # Never repeat any input. Only reconcile a still-live, exactly owned terminal.
                $taskCensus=Invoke-ComparisonOrca $taskCase 'after-test-census' @('terminal','list','--worktree',('id:'+$taskWorktree.id))
                if(-not $taskCensus.ok -or $taskCensus.result.truncated -ne $false -or $taskCensus.result.hostScope.omittedHostIds.Count -ne 0){throw 'Owned resource census unavailable; comparison stopped'}
                foreach($taskTerminal in $taskCensus.result.terminals){
                    if($null -eq $taskNative -or $taskTerminal.handle -ne $taskNative.terminal.handle -or $taskTerminal.incarnationId -ne $taskNative.terminal.incarnationId){throw 'Residual terminal identity unknown; do not close or continue'}
                    $taskClose=Invoke-ComparisonOrca $taskCase 'owned-residual-close' @('terminal','close','--terminal',$taskTerminal.handle)
                    if(-not $taskClose.ok -or -not $taskClose.result.close.ptyKilled){throw 'Owned residual close not confirmed'}
                }
                $null=Get-EmptyComparisonCensus $taskCase 'cleanup-census' $taskWorktree.id
                $taskRemoved=Invoke-ComparisonOrca $taskCase 'worktree-removed' @('worktree','rm','--worktree',('id:'+$taskWorktree.id),'--force')
                if(-not $taskRemoved.ok){throw 'Owned workspace removal not confirmed'}
                $taskAfter=Invoke-ComparisonOrca $taskCase 'worktree-after' @('worktree','show','--worktree',('id:'+$taskWorktree.id))
                if($taskAfter.ok -or $taskAfter.error.code -ne 'selector_not_found'){throw 'Owned workspace still present'}
                $taskRow.cleanup_verified=$true
            }
            $taskRow.stage='settled'
            $taskRow['finished_at']=[DateTime]::UtcNow.ToString('o')
            $taskSummary.rows+=@($taskRow)
            $taskSummary.active_case=$null
            Save-Comparison
            Write-Output ($taskLabel+' passed='+$taskRow.passed+' cleanup='+$taskRow.cleanup_verified+' seconds='+$taskRow.test_seconds)
        }
    }
    $taskAfterSource=Get-SRSourceIdentity $taskRepo $taskSourcePaths
    if($taskAfterSource.source_digest -cne $taskSource.source_digest -or (Get-SRFileDigest $taskExe) -cne $taskSummary.test_exe_sha256){throw 'Frozen probe source or EXE changed'}
    $taskSummary.status='completed'
} catch {
    $taskSummary.status='stopped-on-infrastructure-or-ownership-error'
    $taskSummary['error']=$_.Exception.Message
    throw
} finally {
    foreach($taskKey in $taskKeys){[Environment]::SetEnvironmentVariable($taskKey,$taskPrevious[$taskKey])}
    Pop-Location
    $taskSummary['finished_at']=[DateTime]::UtcNow.ToString('o')
    Save-Comparison
}
