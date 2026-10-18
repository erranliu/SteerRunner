# Match protocol/orca.SerializeCLI. Only the outer process takes the lock;
# its audit wrapper's native child must not take it again.
function Invoke-SRLockedOrca([string]$Executable,[string[]]$Arguments,[int]$TimeoutSeconds=40) {
    $taskLockDirectory=Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'SteerRunner'
    [IO.Directory]::CreateDirectory($taskLockDirectory)|Out-Null
    $taskLock=[IO.File]::Open((Join-Path $taskLockDirectory 'orca-cli.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::ReadWrite)
    $taskOwned=$false
    $taskDeadline=[DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    try {
        while(-not $taskOwned){
            if([DateTime]::UtcNow -ge $taskDeadline){throw 'Orca CLI queue deadline exhausted; no command invoked'}
            try{$taskLock.Lock(0,1);$taskOwned=$true}catch [IO.IOException]{Start-Sleep -Milliseconds 25}
        }
        $taskOutput=@(& $Executable @Arguments)
        $taskCode=$LASTEXITCODE
        return [pscustomobject]@{output=$taskOutput;exit_code=$taskCode}
    } finally {
        if($taskOwned){$taskLock.Unlock(0,1)}
        $taskLock.Dispose()
    }
}
