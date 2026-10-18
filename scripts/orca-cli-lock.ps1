# Match protocol/orca.cliMutexName. Lock only the outer CLI invocation.
function Get-SRCLIMutexName([string]$Path) {
    $taskBytes=[Text.Encoding]::UTF8.GetBytes([IO.Path]::GetFullPath($Path).ToLowerInvariant())
    $taskSHA=[Security.Cryptography.SHA256]::Create()
    try{$taskHash=$taskSHA.ComputeHash($taskBytes)}finally{$taskSHA.Dispose()}
    return 'Global\SteerRunner.OrcaCLI.'+([BitConverter]::ToString($taskHash[0..15]).Replace('-','').ToLowerInvariant())
}
function Invoke-SRLockedOrca([string]$Executable,[string[]]$Arguments,[int]$TimeoutSeconds=40) {
    $taskPath=Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'SteerRunner/orca-cli.lock'
    $taskLock=[Threading.Mutex]::new($false,(Get-SRCLIMutexName $taskPath))
    $taskOwned=$false
    try {
        try{$taskOwned=$taskLock.WaitOne([TimeSpan]::FromSeconds($TimeoutSeconds))}catch [Threading.AbandonedMutexException]{$taskOwned=$true}
        if(-not $taskOwned){throw 'Orca CLI queue deadline exhausted; no command invoked'}
        $taskOutput=@(& $Executable @Arguments)
        $taskCode=$LASTEXITCODE
        return [pscustomobject]@{output=$taskOutput;exit_code=$taskCode}
    } finally {
        if($taskOwned){$taskLock.ReleaseMutex()}
        $taskLock.Dispose()
    }
}