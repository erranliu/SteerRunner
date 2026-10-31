# A single coordinator owns summary.json. Workers own only their case record.
# L4 drains the queue and executes exclusively; L2/L3 use the requested bound.
function Open-SRTestLease([bool]$Exclusive,[string]$Directory = '') {
    $taskLeaseRoot=if($Directory){$Directory}else{Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'SteerRunner/test-scheduler'}
    $null=[IO.Directory]::CreateDirectory($taskLeaseRoot)
    $taskLeasePath=Join-Path $taskLeaseRoot 'slots.lock'
    foreach ($taskSlot in 0..7) {
        $taskStream=[IO.File]::Open($taskLeasePath,[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::ReadWrite)
        try {
            $taskStart=if($Exclusive){0}else{$taskSlot}
            $taskLength=if($Exclusive){8}else{1}
            $taskStream.Lock($taskStart,$taskLength)
            return $taskStream
        } catch [IO.IOException] {
            $taskStream.Dispose()
            if ($Exclusive) { return $null }
        }
    }
    return $null
}

function Invoke-SRCaseBatch {
    param([object[]]$Executions,[string]$Output,[string]$Config,[string]$Catalog,[string]$FixtureParent,[int]$MaxParallel,[string]$RunnerExe = '',[string]$ConfigHelper = '',[string]$ConfigurationSHA256 = '',[string]$RunnerSHA256 = '',[string]$HelperSHA256 = '',[string]$RequirementsSHA256 = '',[scriptblock]$Checkpoint,[string]$NativeDriver = '',[string]$NativeDriverSHA256 = '',[string]$WorkerScript = (Join-Path $PSScriptRoot 'run-test-case.ps1'),[string]$LeaseDirectory = '')
    $taskPending = [Collections.Generic.Queue[object]]::new()
    foreach ($taskExecution in $Executions) { $taskPending.Enqueue($taskExecution) }
    $taskActive = [Collections.Generic.List[object]]::new()
    $taskRecords = [Collections.Generic.List[hashtable]]::new()
    $taskSourceBuilds = @{}
    $taskResources = @{}
    $taskShell = (Get-Process -Id $PID).Path
    while ($taskPending.Count -or $taskActive.Count) {
        while ($taskPending.Count -and $taskActive.Count -lt $MaxParallel) {
            $taskNext = $taskPending.Peek()
            if ((($taskNext.entry.level -eq 'L4' -or $taskNext.entry.driver -eq 'cli-wait-retry') -and $taskActive.Count) -or @($taskActive | Where-Object { $_.record.level -eq 'L4' -or $_.driver -eq 'cli-wait-retry' }).Count) { break }
            $taskLease=$null
            $taskUnavailable = $taskNext.ContainsKey('unavailable') -and $taskNext.unavailable
            if (-not $taskNext.excluded -and -not $taskUnavailable) {
                $taskLease=Open-SRTestLease ($taskNext.entry.level -eq 'L4' -or $taskNext.entry.driver -eq 'cli-wait-retry') -Directory $LeaseDirectory
                if ($null -eq $taskLease) { break }
            }
            $taskNext = $taskPending.Dequeue()
            $taskStem = $taskNext.entry.id + '-' + $taskNext.case.id + '-r' + $taskNext.repeat_index
            $taskRecord = @{level=$taskNext.entry.level;scenario=$taskNext.entry.id;case_id=$taskNext.case.id;repeat_index=$taskNext.repeat_index;status='queued';manifest=$null;evidence=$null;fixture_record=(Join-Path $Output ($taskStem + '-fixture.json'));started_at=$null;finished_at=$null}
            $taskRecords.Add($taskRecord)
            if ($taskUnavailable -and -not $taskNext.excluded) {
                $taskRecord.status='unavailable'; $taskRecord['availability']=$taskNext.availability
                & $Checkpoint $taskRecords.ToArray()
                continue
            }
            if ($taskNext.excluded) {
                $taskRecord.status='excluded'; $taskRecord.reason='User explicitly excluded administrator-required tests; not executed or passed'
                & $Checkpoint $taskRecords.ToArray()
                continue
            }
            $taskResult = Join-Path $Output ($taskStem + '-case.json')
            $taskRequest = Join-Path $Output ($taskStem + '-request.json')
            $taskJob = @{entry=$taskNext.entry;case=$taskNext.case;repeat_index=$taskNext.repeat_index;config=$Config;configuration_sha256=$ConfigurationSHA256;catalog=$Catalog;fixture_parent=$FixtureParent;runner_exe=$RunnerExe;runner_sha256=$RunnerSHA256;config_helper=$ConfigHelper;helper_sha256=$HelperSHA256;requirements_sha256=$RequirementsSHA256;output=$Output;result_path=$taskResult;record=$taskRecord}
            if($NativeDriver -or $NativeDriverSHA256){$taskJob['native_driver']=$NativeDriver;$taskJob['native_driver_sha256']=$NativeDriverSHA256}
            [IO.File]::WriteAllText($taskRequest,($taskJob | ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
            $taskRecord.status='preparing'
            $taskInfo = [Diagnostics.ProcessStartInfo]::new($taskShell)
            $taskInfo.UseShellExecute=$false; $taskInfo.CreateNoWindow=$true
            $taskInfo.WorkingDirectory=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
            $taskInfo.RedirectStandardInput=$true
            $taskInfo.RedirectStandardOutput=$true
            $taskInfo.RedirectStandardError=$true
            $taskStdout=[IO.FileStream]::new((Join-Path $Output ($taskStem + '-worker.log')),[IO.FileMode]::Create,[IO.FileAccess]::Write,[IO.FileShare]::Read)
            $taskStderr=[IO.FileStream]::new((Join-Path $Output ($taskStem + '-worker-stderr.log')),[IO.FileMode]::Create,[IO.FileAccess]::Write,[IO.FileShare]::Read)
            foreach ($taskArgument in @('-NoProfile','-File',$WorkerScript,'-Request',$taskRequest)) { $taskInfo.ArgumentList.Add($taskArgument) }
            try {
                $taskProcess = [Diagnostics.Process]::Start($taskInfo)
                $taskProcess.StandardInput.Close()
                $taskStdoutCopy=$taskProcess.StandardOutput.BaseStream.CopyToAsync($taskStdout)
                $taskStderrCopy=$taskProcess.StandardError.BaseStream.CopyToAsync($taskStderr)
            } catch {
                $taskStdout.Dispose();$taskStderr.Dispose()
                $taskLease.Dispose()
                $taskRecord.status='failed'; $taskRecord.error=$_.Exception.Message; $taskRecord.finished_at=[DateTime]::UtcNow.ToString('o')
                & $Checkpoint $taskRecords.ToArray()
                continue
            }
            $taskActive.Add(@{process=$taskProcess;record=$taskRecord;path=$taskResult;driver=$taskNext.entry.driver;lease=$taskLease;stdout=$taskStdout;stderr=$taskStderr;stdout_copy=$taskStdoutCopy;stderr_copy=$taskStderrCopy})
            & $Checkpoint $taskRecords.ToArray()
            if ($taskNext.entry.level -eq 'L4') { break }
        }
        foreach ($taskChild in @($taskActive.ToArray())) {
            if (-not $taskChild.process.HasExited) {
                if (Test-Path -LiteralPath $taskChild.path) {
                    try {
                        $taskProgress=Get-Content -LiteralPath $taskChild.path -Raw | ConvertFrom-Json -AsHashtable
                        $taskChanged=$false
                        foreach($taskField in @('manifest','evidence','started_at')) {
                            if($taskProgress.ContainsKey($taskField) -and $taskProgress[$taskField] -ne $taskChild.record[$taskField]){$taskChild.record[$taskField]=$taskProgress[$taskField];$taskChanged=$true}
                        }
                        if($taskProgress.status -in @('preparing','running') -and $taskProgress.status -ne $taskChild.record.status){$taskChild.record.status=$taskProgress.status;$taskChanged=$true}
                        if($taskChanged){& $Checkpoint $taskRecords.ToArray()}
                    } catch { Write-Verbose 'Waiting for an in-progress case checkpoint' }
                }
                continue
            }
            try {
                $null=$taskChild.stdout_copy.GetAwaiter().GetResult()
                $null=$taskChild.stderr_copy.GetAwaiter().GetResult()
                if (-not (Test-Path -LiteralPath $taskChild.path)) { throw 'Worker exited without a case record; reconcile its fixture record and original resources' }
                $taskDone = Get-Content -LiteralPath $taskChild.path -Raw | ConvertFrom-Json -AsHashtable
                if ($taskDone.status -notin @('passed','failed') -or ($taskDone.status -eq 'passed' -and $taskChild.process.ExitCode -ne 0)) { throw 'Worker did not complete with a matching result and exit code' }
                foreach ($taskKey in $taskDone.Keys) { $taskChild.record[$taskKey]=$taskDone[$taskKey] }
                if ($taskDone.status -eq 'passed') {
                    $taskIdentity=$taskDone.identity
                    if ($taskSourceBuilds.ContainsKey('runner') -and $taskSourceBuilds.runner -cne $taskIdentity.runner_sha256) { throw 'Source/build changed within qualification batch: Runner' }
                    $taskSourceBuilds.runner=$taskIdentity.runner_sha256
                    $taskFamily=if($taskChild.driver -eq 'controlled-integration'){'controlled'}else{'native'}
                    $taskBuild=if($taskFamily -eq 'controlled'){@($taskIdentity.runner_sha256,$taskIdentity.fixture_sha256)}else{@($taskIdentity.runner_sha256,$taskIdentity.driver_sha256)}
                    if ($taskSourceBuilds.ContainsKey($taskFamily) -and $taskSourceBuilds[$taskFamily] -cne ($taskBuild -join ',')) { throw 'Source/build changed within qualification batch' }
                    $taskSourceBuilds[$taskFamily]=$taskBuild -join ','
                    if ($taskChild.driver -eq 'execution-stability') {
                        $taskTools=@($taskIdentity.helper_sha256,$taskIdentity.runtime_sha256,$taskIdentity.network_sha256) -join ','
                        if ($taskSourceBuilds.ContainsKey('stability') -and $taskSourceBuilds.stability -cne $taskTools) { throw 'Source/build changed within qualification batch: stability tools' }
                        $taskSourceBuilds.stability=$taskTools
                    }
                    if ($taskDone.ContainsKey('resources')) {
                        foreach ($taskResource in $taskDone.resources) {
                            if ($taskResource -match '^(root|base|run|workspace|session|branch|pr):$' -or $taskResources.ContainsKey($taskResource)) { throw "Case resources collide or lack identity: $taskResource" }
                            $taskResources[$taskResource]=$taskDone.scenario
                        }
                    }
                }
            } catch { $taskChild.record.status='failed';$taskChild.record.error=$_.Exception.Message;$taskChild.record.finished_at=[DateTime]::UtcNow.ToString('o') }
            $taskChild.process.Dispose()
            $taskChild.stdout.Dispose();$taskChild.stderr.Dispose()
            $taskChild.lease.Dispose()
            $null=$taskActive.Remove($taskChild)
            & $Checkpoint $taskRecords.ToArray()
        }
        if ($taskActive.Count -or $taskPending.Count) { Start-Sleep -Milliseconds 200 }
    }
}
