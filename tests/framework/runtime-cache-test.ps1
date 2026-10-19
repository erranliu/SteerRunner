param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
$taskRoot=Join-Path $Evidence 'runtime-cache'
New-Item -ItemType Directory -Path $taskRoot -ErrorAction Stop | Out-Null
$taskResources=Join-Path $taskRoot 'source/resources'
foreach($taskPath in @('bin','app.asar.unpacked/out','node_modules/fixture')) {New-Item -ItemType Directory -Path (Join-Path $taskResources $taskPath) -Force | Out-Null}
$taskOrca=Join-Path $taskResources 'bin/orca.exe'
[IO.File]::WriteAllText($taskOrca,'fixture')
$taskNonce=[Guid]::NewGuid().ToString('N')
[IO.File]::WriteAllText((Join-Path $taskResources 'app.asar.unpacked/out/index.js'),$taskNonce)
foreach($taskNumber in 1..16){[IO.File]::WriteAllText((Join-Path $taskResources ('node_modules/fixture/'+$taskNumber+'.js')),($taskNonce+('x'*65536)))}
$taskShell=(Get-Process -Id $PID).Path
$taskWorkers=@()
foreach($taskNumber in 1..2){
 $taskDestination=Join-Path $taskRoot ('worker-'+$taskNumber+'/runtime')
 $taskInfo=[Diagnostics.ProcessStartInfo]::new($taskShell)
 $taskInfo.UseShellExecute=$false
 $taskInfo.RedirectStandardOutput=$true
 $taskInfo.RedirectStandardError=$true
 foreach($taskArg in @('-NoProfile','-File',(Join-Path $Repository 'scripts/prepare-test-orca-runtime.ps1'),'-OrcaExe',$taskOrca,'-Destination',$taskDestination)){$taskInfo.ArgumentList.Add($taskArg)}
 $taskProcess=[Diagnostics.Process]::Start($taskInfo)
 $taskWorkers+=@{process=$taskProcess;out=$taskProcess.StandardOutput.ReadToEndAsync();err=$taskProcess.StandardError.ReadToEndAsync();destination=$taskDestination}
}
foreach($taskWorker in $taskWorkers){
 if(-not $taskWorker.process.WaitForExit(60000)){throw 'Concurrent cache preparation timed out'}
 $taskStdout=$taskWorker.out.GetAwaiter().GetResult()
 $taskStderr=$taskWorker.err.GetAwaiter().GetResult()
 if($taskWorker.process.ExitCode -ne 0){throw ('Concurrent cold-cache worker failed: '+$taskStderr)}
 $taskIndexPath=$taskStdout.Trim()
 $taskIndex=Get-Content -LiteralPath $taskIndexPath -Raw | ConvertFrom-Json
 if(@($taskIndex.files).Count -ne 18){throw 'Concurrent cache fixture did not copy its complete runtime'}
 foreach($taskFile in $taskIndex.files){if((Get-FileHash -LiteralPath (Join-Path $taskWorker.destination $taskFile.path)).Hash -ine $taskFile.sha256){throw 'Concurrent cache copied incomplete bytes'}}
 $taskWorker.process.Dispose()
}
[IO.File]::WriteAllText((Join-Path $taskRoot 'result.json'),(@{passed=$true;workers=2;files_per_worker=18;parallel_cold_cache=$true}|ConvertTo-Json))
Write-Output 'Parallel cold-cache publication preserves complete immutable bytes.'
